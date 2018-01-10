package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
	_ "github.com/golang/protobuf/protoc-gen-go/grpc"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

func main() {
	flag.Parse()
	for _, path := range flag.Args() {
		if err := runPkg(path); err != nil {
			log.Fatal(err)
		}
	}
}

func runPkg(path string) error {
	// TODO: we don't error if the dir does not exist
	files, err := filepath.Glob(filepath.Join(path, "*.gunk"))
	if err != nil {
		return err
	}
	t := translator{
		fset:  token.NewFileSet(),
		files: make(map[string]*ast.File),
		tconfig: &types.Config{
			Importer: dummyImporter{},
		},
	}
	if err := t.addPkg(files...); err != nil {
		return err
	}
	for _, path := range files {
		t.genFile(path, true)
	}
	if err := t.loadProtoDeps(); err != nil {
		return err
	}

	g := generator.New()
	g.Request = t.request()
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

type translator struct {
	gfile *ast.File
	pfile *descriptor.FileDescriptorProto

	fset    *token.FileSet
	files   map[string]*ast.File
	tconfig *types.Config
	pkg     *types.Package
	info    *types.Info

	toGen  []string
	pfiles []*descriptor.FileDescriptorProto

	msgIndex  int32
	enumIndex int32
}

func (t *translator) request() *plugin.CodeGeneratorRequest {
	return &plugin.CodeGeneratorRequest{
		Parameter:      proto.String("plugins=grpc"),
		FileToGenerate: t.toGen,
		ProtoFile:      t.pfiles,
	}
}

func (t *translator) addPkg(paths ...string) error {
	// TODO: support multiple packages
	var list []*ast.File
	name := "default"
	for _, path := range paths {
		file, err := parser.ParseFile(t.fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		name = file.Name.Name
		t.files[path] = file
		list = append(list, file)
	}
	t.pkg = types.NewPackage(name, name)
	t.info = &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	check := types.NewChecker(t.tconfig, t.fset, t.pkg, t.info)
	if err := check.Files(list); err != nil {
		return err
	}
	return nil
}

type dummyImporter struct{}

func (dummyImporter) Import(pkgPath string) (*types.Package, error) {
	name := path.Base(pkgPath)
	return types.NewPackage(pkgPath, name), nil
}

func (t *translator) genFile(path string, toGenerate bool) error {
	t.gfile = t.files[path]
	t.pfile = &descriptor.FileDescriptorProto{
		Name:   &path,
		Syntax: proto.String("proto3"),
	}
	t.addDoc(t.gfile.Doc, "", packagePath)
	for _, decl := range t.gfile.Decls {
		if err := t.decl(decl); err != nil {
			return err
		}
	}
	if toGenerate {
		t.toGen = append(t.toGen, path)
	}
	t.pfiles = append(t.pfiles, t.pfile)
	return nil
}

func (t *translator) decl(decl ast.Decl) error {
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
			msg, err := t.protoMessage(ts)
			if err != nil {
				return err
			}
			t.pfile.MessageType = append(t.pfile.MessageType, msg)
		case *ast.InterfaceType:
			srv, err := t.protoService(ts)
			if err != nil {
				return err
			}
			t.pfile.Service = append(t.pfile.Service, srv)
		case *ast.Ident:
			enum, err := t.protoEnum(ts)
			if err != nil {
				return err
			}
			t.pfile.EnumType = append(t.pfile.EnumType, enum)
		default:
			return fmt.Errorf("invalid declaration type %T", ts.Type)
		}
	}
	return nil
}

func (t *translator) addDoc(doc *ast.CommentGroup, prefix string, path ...int32) {
	if doc == nil {
		return
	}
	if t.pfile.SourceCodeInfo == nil {
		t.pfile.SourceCodeInfo = &descriptor.SourceCodeInfo{}
	}
	t.pfile.SourceCodeInfo.Location = append(t.pfile.SourceCodeInfo.Location,
		&descriptor.SourceCodeInfo_Location{
			Path:            path,
			LeadingComments: proto.String(prefix + doc.Text()),
		},
	)
}

