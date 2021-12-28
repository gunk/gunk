package lint

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"strings"

	"github.com/gunk/gunk/loader"
)

// LintError is an error a linter reported during linting.
type LintError struct {
	Pos string // "file:line:col" or "file:line" or "" or "-"
	Msg error
}

func (l LintError) Error() string {
	if l.Pos == "" {
		return l.Msg.Error()
	}
	return l.Pos + ": " + l.Msg.Error()
}

func (l LintError) Unwrap() error {
	return l.Msg
}

type linter func(*Linter, []*loader.GunkPackage)

var linters = map[string]linter{
	"unused": lintUnused,
}

// Run starts the linter in the provided directory with the specified
// arguments.
// If enable is not empty, it is treated as a whitelist.
// If disable is not empty, it is treated as a blacklist.
func Run(dir string, enable string, disable string, args ...string) error {
	l := New(dir)
	pkgs, err := l.Load(args...)
	if err != nil {
		return fmt.Errorf("error loading packages: %w", err)
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("no Gunk packages to lint")
	}
	if loader.PrintErrors(pkgs) > 0 {
		return fmt.Errorf("encountered package loading errors")
	}
	// Decide linters to run
	lintersToRun := make(map[string]linter, len(linters))
	if enable == "" {
		for k, v := range linters {
			lintersToRun[k] = v
		}
	} else {
		for _, name := range strings.Split(enable, ",") {
			linter, ok := linters[name]
			if !ok {
				return fmt.Errorf("unknown linter: %q", name)
			}
			lintersToRun[name] = linter
		}
	}
	if disable != "" {
		for _, name := range strings.Split(disable, ",") {
			_, ok := linters[name]
			if !ok {
				return fmt.Errorf("unknown linter: %q", name)
			}
			delete(lintersToRun, name)
		}
	}
	// Run the linters
	for _, v := range lintersToRun {
		v(l, pkgs)
	}
	if l.PrintErrors() > 0 {
		return fmt.Errorf("encountered linting errors")
	}
	return nil
}

// Linter is a struct that holds the state of the linter.
type Linter struct {
	*loader.Loader
	Err []LintError
}

// New creates a new initialized linter instance.
func New(dir string) *Linter {
	return &Linter{
		Loader: &loader.Loader{
			Dir:   dir,
			Fset:  token.NewFileSet(),
			Types: true,
		},
		Err: make([]LintError, 0),
	}
}

// PrintErrors print the errors the linter accumulated and returns the amount
// of errors that have been printed.
func (l Linter) PrintErrors() int {
	for _, v := range l.Err {
		fmt.Fprintln(os.Stderr, v)
	}
	return len(l.Err)
}

func (l *Linter) addError(n ast.Node, formatStr string, args ...interface{}) {
	l.Err = append(l.Err, LintError{
		Pos: l.Fset.Position(n.Pos()).String(),
		Msg: fmt.Errorf(formatStr, args...),
	})
}

// walk is a helper function that creates a visitor to walk the AST tree and
// recurses into the node if the function returns true for the node.
func walk(root ast.Node, f func(ast.Node) bool) {
	ast.Walk(visitor(f), root)
}

type visitor func(ast.Node) bool

// Visit implements ast.Visitor.
func (v visitor) Visit(n ast.Node) ast.Visitor {
	if v(n) {
		// Continue to recurse.
		return v
	}
	return nil
}
