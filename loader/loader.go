package loader

import (
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/scanner"
	"go/token"
	"go/types"
	"html/template"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/golang/protobuf/proto"
	desc "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"golang.org/x/tools/go/packages"

	"github.com/gunk/gunk/assets"
	"github.com/gunk/gunk/log"
)

type Loader struct {
	Dir  string
	Fset *token.FileSet

	// If Types is true, we parse and type-check the given packages and all
	// transitive dependencies, including gunk tags. Otherwise, we only
	// parse the given packages.
	Types bool

	cache map[string]*GunkPackage // map from import path to pkg
}

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func hexRand(enclen int) string {
	p := make([]byte, hex.DecodedLen(enclen))
	// math/rand's Read can't error
	_, _ = seededRand.Read(p)
	return hex.EncodeToString(p)
}

// addTempGoFiles adds a temporary empty Go file with a random name to all Gunk
// packages with no Go files, so that packages.Load can find them via patterns
// like "./...". Check all directories within the current module, falling back
// to all directories under the current directory.
func (l *Loader) addTempGoFiles() (undo func(), _ error) {
	// TODO(mvdan): Use go/packages.Config.Overlay once it supports adding
	// new Go packages, as that removes the need for writing to disk and
	// cleaning up after ourselves.
	// See https://github.com/golang/go/issues/29047.
	root := "."
	cmd := exec.Command("go", "list", "-m", "-f={{.Dir}}")
	cmd.Dir = l.Dir
	// use "." if we encountered an error, for e.g. GOPATH mode
	if out, err := cmd.Output(); err == nil {
		root = strings.TrimSpace(string(out))
	}
	if root == "" {
		return nil, fmt.Errorf("empty module root; missing module?")
	}

	var toDelete []string
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if strings.Contains(path, "@v") {
			// in the module cache; skip, as that's read-only anyway
			return filepath.SkipDir
		}
		infos, err := ioutil.ReadDir(path)
		if err != nil {
			return err
		}
		pkgName := info.Name() // default to the directory basename
		anyGunk := false
		for _, info := range infos {
			name := info.Name()
			if strings.HasSuffix(name, ".go") {
				// has Go files; nothing to do
				return nil
			}
			if strings.HasSuffix(name, ".gunk") {
				f, err := parser.ParseFile(token.NewFileSet(),
					filepath.Join(path, name), nil, parser.PackageClauseOnly)
				// Ignore errors, since Gunk packages being
				// walked but not being loaded might have
				// invalid syntax.
				if err == nil {
					pkgName = f.Name.Name
				}
				anyGunk = true
				break
			}
		}
		if !anyGunk {
			return nil
		}
		tmpPath := filepath.Join(path, "gunkpkg-"+hexRand(8)+".go")
		if err := ioutil.WriteFile(tmpPath, []byte("package "+pkgName), 0666); err != nil {
			return err
		}
		toDelete = append(toDelete, tmpPath)
		return nil
	}); err != nil {
		return nil, err
	}
	return func() {
		anyErr := false
		for _, path := range toDelete {
			if err := os.Remove(path); err != nil {
				anyErr = true
				fmt.Fprintf(os.Stderr, "could not delete gunkpkg file: %v", err)
			}
		}
		if anyErr {
			panic("could not delete some of the gunkpkg files")
		}
	}, nil
}

