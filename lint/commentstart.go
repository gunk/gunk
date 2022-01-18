package lint

import (
	"go/ast"
	"strings"

	"github.com/gunk/gunk/loader"
)

// lintCommentStart reports all comments that do not start with the name of the
// object they're describing.
func lintCommentStart(l *Linter, pkgs []*loader.GunkPackage) {
	for _, pkg := range pkgs {
		for _, f := range pkg.GunkSyntax {
			ast.Inspect(f, func(n ast.Node) bool {
				switch v := n.(type) {
				default:
					return false
				case *ast.File, *ast.GenDecl, *ast.StructType, *ast.InterfaceType, *ast.FieldList:
					return true
				case *ast.TypeSpec:
					checkCommentStart(l, n, v.Name.Name, v.Doc.Text())
					return true
				case *ast.Field:
					if len(v.Names) != 1 {
						return true
					}
					checkCommentStart(l, n, v.Names[0].Name, v.Doc.Text())
					return true
				}
			})
		}
	}
}

// checkCommentStart checks the start of a comment for a name prefix and adds a
// linter error otherwise.
func checkCommentStart(l *Linter, n ast.Node, name string, comment string) {
	prefix := name + " "
	if strings.HasPrefix(comment, prefix) {
		return
	}
	if comment == "" {
		l.addError(n, "missing comment for %q", name)
		return
	}
	l.addError(n, "comment for %q must start with %q", name, prefix)
}
