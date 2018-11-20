package loader

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	desc "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"golang.org/x/tools/go/packages"

	"github.com/gunk/gunk/log"
)

// Load loads the Gunk packages on the provided patterns from the given dir and
// using the given fileset.
//
// Similar to Go, if a path begins with ".", it is interpreted as a file system
// path where a package is located, and "..." patterns are supported.
func Load(dir string, fset *token.FileSet, patterns ...string) ([]*GunkPackage, error) {
	// First, translate the patterns to package paths.
	cfg := &packages.Config{
		Dir:  dir,
		Mode: packages.LoadFiles,
	}
	lpkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}

	// Add the Gunk files to each package.
	pkgs := make([]*GunkPackage, 0, len(lpkgs))
	for _, lpkg := range lpkgs {
		pkg, err := toGunkPackage(fset, lpkg)
		if err != nil {
			return nil, err
		}
		if len(pkg.GunkFiles) == 0 {
			// A Go package that isn't a Gunk package - skip it.
			continue
		}
		pkgs = append(pkgs, pkg)
	}

	return pkgs, nil
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

	ProtoName string // protobuf package name
}

func toGunkPackage(fset *token.FileSet, lpkg *packages.Package) (*GunkPackage, error) {
	if len(lpkg.Errors) > 0 {
		return nil, lpkg.Errors[0]
	}

	pkgDir := ""
	for _, gofile := range lpkg.GoFiles {
		dir := filepath.Dir(gofile)
		if pkgDir == "" {
			pkgDir = dir
		} else if dir != pkgDir {
			return nil, fmt.Errorf("multiple dirs for %s: %s %s",
				lpkg.PkgPath, pkgDir, dir)
		}
	}

	matches, err := filepath.Glob(filepath.Join(pkgDir, "*.gunk"))
	if err != nil {
		return nil, err
	}

	pkg := &GunkPackage{
		Package:   *lpkg,
		Dir:       pkgDir,
		GunkFiles: matches,
	}

	// parse the gunk files
	for _, fpath := range pkg.GunkFiles {
		file, err := parser.ParseFile(fset, fpath, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		// to make the generated code independent of the current
		// directory when running gunk
		relPath := pkg.PkgPath + "/" + filepath.Base(fpath)
		pkg.GunkNames = append(pkg.GunkNames, relPath)
		pkg.GunkSyntax = append(pkg.GunkSyntax, file)

		name, err := protoPackageName(fset, file)
		if err != nil {
			return nil, err
		}
		if pkg.ProtoName == "" {
			pkg.ProtoName = name
		} else if name != "" {
			return nil, fmt.Errorf("proto package name mismatch: %q %q",
				pkg.ProtoName, name)
		}
	}
	if pkg.ProtoName == "" {
		pkg.ProtoName = pkg.Name
	}
	return pkg, nil
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

// LoadProto loads the specified protobuf packages as if they were dependencies.
//
// It does so with protoc, to leverage protoc's features such as locating the
// files, and the protoc parser to get a FileDescriptorProto out of the proto
// file content.
func LoadProto(names ...string) ([]*desc.FileDescriptorProto, error) {
	tmpl := template.Must(template.New("letter").Parse(`
syntax = "proto3";

{{range $_, $name := .}}import "{{$name}}";
{{end}}
`))
	importsFile, err := os.Create("gunk-proto")
	if err != nil {
		return nil, err
	}
	if err := tmpl.Execute(importsFile, names); err != nil {
		return nil, err
	}
	if err := importsFile.Close(); err != nil {
		return nil, err
	}
	defer os.Remove("gunk-proto")

	// TODO: any way to specify stdout while being portable?
	cmd := log.ExecCommand("protoc", "-o/dev/stdout", "--include_imports", "gunk-proto")
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
	for i := 0; i < len(fset.File); {
		if *fset.File[i].Name == "gunk-proto" {
			fset.File = append(fset.File[:i], fset.File[i+1:]...)
		} else {
			i++
		}
	}
	return fset.File, nil
}

// SplitGunkTag splits a '+gunk' tag from a comment group, returning the leading
// documentation and the tag's Go expression.
func SplitGunkTag(fset *token.FileSet, comment *ast.CommentGroup) (string, ast.Expr, error) {
	docLines := strings.Split(comment.Text(), "\n")
	var tagLines []string
	for i, line := range docLines {
		if strings.HasPrefix(line, "+gunk ") {
			tagLines = docLines[i:]
			// Replace "+gunk" with spaces, so that we keep the
			// tag's lines all starting at the same column, for
			// accurate position information later.
			tagLines[0] = strings.Replace(tagLines[0], "+gunk", "     ", 1)
			docLines = docLines[:i]
			break
		}
	}
	doc := strings.TrimSpace(strings.Join(docLines, "\n"))
	tagStr := strings.Join(tagLines, "\n")
	if strings.TrimSpace(tagStr) == "" {
		return doc, nil, nil
	}
	tag, err := parser.ParseExprFrom(fset, "", tagStr, 0)
	if err != nil {
		tagPos := fset.Position(comment.Pos())
		tagPos.Line += len(docLines) // relative to the "+gunk" line
		tagPos.Column += len("// ")  // .Text() stripped these prefixes
		return "", nil, ErrorAbsolutePos(err, tagPos)
	}
	// TODO: make positions in the tag expression absolute too
	return doc, tag, nil
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
