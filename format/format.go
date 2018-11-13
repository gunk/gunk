package format

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"io/ioutil"

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
			if err := formatFile(fset, path, file); err != nil {
				return err
			}
		}
	}
	return nil
}

func formatFile(fset *token.FileSet, path string, file *ast.File) error {
	orig, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return err
	}
	got := buf.Bytes()
	if bytes.Equal(orig, got) {
		// already formatted; nothing to do
		return nil
	}
	return ioutil.WriteFile(path, got, 0666)
}
