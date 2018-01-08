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
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
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
	return runPaths(matches...)
}

func runPaths(paths ...string) error {
	req := &plugin.CodeGeneratorRequest{}
	for _, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		req.FileToGenerate = append(req.FileToGenerate, path)
		pfile, err := protoFile(f, path)
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
	file, err := parser.ParseFile(fset, filename, r, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	g := gunkGen{
		gfile: file,
		pfile: &descriptor.FileDescriptorProto{
			Name:   &filename,
			Syntax: proto.String("proto3"),
		},
	}
	for _, decl := range file.Decls {
		if err := g.decl(decl); err != nil {
			return nil, err
		}
	}
	return g.pfile, nil
}

type gunkGen struct {
	gfile *ast.File
	pfile *descriptor.FileDescriptorProto

	msgIndex  int32
	enumIndex int32
}

func (g *gunkGen) decl(decl ast.Decl) error {
	gd, ok := decl.(*ast.GenDecl)
	if !ok {
		return fmt.Errorf("invalid declaration %T", decl)
	}
	if gd.Tok != token.TYPE {
		return nil
	}
	for _, spec := range gd.Specs {
		ts := spec.(*ast.TypeSpec)
		if ts.Doc == nil {
			// pass it on to the helpers
			ts.Doc = gd.Doc
		}
		switch ts.Type.(type) {
		case *ast.StructType:
			msg, err := g.protoMessage(ts)
			if err != nil {
				return err
			}
			g.pfile.MessageType = append(g.pfile.MessageType, msg)
		case *ast.InterfaceType:
			// TODO: services
		case *ast.Ident:
			enum, err := g.protoEnum(ts)
			if err != nil {
				return err
			}
			g.pfile.EnumType = append(g.pfile.EnumType, enum)
		default:
			return fmt.Errorf("invalid declaration type %T", ts.Type)
		}
	}
	return nil
}

func (g *gunkGen) addDoc(doc *ast.CommentGroup, path ...int32) {
	if doc == nil {
		return
	}
	if g.pfile.SourceCodeInfo == nil {
		g.pfile.SourceCodeInfo = &descriptor.SourceCodeInfo{}
	}
	g.pfile.SourceCodeInfo.Location = append(g.pfile.SourceCodeInfo.Location,
		&descriptor.SourceCodeInfo_Location{
			Path:            path,
			LeadingComments: proto.String(doc.Text()),
		},
	)
}

func (g *gunkGen) protoMessage(tspec *ast.TypeSpec) (*descriptor.DescriptorProto, error) {
	g.addDoc(tspec.Doc, messagePath, g.msgIndex)
	msg := &descriptor.DescriptorProto{
		Name: &tspec.Name.Name,
	}
	stype := tspec.Type.(*ast.StructType)
	for i, field := range stype.Fields.List {
		if len(field.Names) != 1 {
			return nil, fmt.Errorf("need all fields to have one name")
		}
		g.addDoc(field.Doc, messagePath, g.msgIndex, messageFieldPath, int32(i))
		pfield := &descriptor.FieldDescriptorProto{
			Name:   &field.Names[0].Name,
			Number: protoNumber(field.Tag),
		}
		switch ptype, tname := protoType(field.Type); ptype {
		case 0:
			return nil, fmt.Errorf("unsupported field type: %v", field.Type)
		case descriptor.FieldDescriptorProto_TYPE_ENUM:
			pfield.Type = &ptype
			pfield.TypeName = &tname
		default:
			pfield.Type = &ptype
		}
		msg.Field = append(msg.Field, pfield)
	}
	g.msgIndex++
	return msg, nil
}

func (g *gunkGen) protoEnum(tspec *ast.TypeSpec) (*descriptor.EnumDescriptorProto, error) {
	g.addDoc(tspec.Doc, enumPath, g.enumIndex)
	enum := &descriptor.EnumDescriptorProto{
		Name: &tspec.Name.Name,
	}
	for _, decl := range g.gfile.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		// TODO: don't force iotas, use go/types for constant
		// folding
		iotaVal := 0
		carryType := false
		for _, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			ident, ok := vs.Type.(*ast.Ident)
			if carryType && ok {
				carryType = false
			}
			if !carryType {
				carryType = ok && ident.Name == *enum.Name
			}
			if !carryType {
				continue
				iotaVal = 0
			}
			for _, name := range vs.Names {
				enum.Value = append(enum.Value, &descriptor.EnumValueDescriptorProto{
					Name:   &name.Name,
					Number: proto.Int32(int32(iotaVal)),
				})
				iotaVal++
			}
		}
	}
	g.enumIndex++
	return enum, nil
}

func protoNumber(fieldTag *ast.BasicLit) *int32 {
	if fieldTag == nil {
		return nil
	}
	str, _ := strconv.Unquote(fieldTag.Value)
	tag := reflect.StructTag(str)
	number, _ := strconv.Atoi(tag.Get("pb"))
	return proto.Int32(int32(number))
}

func protoType(from ast.Expr) (descriptor.FieldDescriptorProto_Type, string) {
	switch x := from.(type) {
	case *ast.Ident:
		switch x.Name {
		case "string":
			return descriptor.FieldDescriptorProto_TYPE_STRING, ""
		default:
			return descriptor.FieldDescriptorProto_TYPE_ENUM, "." + x.Name
		}
	}
	return 0, ""
}

const (
	// tag numbers in FileDescriptorProto
	packagePath = 2 // package
	messagePath = 4 // message_type
	enumPath    = 5 // enum_type
	// tag numbers in DescriptorProto
	messageFieldPath   = 2 // field
	messageMessagePath = 3 // nested_type
	messageEnumPath    = 4 // enum_type
	messageOneofPath   = 8 // oneof_decl
	// tag numbers in EnumDescriptorProto
	enumValuePath = 2 // value
)