// Load loads the Gunk packages on the provided patterns from the given dir and
// using the given fileset.
//
// Similar to Go, if a path begins with ".", it is interpreted as a file system
// path where a package is located, and "..." patterns are supported.
func (l *Loader) Load(patterns ...string) ([]*GunkPackage, error) {
	if len(patterns) == 1 {
		pkgPath := patterns[0]
		if pkg := l.cache[pkgPath]; pkg != nil {
			return []*GunkPackage{pkg}, nil
		}
	}

	var pkgs []*GunkPackage
	loadFiles := len(patterns) > 0 && strings.HasSuffix(patterns[0], ".gunk")
	if loadFiles {
		// If we're given a number of files, construct a
		// packages.Package manually. go/packages will treat foo.gunk as
		// an import path instead of a file, as it's not a Go file.
		pkgs = append(pkgs, &GunkPackage{
			Package: packages.Package{
				ID:      "command-line-arguments",
				Name:    "", // will be filled later
				PkgPath: "command-line-arguments",
			},
			GunkFiles: patterns,
		})
	} else {
		// First, make sure that all Gunk packages have Go files.
		undo, err := l.addTempGoFiles()
		if err != nil {
			return nil, err
		}
		defer undo()

		// Load the Gunk packages as Go packages.
		cfg := &packages.Config{
			Dir:  l.Dir,
			Mode: packages.LoadFiles,
		}
		lpkgs, err := packages.Load(cfg, patterns...)
		if err != nil {
			return nil, err
		}
		for _, lpkg := range lpkgs {
			pkg := &GunkPackage{Package: *lpkg}
			findGunkFiles(pkg)
			if len(pkg.GunkFiles) == 0 {
				// A Go package that isn't a Gunk package - skip it.
				continue
			}
			pkgs = append(pkgs, pkg)
		}
	}

	// Add the Gunk files to each package.
	for _, pkg := range pkgs {
		l.parseGunkPackage(pkg)
		l.validatePackage(pkg)
		if l.cache == nil {
			l.cache = make(map[string]*GunkPackage)
		}
		l.cache[pkg.PkgPath] = pkg
	}
	return pkgs, nil
}

// findGunkFiles fills a package's GunkFiles field with the gunk files found in
// the package directory. This is used when loading a Gunk package via an import
// path or a directory.
//
// Note that this requires all the source files within the package to be in the
// same directory, which is true for Go Modules and GOPATH, but not other build
// systems like Bazel.
func findGunkFiles(pkg *GunkPackage) {
	for _, gofile := range pkg.GoFiles {
		dir := filepath.Dir(gofile)
		if pkg.Dir == "" {
			pkg.Dir = dir
		} else if dir != pkg.Dir {
			pkg.addError(ListError, 0, nil, "multiple dirs for %s: %s %s",
				pkg.PkgPath, pkg.Dir, dir)
			return // we can't continue
		}
	}

	matches, err := filepath.Glob(filepath.Join(pkg.Dir, "*.gunk"))
	if err != nil {
		// can only be a malformed pattern; should never happen.
		panic(err.Error())
	}
	pkg.GunkFiles = matches
}

const (
	UnknownError = packages.UnknownError
	ListError    = packages.ListError
	ParseError   = packages.ParseError
	TypeError    = packages.TypeError

	// Our kinds of errors. Add a gap of 10 to be sure we won't conflict
	// with previous enum values.

	ValidateError = packages.TypeError + 10 + iota
)

// Import satisfies the go/types.Importer interface.
//
// Unlike standard Go ones like go/importer and x/tools/go/packages, this one is
// adapted to load Gunk packages.
//
// Aside from that, it is very similar to standard Go importers that load from
// source.
func (l *Loader) Import(path string) (*types.Package, error) {
	if !strings.Contains(path, ".") {
		cfg := &packages.Config{Mode: packages.LoadTypes}
		pkgs, err := packages.Load(cfg, path)
		if err != nil {
			return nil, err
		}
		if len(pkgs) != 1 {
			panic("expected go/packages.Load to return exactly one package")
		}
		return pkgs[0].Types, nil
	}
	pkgs, err := l.Load(path)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 {
		panic("expected Loader.Load to return exactly one package")
	}
	return pkgs[0].Types, nil
}

