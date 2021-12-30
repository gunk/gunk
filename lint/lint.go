package lint

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"sort"
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

type linter struct {
	Usage string
	Run   func(*Linter, []*loader.GunkPackage)
}

var linters = map[string]linter{
	"commentstart": {
		Usage: "enforces comments to start with the name of the described object",
		Run:   lintCommentStart,
	},
	"json": {
		Usage: "enforces JSON tags to be snake case versions of field name",
		Run:   lintJSON,
	},
	"unimport": {
		Usage: "lists all imports that are unused",
		Run:   lintUnimport,
	},
	"unused": {
		Usage: "lists all enums and structs that are unused",
		Run:   lintUnused,
	},
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
		v.Run(l, pkgs)
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

// PrintLinters prints out all linters to stdout.
func PrintLinters() {
	fmt.Println("Linters available:")
	// Sort the name of the linters before displaying.
	keys := make([]string, 0, len(linters))
	for k := range linters {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	// Print the linter.
	for _, k := range keys {
		v := linters[k]
		fmt.Printf("\t%-10s - %s\n", k, v.Usage)
	}
}
