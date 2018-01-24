package main

import (
	"bytes"
	"flag"
	"go/ast"
	"go/build"
	"go/token"
	"go/types"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/golang/protobuf/proto"
	desc "github.com/golang/protobuf/protoc-gen-go/descriptor"
	_ "github.com/golang/protobuf/protoc-gen-go/grpc"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

func main() {
	flag.Parse()
	if err := runPaths("", flag.Args()...); err != nil {
		log.Fatal(err)
	}
}

// runPaths runs gunk on the gunk packages located at the given import
// paths. Just like most Go tools, if a path beings with ".", it is
// interpreted as a file system path where a package is located.
//
// If gopath is empty, the default is used.
func runPaths(gopath string, paths ...string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	t := translator{
		wd:   wd,
		fset: token.NewFileSet(),
		tconfig: &types.Config{
			DisableUnusedImportCheck: true,
		},
		info: &types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
			Defs:  make(map[*ast.Ident]types.Object),
			Uses:  make(map[*ast.Ident]types.Object),
		},
		bldPkgs:   make(map[string]*build.Package),
		typPkgs:   make(map[string]*types.Package),
		astPkgs:   make(map[string]map[string]*ast.File),
		toGen:     make(map[string]map[string]bool),
		allProto:  make(map[string]*desc.FileDescriptorProto),
		origPaths: make(map[string]string),
	}
	t.tconfig.Importer = &t
	t.bctx = build.Default
	if gopath != "" {
		t.bctx.GOPATH = gopath
	}
	for _, path := range paths {
		if err := t.addPkg(path); err != nil {
			return err
		}
		if err := t.translatePkg(path); err != nil {
			return err
		}
	}
	if err := t.loadProtoDeps(); err != nil {
		return err
	}
	for _, path := range paths {
		if err := t.generatePkg(path); err != nil {
			return err
		}
	}
	return nil
}

type translator struct {
	bctx build.Context
	wd   string

	gfile *ast.File
	pfile *desc.FileDescriptorProto
	tpkg  *types.Package

	fset    *token.FileSet
	tconfig *types.Config
	info    *types.Info

	astPkgs map[string]map[string]*ast.File
	bldPkgs map[string]*build.Package
	typPkgs map[string]*types.Package

	toGen     map[string]map[string]bool
	allProto  map[string]*desc.FileDescriptorProto
	origPaths map[string]string

	msgIndex  int32
	srvIndex  int32
	enumIndex int32
}

// generatePkg runs the proto files resulting from translating gunk
// packages through a code generator, such as protoc-gen-go to generate
// Go packages.
//
// The resulting files are written alongside the original gunk files in
// the directory of each gunk package.
func (t *translator) generatePkg(path string) error {
	req := t.requestForPkg(path)
	bs, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	cmd := exec.Command("protoc-gen-go")
	cmd.Stdin = bytes.NewReader(bs)
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	var resp plugin.CodeGeneratorResponse
	if err := proto.Unmarshal(out, &resp); err != nil {
		return err
	}
	for _, rf := range resp.File {
		// to turn foo.gunk.pb.go into foo.pb.go
		inPath := strings.Replace(*rf.Name, ".pb.go", "", 1)
		outPath := t.origPaths[inPath]
		outPath = strings.Replace(outPath, ".gunk", ".pb.go", 1)
		data := []byte(*rf.Content)
		if err := ioutil.WriteFile(outPath, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func (t *translator) requestForPkg(path string) *plugin.CodeGeneratorRequest {
	// For deterministic output, as the first file in each package
	// gets an extra package godoc.
	req := &plugin.CodeGeneratorRequest{
		Parameter: proto.String("plugins=grpc"),
	}
	for file := range t.toGen[path] {
		req.FileToGenerate = append(req.FileToGenerate, file)
	}
	sort.Strings(req.FileToGenerate)
	for _, pfile := range t.allProto {
		req.ProtoFile = append(req.ProtoFile, pfile)
	}
	return req
}
