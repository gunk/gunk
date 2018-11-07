package loader

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
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