type GunkPackage struct {
	packages.Package
	Dir string // for now, we require all files to be in the same dir

	GunkFiles  []string    // absolute paths of the Gunk files
	GunkSyntax []*ast.File // syntax trees for the files in GunkFiles

	// GunkNames are unique arbitrary names for each gunk file in GunkFiles.
	// We don't want to use absolute paths when referring to files in the
	// CodeGeneratorRequest, because that will trigger many generators to
	// write to disk.
	GunkNames []string

	// GunkTags stores the "+gunk" tags associated with each syntax tree
	// node in GunkSyntax.
	GunkTags map[ast.Node][]GunkTag

	Imports map[string]*GunkPackage

	ProtoName string // protobuf package name
}

func (g *GunkPackage) addError(kind packages.ErrorKind, tokenPos token.Pos, fset *token.FileSet, format string, args ...interface{}) {
	pos := ""
	if tokenPos > 0 && fset != nil {
		pos = fset.Position(tokenPos).String()
	}
	g.Errors = append(g.Errors, packages.Error{
		Pos:  pos,
		Msg:  fmt.Sprintf(format, args...),
		Kind: kind,
	})
}

type GunkTag struct {
	ast.Expr            // original expression
	Type     types.Type // type of the expression

	Value constant.Value // constant value of the expression, if any
}

// parseGunkPackage parses the package's GunkFiles, and type-checks the package
// if l.Types is set.
func (l *Loader) parseGunkPackage(pkg *GunkPackage) {
	// parse the gunk files
	for _, fpath := range pkg.GunkFiles {
		file, err := parser.ParseFile(l.Fset, fpath, nil, parser.ParseComments)
		if err != nil {
			pkg.addError(ParseError, 0, nil, "%s", err)
			continue
		}
		// to make the generated code independent of the current
		// directory when running gunk
		relPath := pkg.PkgPath + "/" + filepath.Base(fpath)
		pkg.GunkNames = append(pkg.GunkNames, relPath)
		pkg.GunkSyntax = append(pkg.GunkSyntax, file)

		if name := file.Name.Name; pkg.Name == "" {
			pkg.Name = name
		} else if pkg.Name != name && l.Types {
			pkg.addError(ValidateError, 0, nil, "gunk package name mismatch: %q %q",
				pkg.Name, name)
		}

		name, err := protoPackageName(l.Fset, file)
		if err != nil {
			pkg.addError(ParseError, 0, nil, "%s", err)
			continue
		}
		if pkg.ProtoName == "" {
			pkg.ProtoName = name
		} else if name != "" && l.Types {
			pkg.addError(ValidateError, 0, nil, "proto package name mismatch: %q %q",
				pkg.ProtoName, name)
			continue
		}
	}
	if pkg.ProtoName == "" {
		pkg.ProtoName = pkg.Name
	}

	// the reported error will be handle at generate.Run function.
	if len(pkg.Errors) > 0 {
		return
	}

	if !l.Types {
		return
	}

	pkg.Types = types.NewPackage(pkg.PkgPath, pkg.Name)
	tconfig := &types.Config{
		DisableUnusedImportCheck: true,
		Importer:                 l,
	}
	pkg.TypesInfo = &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Scopes:     make(map[ast.Node]*types.Scope),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}

	check := types.NewChecker(tconfig, l.Fset, pkg.Types, pkg.TypesInfo)
	if err := check.Files(pkg.GunkSyntax); err != nil {
		pkg.addError(TypeError, 0, nil, "%s", err)
		return
	}
	pkg.Imports = make(map[string]*GunkPackage)
	for _, file := range pkg.GunkSyntax {
		l.splitGunkTags(pkg, file)
		for _, spec := range file.Imports {
			// we can't error, since the file parsed correctly
			pkgPath, _ := strconv.Unquote(spec.Path.Value)
			pkgs, err := l.Load(pkgPath)
			if err != nil {
				// shouldn't happen?
				panic(err)
			}
			if len(pkgs) == 1 {
				pkg.Imports[pkgPath] = pkgs[0]
			}
		}
	}
}

