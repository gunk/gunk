package lint

import (
	"go/ast"
	"go/types"
	"strings"

	"github.com/gunk/gunk/loader"
)

// lintComment reports all comments that do not start with the name of the
// object they're describing or end with a period.
func lintComment(l *Linter, pkgs []*loader.GunkPackage) {
	for _, pkg := range pkgs {
		for _, f := range pkg.GunkSyntax {
			ast.Inspect(f, func(n ast.Node) bool {
				switch v := n.(type) {
				default:
					return false
				case *ast.File, *ast.GenDecl, *ast.StructType, *ast.InterfaceType, *ast.FieldList:
					return true
				case *ast.TypeSpec:
					checkComment(l, n, v.Name.Name, v.Doc.Text(), true)
					return true
				case *ast.Field:
					if len(v.Names) != 1 {
						return true
					}
					typ := pkg.TypesInfo.TypeOf(v.Names[0])
					_, isMethod := typ.(*types.Signature)
					checkComment(l, n, v.Names[0].Name, v.Doc.Text(), !isMethod)
					return true
				}
			})
		}
	}
}

// checkComment checks the comment for a name prefix and dot suffix. It adds a
// linter error otherwise.
func checkComment(l *Linter, n ast.Node, name string, comment string, startWithIs bool) {
	prefix := name + " "
	if !strings.HasPrefix(comment, prefix) {
		if comment == "" {
			l.addError(n, "missing comment for %q", name)
			return
		}
		l.addError(n, "comment for %q must start with %q", name, prefix)
		return
	}
	if !strings.HasSuffix(strings.TrimSpace(comment), ".") {
		l.addError(n, "comment for %q must end with a period", name)
		return
	}
	if !startWithIs {
		return
	}
	if strings.HasPrefix(comment, prefix+"is ") || strings.HasPrefix(comment, prefix+"are ") {
		return
	}
	l.addError(n, "comment for %q must start with %q", name, prefix+"is/are ")
}
