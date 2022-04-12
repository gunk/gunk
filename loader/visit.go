package loader

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// This file is an almost exact copy of go/packages/visit.go, but changed to
// work on Gunk packages.
// Visit visits all the packages in the import graph whose roots are
// pkgs, calling the optional pre function the first time each package
// is encountered (preorder), and the optional post function after a
// package's dependencies have been visited (postorder).
// The boolean result of pre(pkg) determines whether
// the imports of package pkg are visited.
func Visit(pkgs []*GunkPackage, pre func(*GunkPackage) bool, post func(*GunkPackage)) {
	seen := make(map[*GunkPackage]bool)
	var visit func(*GunkPackage)
	visit = func(pkg *GunkPackage) {
		if seen[pkg] {
			return
		}
		seen[pkg] = true
		if pre == nil || pre(pkg) {
			paths := make([]string, 0, len(pkg.Imports))
			for path := range pkg.Imports {
				paths = append(paths, path)
			}
			sort.Strings(paths) // Imports is a map, this makes visit stable
			for _, path := range paths {
				visit(pkg.Imports[path])
			}
		}
		if post != nil {
			post(pkg)
		}
	}
	for _, pkg := range pkgs {
		visit(pkg)
	}
}

// PrintErrors prints to os.Stderr the accumulated errors of all
// packages in the import graph rooted at pkgs, dependencies first.
// PrintErrors returns the number of errors present, including import errors
// which aren't printed.
func PrintErrors(pkgs []*GunkPackage) int {
	var n int
	Visit(pkgs, nil, func(pkg *GunkPackage) {
		for _, err := range pkg.Errors {
			n++
			if strings.Contains(err.Error(), "error importing package") {
				continue
			}
			if pkg.errorsPrinted {
				continue
			}
			fmt.Fprintln(os.Stderr, err)
		}
		pkg.errorsPrinted = true
	})
	return n
}