// validatePackage sanity checks a gunk package, to find common errors which are
// shared among all gunk commands.
func (l *Loader) validatePackage(pkg *GunkPackage) {
	for _, file := range pkg.GunkSyntax {
		ast.Inspect(file, func(node ast.Node) bool {
			st, ok := node.(*ast.StructType)
			if !ok || st.Fields == nil {
				return true
			}

			// Look through all fields for anonymous/unnamed types.
			for _, field := range st.Fields.List {
				if len(field.Names) < 1 {
					pkg.addError(ParseError, st.Pos(), l.Fset, "anonymous struct fields are not supported")
					return false
				}
			}

			// Check for struct tag 'pb' and ensure that if it does exist
			// it is a valid integer, and it is unique in that struct.
			// The other validation should happen in format and generate
			// as they both treat the same error cases differently.
			usedSequences := make(map[int]bool, len(st.Fields.List))
			for _, f := range st.Fields.List {
				if f.Tag == nil {
					continue
				}
				fieldName := f.Names[0].Name
				str, _ := strconv.Unquote(f.Tag.Value)
				stag := reflect.StructTag(str)
				val, ok := stag.Lookup("pb")
				if !ok || val == "" {
					continue
				}
				sequence, err := strconv.Atoi(val)
				if err != nil {
					pkg.addError(ValidateError, st.Pos(), l.Fset, "unable to convert tag to number on %s: %v", fieldName, err)
					continue
				}
				if usedSequences[sequence] {
					pkg.addError(ValidateError, st.Pos(), l.Fset, "sequence %q on %s has already been used in this struct", val, fieldName)
					continue
				}
				usedSequences[sequence] = true
			}
			return true
		})
	}
}

const protoCommentPrefix = "// proto "

func protoPackageName(fset *token.FileSet, file *ast.File) (string, error) {
	packageLine := fset.Position(file.Package).Line
allComments:
	for _, cgroup := range file.Comments {
		for _, comment := range cgroup.List {
			cline := fset.Position(comment.Pos()).Line
			if cline < packageLine {
				continue // comment before package line
			} else if cline > packageLine {
				break allComments // we're past the package line
			}
			quoted := strings.TrimPrefix(comment.Text, protoCommentPrefix)
			if quoted == comment.Text {
				continue // comment doesn't have the prefix
			}
			unquoted, err := strconv.Unquote(quoted)
			return unquoted, err
		}
	}

	// none found
	return "", nil
}

type ProtoLoader struct {
	// Dir is the absolute path from where the LoadProto method
	// will load proto files.
	// If empty, it will load from executing directory
	Dir        string
	ProtocPath string
}

