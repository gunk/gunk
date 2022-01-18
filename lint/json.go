package lint

import (
	"go/ast"
	"reflect"
	"strconv"

	"github.com/gunk/gunk/loader"
	"github.com/kenshaw/snaker"
)

func lintJSON(l *Linter, pkgs []*loader.GunkPackage) {
	for _, pkg := range pkgs {
		for _, f := range pkg.GunkSyntax {
			ast.Inspect(f, func(n ast.Node) bool {
				switch v := n.(type) {
				default:
					return false
				case *ast.File, *ast.GenDecl, *ast.TypeSpec, *ast.StructType, *ast.FieldList:
					// Continue walking down the tree for these types.
					return true
				case *ast.Field:
					if v.Tag == nil {
						l.addError(n, "expecting JSON tag, found none")
						return false
					}
					tagValue, err := strconv.Unquote(v.Tag.Value)
					if err != nil {
						l.addError(n, "invalid struct tag")
						return false
					}
					tag := reflect.StructTag(tagValue)
					json, ok := tag.Lookup("json")
					if !ok {
						l.addError(n, "expecting JSON tag, found none")
						return false
					}
					if len(v.Names) != 1 {
						l.addError(n, "expected exactly 1 name, got %d", len(v.Names))
						return false
					}
					snakeCase := snaker.CamelToSnakeIdentifier(v.Names[0].Name)
					if json != snakeCase {
						l.addError(n, "JSON name must be snake case of field name")
						return false
					}
				}
				return false
			})
		}
	}
}
