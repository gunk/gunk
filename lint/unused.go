package lint

import (
	"go/ast"
	"go/types"

	"github.com/gunk/gunk/loader"
)

type containerType interface {
	Elem() types.Type
}

// lintUnused reports all unused structs and enums.
func lintUnused(l *Linter, pkgs []*loader.GunkPackage) {
	// Collect used entries.
	usedDecl := make(map[string]bool)
	for _, pkg := range pkgs {
		addType := func(typ types.Type) {
			// Mark the type as used.
			for typ != nil {
				usedDecl[typ.String()] = true
				parent, ok := typ.(containerType)
				if !ok {
					return
				}
				typ = parent.Elem()
			}
		}
		for _, f := range pkg.GunkSyntax {
			ast.Inspect(f, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.Field:
					addType(pkg.TypesInfo.Types[v.Type].Type)
				}
				return true
			})
		}
	}
	// Find all unused declarations.
	for _, pkg := range pkgs {
		for _, f := range pkg.GunkSyntax {
			ast.Inspect(f, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.TypeSpec:
					if _, ok := v.Type.(*ast.InterfaceType); ok {
						// Skip interface types, they define services and should
						// always be considered used.
						return false
					}
					obj := pkg.TypesInfo.Defs[v.Name]
					if !usedDecl[obj.Type().String()] {
						l.addError(n, "unused declared type: %s", v.Name)
					}
					return false
				}
				return true
			})
		}
	}
}