// LoadProto loads the specified protobuf packages as if they were dependencies.
//
// It does so with protoc, to leverage protoc's features such as locating the
// files, and the protoc parser to get a FileDescriptorProto out of the proto
// file content.
func (l *ProtoLoader) LoadProto(names ...string) ([]*desc.FileDescriptorProto, error) {
	tmpl := template.Must(template.New("letter").Parse(`
syntax = "proto3";

{{range $_, $name := .}}import "{{$name}}";
{{end}}
`))
	// Imports to load from in-memory
	generatedFilesToLoad := []string{}
	// Imports to load using protoc
	filteredNames := make([]string, 0, len(names))

	// Check to see if we are trying to load any libraries that we have
	// bundled with Gunk. If so, load the generated libraries. If not, use
	// protoc to load those libraries from disk.
	for _, n := range names {
		switch n {
		case "google/api/annotations.proto":
			generatedFilesToLoad = append(generatedFilesToLoad, "google_api_annotations.fdp")
		case "google/protobuf/empty.proto":
			generatedFilesToLoad = append(generatedFilesToLoad, "google_protobuf_empty.fdp")
		case "google/protobuf/timestamp.proto":
			generatedFilesToLoad = append(generatedFilesToLoad, "google_protobuf_timestamp.fdp")
		case "google/protobuf/duration.proto":
			generatedFilesToLoad = append(generatedFilesToLoad, "google_protobuf_duration.fdp")
		case "protoc-gen-swagger/options/annotations.proto":
			generatedFilesToLoad = append(generatedFilesToLoad, "protoc-gen-swagger_options_annotations.fdp")
		default:
			filteredNames = append(filteredNames, n)
		}
	}
	var combinedFset desc.FileDescriptorSet
	// Use protoc to load any imports that aren't currently bundles with
	// Gunk.
	if len(filteredNames) > 0 {
		gunkProtoFile := "gunk-proto"
		if l.Dir != "" {
			gunkProtoFile = filepath.Join(l.Dir, gunkProtoFile)
		}
		importsFile, err := os.Create(gunkProtoFile)
		if err != nil {
			return nil, err
		}
		if err := tmpl.Execute(importsFile, filteredNames); err != nil {
			return nil, err
		}
		if err := importsFile.Close(); err != nil {
			return nil, err
		}
		defer os.Remove(gunkProtoFile)

		// TODO(mvdan): any way to specify stdout while being portable?
		// See https://github.com/protocolbuffers/protobuf/issues/4163.
		args := []string{
			"-o/dev/stdout",
			"--include_imports",
			gunkProtoFile,
		}
		if l.Dir != "" {
			args = append(args, "-I"+l.Dir)
		}
		protocPath := "protoc"
		if l.ProtocPath != "" {
			protocPath = l.ProtocPath
		}
		cmd := log.ExecCommand(protocPath, args...)
		out, err := cmd.Output()
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok {
				return nil, fmt.Errorf("protoc %s: %s", e, e.Stderr)
			}
			return nil, err
		}
		var fset desc.FileDescriptorSet
		if err := proto.Unmarshal(out, &fset); err != nil {
			return nil, err
		}
		for i := 0; i < len(fset.File); i++ {
			if *fset.File[i].Name != "gunk-proto" {
				continue
			}
			combinedFset.File = append(fset.File[:i], fset.File[i+1:]...)
		}
	}
	// Load any bundled libraries.
	for _, fileToLoad := range generatedFilesToLoad {
		file, err := assets.Assets.Open(fileToLoad)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		fi, err := file.Stat()
		if err != nil {
			return nil, err
		}
		f := make([]byte, fi.Size())
		if _, err := file.Read(f); err != nil {
			if err != io.EOF {
				return nil, err
			}
		}
		var fset desc.FileDescriptorSet
		if err := proto.Unmarshal(f, &fset); err != nil {
			return nil, err
		}
		combinedFset.File = append(combinedFset.File, fset.File...)
	}
	return combinedFset.File, nil
}

// splitGunkTags parses and typechecks gunk tags from the comments in a Gunk
// file, adding them to pkg.GunkTags and removing the source lines from each
// comment.
func (l *Loader) splitGunkTags(pkg *GunkPackage, file *ast.File) {
	ast.Inspect(file, func(node ast.Node) bool {
		if gd, ok := node.(*ast.GenDecl); ok {
			if len(gd.Specs) != 1 {
				return true
			}
			if doc := nodeDoc(gd.Specs[0]); doc != nil {
				// Move the doc to the only spec, since we want
				// +gunk tags attached to the type specs.
				*doc = gd.Doc
			}
			return true
		}
		doc := nodeDoc(node)
		if doc == nil {
			return true
		}
		docText, exprs, err := SplitGunkTag(pkg, l.Fset, *doc)
		if err != nil {
			pkg.addError(ParseError, 0, nil, "%s", err)
			return false
		}
		if len(exprs) > 0 {
			if pkg.GunkTags == nil {
				pkg.GunkTags = make(map[ast.Node][]GunkTag)
			}
			pkg.GunkTags[node] = exprs
			**doc = *CommentFromText(*doc, docText)
		}
		return true
	})
	// TODO(mvdan): check that we aren't ignoring any other +gunk comments,
	// to prevent human error.
}

