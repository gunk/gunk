package format

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"io/ioutil"
	"strings"

	"github.com/gunk/gunk/loader"
)

// Run formats Gunk files to be canonically formatted.
func Run(dir string, args ...string) error {
	fset := token.NewFileSet()
	pkgs, err := loader.Load(dir, fset, args...)
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("no Gunk packages to format")
	}
	for _, pkg := range pkgs {
		for i, file := range pkg.GunkSyntax {
			path := pkg.GunkFiles[i]
			orig, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			got, err := formatFile(fset, file)
			if err != nil {
				return err
			}

			if !bytes.Equal(orig, got) {
				if err := ioutil.WriteFile(path, got, 0666); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Source canonically formats a single Gunk file, returning the result and any
// error encountered.
func Source(src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", src, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	return formatFile(fset, file)
}

func formatFile(fset *token.FileSet, file *ast.File) (_ []byte, formatErr error) {
	// Use custom panic values to report errors from the inspect func,
	// since that's the easiest way to immediately halt the process and
	// return the error.
	type inspectError struct{ err error }
	defer func() {
		if r := recover(); r != nil {
			if ierr, ok := r.(inspectError); ok {
				formatErr = ierr.err
			} else {
				panic(r)
			}
		}
	}()
	ast.Inspect(file, func(node ast.Node) bool {
		group, ok := node.(*ast.CommentGroup)
		if !ok {
			return true
		}
		doc, tag, err := loader.SplitGunkTag(fset, group)
		if err != nil {
			panic(inspectError{err})
		}
		if tag == nil {
			// no gunk tag
			return true
		}
		var buf bytes.Buffer

		// Print with space indentation, since all comment lines begin
		// with "// " and we don't want to mix spaces and tabs.
		config := printer.Config{Mode: printer.UseSpaces, Tabwidth: 8}
		if err := config.Fprint(&buf, fset, tag); err != nil {
			panic(inspectError{err})
		}
		text := doc + "\n\n+gunk " + buf.String()
		*group = *commentFromText(group, text)
		return true
	})
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func commentFromText(orig ast.Node, text string) *ast.CommentGroup {
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
