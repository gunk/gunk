package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/build"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	desc "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/golang/protobuf/protoc-gen-go/generator"
	_ "github.com/golang/protobuf/protoc-gen-go/grpc"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
)

func main() {
	flag.Parse()
	if err := runPaths("", flag.Args()...); err != nil {
		log.Fatal(err)
	}
}

func runPaths(gopath string, paths ...string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	t := translator{
		wd:      wd,
		fset:    token.NewFileSet(),
		tconfig: &types.Config{},
		info: &types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
			Defs:  make(map[*ast.Ident]types.Object),
			Uses:  make(map[*ast.Ident]types.Object),
		},
		bldPkgs:  make(map[string]*build.Package),
		typPkgs:  make(map[string]*types.Package),
		astPkgs:  make(map[string]map[string]*ast.File),
		toGen:    make(map[string]map[string]bool),
		allProto: make(map[string]*desc.FileDescriptorProto),
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
		if err := t.transPkg(path); err != nil {
			return err
		}
	}
	if err := t.loadProtoDeps(); err != nil {
		return err
	}
	for _, path := range paths {
		if err := t.genPkg(path); err != nil {
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

	toGen    map[string]map[string]bool
	allProto map[string]*desc.FileDescriptorProto

	msgIndex  int32
	srvIndex  int32
	enumIndex int32
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
	sort.Slice(req.ProtoFile, func(i, j int) bool {
		f1, f2 := req.ProtoFile[i], req.ProtoFile[j]
		return *f1.Name < *f2.Name
	})
	return req
}

func (t *translator) genPkg(path string) error {
	g := generator.New()
	g.Request = t.requestForPkg(path)
	g.CommandLineParameters(g.Request.GetParameter())

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

func (t *translator) addPkg(path string) error {
	bpkg, err := t.bctx.Import(path, t.wd, build.FindOnly)
	if err != nil {
		return err
	}
	// TODO: we don't error if the dir does not exist
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
		astFiles[match] = file
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

func (t *translator) Import(path string) (*types.Package, error) {
	if tpkg := t.typPkgs[path]; tpkg != nil {
		return tpkg, nil
	}
	if err := t.addPkg(path); err != nil {
		return nil, err
	}
	return t.typPkgs[path], nil
}

func (t *translator) transPkg(path string) error {
	t.tpkg = t.typPkgs[path]
	astFiles := t.astPkgs[path]
	for file := range astFiles {
		if err := t.transFile(path, file, true); err != nil {
			return err
		}
	}
	for name, gfile := range astFiles {
		pfile := t.allProto[name]
		for oname := range astFiles {
			if name != oname {
				pfile.Dependency = append(pfile.Dependency, oname)
			}
		}
		for _, imp := range gfile.Imports {
			opath, _ := strconv.Unquote(imp.Path.Value)
			for oname := range t.astPkgs[opath] {
				pfile.Dependency = append(pfile.Dependency, oname)
				t.transFile(opath, oname, false)
			}
		}
	}
	return nil
}

func (t *translator) transFile(path, file string, toGenerate bool) error {
	if _, ok := t.allProto[file]; ok {
		if toGenerate {
			// if it was a dependency first and passed as an
			// argument later, do generate it
			t.toGen[path][file] = true
		}
		return nil
	}
	bpkg := t.bldPkgs[path]
	astFiles := t.astPkgs[path]
	t.gfile = astFiles[file]
	t.pfile = &desc.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String(file),
		Package: proto.String(bpkg.ImportPath),
		Options: &desc.FileOptions{
			GoPackage: proto.String(bpkg.Name),
		},
	}
	t.addDoc(t.gfile.Doc, nil, packagePath)
	for _, decl := range t.gfile.Decls {
		if err := t.decl(decl); err != nil {
			return err
		}
	}
	t.toGen[path][file] = toGenerate
	t.allProto[file] = t.pfile
	return nil
}

func (t *translator) decl(decl ast.Decl) error {
	gd, ok := decl.(*ast.GenDecl)
	if !ok {
		return fmt.Errorf("invalid declaration %T", decl)
	}
	switch gd.Tok {
	case token.TYPE: // below
	case token.CONST: // for enums
		break
	case token.IMPORT: // imports
	default:
		return fmt.Errorf("invalid declaration token %v", gd.Tok)
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

func (t *translator) addDoc(doc *ast.CommentGroup, f func(string) string, path ...int32) {
	if doc == nil {
		return
	}
	if t.pfile.SourceCodeInfo == nil {
		t.pfile.SourceCodeInfo = &desc.SourceCodeInfo{}
	}
	text := doc.Text()
	if f != nil {
		text = f(text)
	}
	t.pfile.SourceCodeInfo.Location = append(t.pfile.SourceCodeInfo.Location,
		&desc.SourceCodeInfo_Location{
			Path:            path,
			LeadingComments: proto.String(text),
		},
	)
}

func (t *translator) protoMessage(tspec *ast.TypeSpec) (*desc.DescriptorProto, error) {
	t.addDoc(tspec.Doc, nil, messagePath, t.msgIndex)
	msg := &desc.DescriptorProto{
		Name: proto.String(tspec.Name.Name),
	}
	stype := tspec.Type.(*ast.StructType)
	for i, field := range stype.Fields.List {
		if len(field.Names) != 1 {
			return nil, fmt.Errorf("need all fields to have one name")
		}
		t.addDoc(field.Doc, nil, messagePath, t.msgIndex, messageFieldPath, int32(i))
		pfield := &desc.FieldDescriptorProto{
			Name:   proto.String(field.Names[0].Name),
			Number: protoNumber(field.Tag),
		}
		switch ptype, tname := t.protoType(field.Type, nil); ptype {
		case 0:
			return nil, fmt.Errorf("unsupported field type: %v", field.Type)
		case desc.FieldDescriptorProto_TYPE_ENUM:
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

func (t *translator) protoService(tspec *ast.TypeSpec) (*desc.ServiceDescriptorProto, error) {
	srv := &desc.ServiceDescriptorProto{
		Name: proto.String(tspec.Name.Name),
	}
	itype := tspec.Type.(*ast.InterfaceType)
	for i, method := range itype.Methods.List {
		if len(method.Names) != 1 {
			return nil, fmt.Errorf("need all methods to have one name")
		}
		t.addDoc(method.Doc, stripGunkTags, servicePath, t.srvIndex,
			serviceMethodPath, int32(i))
		pmethod := &desc.MethodDescriptorProto{
			Name: proto.String(method.Names[0].Name),
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
	t.srvIndex++
	return srv, nil
}

func namePrefix(name string) func(string) string {
	return func(text string) string {
		return name + "_" + text
	}
}

func stripGunkTags(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "+gunk") {
			lines = lines[:i]
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
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
	for _, pfile := range t.allProto {
		for _, dep := range pfile.Dependency {
			if _, e := t.allProto[dep]; !e {
				missing[dep] = true
			}
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

func (t *translator) protoParamType(fields *ast.FieldList) (*string, error) {
	if fields == nil || len(fields.List) == 0 {
		t.addProtoDep("google/protobuf/empty.proto")
		return proto.String(".google.protobuf.Empty"), nil
	}
	if len(fields.List) > 1 {
		return nil, fmt.Errorf("need all methods to have <=1 results")
	}
	field := fields.List[0]
	_, tname := t.protoType(field.Type, nil)
	if tname == "" {
		return nil, fmt.Errorf("could not get type for %v", field.Type)
	}
	return &tname, nil
}

func (t *translator) protoEnum(tspec *ast.TypeSpec) (*desc.EnumDescriptorProto, error) {
	t.addDoc(tspec.Doc, nil, enumPath, t.enumIndex)
	enum := &desc.EnumDescriptorProto{
		Name: proto.String(tspec.Name.Name),
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
			t.addDoc(vs.Doc, namePrefix(tspec.Name.Name),
				enumPath, t.enumIndex, enumValuePath, int32(i))
			val := t.info.Defs[name].(*types.Const).Val()
			ival, _ := constant.Int64Val(val)
			enum.Value = append(enum.Value, &desc.EnumValueDescriptorProto{
				Name:   proto.String(name.Name),
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

func (t *translator) protoType(expr ast.Expr, pkg *types.Package) (desc.FieldDescriptorProto_Type, string) {
	if pkg == nil {
		pkg = t.tpkg
	}
	switch x := expr.(type) {
	case *ast.Ident:
		switch x.Name {
		case "string":
			return desc.FieldDescriptorProto_TYPE_STRING, x.Name
		default:
			fullName := "." + pkg.Path() + "." + x.Name
			return desc.FieldDescriptorProto_TYPE_ENUM, fullName
		}
	case *ast.SelectorExpr:
		id, ok := x.X.(*ast.Ident)
		if !ok {
			break
		}
		pkg := t.info.ObjectOf(id).(*types.PkgName).Imported()
		return t.protoType(x.Sel, pkg)
	}
	return 0, ""
}

const (
	packagePath       = 2 // FileDescriptorProto.Package
	messagePath       = 4 // FileDescriptorProto.MessageType
	enumPath          = 5 // FileDescriptorProto.EnumType
	servicePath       = 6 // FileDescriptorProto.Service
	messageFieldPath  = 2 // DescriptorProto.Field
	enumValuePath     = 2 // EnumDescriptorProto.Value
	serviceMethodPath = 2 // ServiceDescriptorProto.Method
)