func nodeDoc(node ast.Node) **ast.CommentGroup {
	switch node := node.(type) {
	case *ast.File:
		return &node.Doc
	case *ast.Field:
		return &node.Doc
	case *ast.TypeSpec:
		return &node.Doc
	case *ast.ValueSpec:
		return &node.Doc
	}
	return nil
}

// TODO(mvdan): both loader and format use CommentFromText, but it feels awkward
// to have it here.

// CommentFromText creates a multi-line comment from the given text, with its
// start and end positions matching the given node's.
func CommentFromText(orig ast.Node, text string) *ast.CommentGroup {
	group := &ast.CommentGroup{}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		comment := &ast.Comment{Text: "// " + line}

		// Ensure that group.Pos() and group.End() stay on the same
		// lines, to ensure that printing doesn't move the comment
		// around or introduce newlines.
		switch i {
		case 0:
			comment.Slash = orig.Pos()
		case len(lines) - 1:
			comment.Slash = orig.End()
		}
		group.List = append(group.List, comment)
	}
	return group
}

// SplitGunkTag splits '+gunk' tags from a comment group, returning the leading
// documentation and the tags Go expressions.
//
// If pkg is not nil, the tag is also type-checked using the package's type
// information.
func SplitGunkTag(pkg *GunkPackage, fset *token.FileSet, comment *ast.CommentGroup) (string, []GunkTag, error) {
	// Remove the comment leading and / or trailing identifier; // and /* */ and `
	docLines := strings.Split(comment.Text(), "\n")
	var gunkTagLines []string
	var gunkTagPos []int
	var commentLines []string
	foundGunkTag := false
	for i, line := range docLines {
		if strings.HasPrefix(line, "+gunk ") {
			// Replace "+gunk" with spaces, so that we keep the
			// tag's lines all starting at the same column, for
			// accurate position information later.
			gunkTagLine := strings.Replace(line, "+gunk", "     ", 1)
			gunkTagLines = append(gunkTagLines, gunkTagLine)
			gunkTagPos = append(gunkTagPos, i)
			foundGunkTag = true
		} else if foundGunkTag {
			gunkTagLines[len(gunkTagLines)-1] += "\n" + line
		} else {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			commentLines = append(commentLines, line)
		}
	}
	if len(gunkTagLines) == 0 {
		return comment.Text(), nil, nil
	}
	var tags []GunkTag
	for i, gunkTag := range gunkTagLines {
		expr, err := parser.ParseExprFrom(fset, "", gunkTag, 0)
		if err != nil {
			tagPos := fset.Position(comment.Pos())
			tagPos.Line += gunkTagPos[i] // relative to the "+gunk" line
			tagPos.Column += len("// ")  // .Text() stripped these prefixes
			return "", nil, ErrorAbsolutePos(err, tagPos)
		}
		tag := GunkTag{Expr: expr}
		if pkg != nil {
			tv, err := types.Eval(fset, pkg.Types, comment.Pos(), gunkTag)
			if err != nil {
				return "", nil, err
			}
			tag.Type, tag.Value = tv.Type, tv.Value
		}
		tags = append(tags, tag)
	}
	// TODO: make positions in the tag expression absolute too
	return strings.Join(commentLines, "\n"), tags, nil
}

// ErrorAbsolutePos modifies all positions in err, considered to be relative to
// pos. This is useful so that the position information of syntax tree nodes
// parsed from a comment are relative to the entire file, and not only relative
// to the comment containing the source.
func ErrorAbsolutePos(err error, pos token.Position) error {
	list, ok := err.(scanner.ErrorList)
	if !ok {
		return err
	}
	for i, err := range list {
		err.Pos.Filename = pos.Filename
		err.Pos.Line += pos.Line
		err.Pos.Line-- // since these numbers are 1-based
		err.Pos.Column += pos.Column
		err.Pos.Column-- // since these numbers are 1-based
		list[i] = err
	}
	return list
}
