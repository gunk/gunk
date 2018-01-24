package main

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/types"
	"html/template"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/golang/protobuf/proto"
	desc "github.com/golang/protobuf/protoc-gen-go/descriptor"
)

// addPkg sets up a gunk package to be translated and generated. It is
// parsed from the gunk files on disk and type-checked, gathering all
// the info needed later on.
func (t *translator) addPkg(path string) error {
	bpkg, err := t.bctx.Import(path, t.wd, build.FindOnly)
	if err != nil {
		return err
	}
	matches, err := filepath.Glob(filepath.Join(bpkg.Dir, "*.gunk"))
	if err != nil {
		return err
	}
	// TODO: support multiple packages
	var list []*ast.File
	bpkg.Name = "default"
	astFiles := make(map[string]*ast.File)
	for _, match := range matches {
		file, err := parser.ParseFile(t.fset, match, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		bpkg.Name = file.Name.Name
		// to make the generated code independent of the current
		// directory when running gunk
		relPath := bpkg.ImportPath + "/" + filepath.Base(match)
		astFiles[relPath] = file
		t.origPaths[relPath] = match
		list = append(list, file)
	}
	tpkg := types.NewPackage(bpkg.ImportPath, bpkg.Name)
	check := types.NewChecker(t.tconfig, t.fset, tpkg, t.info)
	if err := check.Files(list); err != nil {
		return err
	}
	t.bldPkgs[path] = bpkg
	t.typPkgs[tpkg.Path()] = tpkg
	t.astPkgs[tpkg.Path()] = astFiles
	t.toGen[path] = make(map[string]bool)
	return nil
}

// Import is our own implementation of types.Importer. Unlike standard
// Go ones like go/importer and x/tools/go/loader, this one uses our own
// addPkg to instead load gunk packages.
//
// Aside from that, it is very similar to standard Go importers that
// load from source. It too uses a cache to avoid loading packages
// multiple times.
func (t *translator) Import(path string) (*types.Package, error) {
	if tpkg := t.typPkgs[path]; tpkg != nil {
		return tpkg, nil
	}
	if err := t.addPkg(path); err != nil {
		return nil, err
	}
	if err := t.translatePkg(path); err != nil {
		return nil, err
	}
	return t.typPkgs[path], nil
}

// addProtoDep is called when a gunk file is known to require importing
// of a proto file, such as when using google.protobuf.Empty.
func (t *translator) addProtoDep(protoPath string) {
	for _, dep := range t.pfile.Dependency {
		if dep == protoPath {
			return // already in there
		}
	}
	t.pfile.Dependency = append(t.pfile.Dependency, protoPath)
}

// loadProtoDeps loads all the proto dependencies added with
// addProtoDep. It does so with protoc, to leverage its features like
// locating the files, and its parser to get a FileDescriptorProto out
// of the proto file content.
func (t *translator) loadProtoDeps() error {
	missing := make(map[string]bool)
	for _, pfile := range t.allProto {
		for _, dep := range pfile.Dependency {
			if _, e := t.allProto[dep]; !e {
				missing[dep] = true
			}
		}
	}
	tmpl := template.Must(template.New("letter").Parse(`
syntax = "proto3";

{{range $dep, $_ := .}}import "{{$dep}}";
{{end}}
`))
	importsFile, err := os.Create("gunk-proto")
	if err != nil {
		return err
	}
	if err := tmpl.Execute(importsFile, missing); err != nil {
		return err
	}
	if err := importsFile.Close(); err != nil {
		return err
	}
	defer os.Remove("gunk-proto")
	// TODO: any way to specify stdout while being portable?
	cmd := exec.Command("protoc", "-o/dev/stdout", "--include_imports", "gunk-proto")
	out, err := cmd.Output()
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("%s", e.Stderr)
		}
		return err
	}
	var fset desc.FileDescriptorSet
	if err := proto.Unmarshal(out, &fset); err != nil {
		return err
	}
	for _, pfile := range fset.File {
		if *pfile.Name == "gunk-proto" {
			continue
		}
		t.allProto[*pfile.Name] = pfile
	}
	return nil
}
