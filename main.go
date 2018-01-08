package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

func main() {
	flag.Parse()
	for _, path := range flag.Args() {
		if err := runDir(path); err != nil {
			log.Fatal(err)
		}
	}
}

func runDir(path string) error {
	// TODO: we don't error if the dir does not exist
	matches, err := filepath.Glob(filepath.Join(path, "*.gunk"))
	if err != nil {
		return err
	}
	req := &plugin.CodeGeneratorRequest{}
	for _, match := range matches {
		f, err := os.Open(match)
		if err != nil {
			return err
		}
		req.FileToGenerate = append(req.FileToGenerate, match)
		pfile, err := protoFile(f, match)
		f.Close()
		if err != nil {
			return err
		}
		req.ProtoFile = append(req.ProtoFile, pfile)
	}
	g := generator.New()
	g.Request = req
	g.CommandLineParameters(g.Request.GetParameter())

	// Create a wrapped version of the Descriptors and EnumDescriptors that
	// point to the file that defines them.
	g.WrapTypes()

	g.SetPackageNames()
	g.BuildTypeNameMap()

	g.GenerateAllFiles()
	for _, rf := range g.Response.File {
		// to turn foo.gunk.pb.go into foo.pb.go
		name := strings.Replace(*rf.Name, ".gunk", "", 1)
		data := []byte(*rf.Content)
		if err := ioutil.WriteFile(name, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

func protoFile(r io.Reader, filename string) (*descriptor.FileDescriptorProto, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, r, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	pfile := &descriptor.FileDescriptorProto{
		Name: &filename,
	}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			return nil, fmt.Errorf("%s: invalid declaration type %T", filename, decl)
		}
		switch gd.Tok {
		case token.TYPE:
			for _, spec := range gd.Specs {
				ts := spec.(*ast.TypeSpec)
				switch ts.Type.(type) {
				case *ast.StructType:
					msg, err := protoMessage(ts)
					if err != nil {
						return nil, err
					}
					pfile.MessageType = append(pfile.MessageType, msg)
				}
			}
		}
	}
	return pfile, nil
}

func protoMessage(tspec *ast.TypeSpec) (*descriptor.DescriptorProto, error) {
	msg := &descriptor.DescriptorProto{
		Name: &tspec.Name.Name,
	}
	stype := tspec.Type.(*ast.StructType)
	for _, field := range stype.Fields.List {
		if len(field.Names) != 1 {
			return nil, fmt.Errorf("need all fields to have one name")
		}
		pfield := &descriptor.FieldDescriptorProto{
			Name: &field.Names[0].Name,
		}
		switch ptype, tname := protoType(field.Type); ptype {
		case 0:
			return nil, fmt.Errorf("unsupported field type: %v", field.Type)
		case descriptor.FieldDescriptorProto_TYPE_ENUM:
			continue
			pfield.Type = &ptype
			pfield.TypeName = &tname
		default:
			pfield.Type = &ptype
		}
		msg.Field = append(msg.Field, pfield)
	}
	return msg, nil
}

func protoType(from ast.Expr) (descriptor.FieldDescriptorProto_Type, string) {
	switch x := from.(type) {
	case *ast.Ident:
		switch x.Name {
		case "string":
			return descriptor.FieldDescriptorProto_TYPE_STRING, ""
		default:
			return descriptor.FieldDescriptorProto_TYPE_ENUM, x.Name
		}
	}
	return 0, ""
}