func (t *translator) protoMessage(tspec *ast.TypeSpec) (*descriptor.DescriptorProto, error) {
	t.addDoc(tspec.Doc, "", messagePath, t.msgIndex)
	msg := &descriptor.DescriptorProto{
		Name: &tspec.Name.Name,
	}
	stype := tspec.Type.(*ast.StructType)
	for i, field := range stype.Fields.List {
		if len(field.Names) != 1 {
			return nil, fmt.Errorf("need all fields to have one name")
		}
		t.addDoc(field.Doc, "", messagePath, t.msgIndex, messageFieldPath, int32(i))
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
	t.msgIndex++
	return msg, nil
}

func (t *translator) protoService(tspec *ast.TypeSpec) (*descriptor.ServiceDescriptorProto, error) {
	srv := &descriptor.ServiceDescriptorProto{
		Name: &tspec.Name.Name,
	}
	itype := tspec.Type.(*ast.InterfaceType)
	for _, method := range itype.Methods.List {
		if len(method.Names) != 1 {
			return nil, fmt.Errorf("need all methods to have one name")
		}
		pmethod := &descriptor.MethodDescriptorProto{
			Name: &method.Names[0].Name,
		}
		sign := method.Type.(*ast.FuncType)
		var err error
		pmethod.InputType, err = t.protoParamType(sign.Params)
		if err != nil {
			return nil, err
		}
		pmethod.OutputType, err = t.protoParamType(sign.Results)
		if err != nil {
			return nil, err
		}
		srv.Method = append(srv.Method, pmethod)
	}
	return srv, nil
}

func (t *translator) addProtoDep(path string) {
	for _, dep := range t.pfile.Dependency {
		if dep == path {
			return // already in there
		}
	}
	t.pfile.PublicDependency = append(t.pfile.PublicDependency,
		int32(len(t.pfile.Dependency)))
	t.pfile.Dependency = append(t.pfile.Dependency, path)
}

func (t *translator) loadProtoDeps() error {
	missing := make(map[string]bool)
	for _, pfile := range t.pfiles {
		for _, dep := range pfile.Dependency {
			missing[dep] = true
		}
	}
	tmpl := template.Must(template.New("letter").Parse(`
syntax = "proto3";

{{ range $dep, $_ := . }}import "{{ $dep }}";
{{ end }}
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
		return err
	}
	var fset descriptor.FileDescriptorSet
	if err := proto.Unmarshal(out, &fset); err != nil {
		return err
	}
	for _, pfile := range fset.File {
		t.pfiles = append(t.pfiles, pfile)
	}
	return nil
}

func (t *translator) protoParamType(fields *ast.FieldList) (*string, error) {
	if fields == nil || len(fields.List) == 0 {
		t.addProtoDep("google/protobuf/empty.proto")
		return proto.String(".google.protobuf.Empty"), nil
	}
	if len(fields.List) > 1 {
		return nil, fmt.Errorf("need all methods to have <=1 results")
	}
	field := fields.List[0]
	_, tname := protoType(field.Type)
	if tname == "" {
		return nil, fmt.Errorf("could not get type for %v", field.Type)
	}
	return &tname, nil
}

func (t *translator) protoEnum(tspec *ast.TypeSpec) (*descriptor.EnumDescriptorProto, error) {
	t.addDoc(tspec.Doc, "", enumPath, t.enumIndex)
	enum := &descriptor.EnumDescriptorProto{
		Name: &tspec.Name.Name,
	}
	enumType := t.info.TypeOf(tspec.Name)
	for _, decl := range t.gfile.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for i, spec := range gd.Specs {
			vs := spec.(*ast.ValueSpec)
			// .proto files have the same limitation, and it
			// allows per-value godocs
			if len(vs.Names) != 1 {
				return nil, fmt.Errorf("need all value specs to define one name")
			}
			name := vs.Names[0]
			if t.info.TypeOf(name) != enumType {
				continue
			}
			// SomeVal will be exported as SomeType_SomeVal
			t.addDoc(vs.Doc, tspec.Name.Name+"_",
				enumPath, t.enumIndex, enumValuePath, int32(i))
			val := t.info.Defs[name].(*types.Const).Val()
			ival, _ := constant.Int64Val(val)
			enum.Value = append(enum.Value, &descriptor.EnumValueDescriptorProto{
				Name:   &name.Name,
				Number: proto.Int32(int32(ival)),
			})
		}
	}
	t.enumIndex++
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
			return descriptor.FieldDescriptorProto_TYPE_STRING, x.Name
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
