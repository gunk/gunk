package lint

import (
	"go/ast"

	"github.com/gunk/gunk/loader"
)

// lintUnused reports all unused structs and enums.
func lintUnused(l *Linter, pkgs []*loader.GunkPackage) {
	// Collect used entries.
	usedDecl := make(map[string]bool)
	for _, pkg := range pkgs {
		addIdent := func(i *ast.Ident) {
			// Mark the type the identifier uses as used.
			obj := pkg.TypesInfo.Uses[i]
			usedDecl[obj.Type().String()] = true
		}
		for _, f := range pkg.GunkSyntax {
			walk(f, func(n ast.Node) bool {
				switch v := n.(type) {
				case *ast.Field:
					switch t := v.Type.(type) {
					case *ast.Ident:
						addIdent(t)
					case *ast.SelectorExpr:
						addIdent(t.Sel)
					}
				}
				return true
			})
		}
	}
	// Find all unused declarations.
	for _, pkg := range pkgs {
		for _, f := range pkg.GunkSyntax {
			walk(f, func(n ast.Node) bool {
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
