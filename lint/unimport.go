package lint

import (
	"go/ast"
	"go/types"
	"strconv"

	"github.com/gunk/gunk/loader"
)

func lintUnimport(l *Linter, pkgs []*loader.GunkPackage) {
	for _, pkg := range pkgs {
		for _, f := range pkg.GunkSyntax {
			usedImports := make(map[string]bool)
			addType := func(typ types.Type) {
				// Mark the package imported by the type as used.
				for typ != nil {
					if named, ok := typ.(*types.Named); ok {
						pkg := named.Obj().Pkg()
						if pkg != nil {
							usedImports[pkg.Path()] = true
						}
					}
					parent, ok := typ.(containerType)
					if !ok {
						return
					}
					typ = parent.Elem()
				}
			}
			ast.Inspect(f, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.Field:
					addType(pkg.TypesInfo.Types[v.Type].Type)
				}
				return true
			})
			for _, list := range pkg.GunkTags {
				for _, v := range list {
					addType(v.Type)
				}
			}
			ast.Inspect(f, func(n ast.Node) bool {
				switch v := n.(type) {
				default:
					return false
				case *ast.File, *ast.GenDecl:
					return true
				case *ast.ImportSpec:
					importPath, err := strconv.Unquote(v.Path.Value)
					if err != nil {
						l.addError(n, "failed to parse import %q", v.Path.Value)
					}
					if !usedImports[importPath] {
						l.addError(n, "unused import %s", importPath)
					}
					return false
				}
			})
		}
	}
}
