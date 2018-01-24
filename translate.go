package main

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	desc "github.com/golang/protobuf/protoc-gen-go/descriptor"
)

// translatePkg translates all the gunk files in a gunk package to the
// proto language. All the files within the package, including all the
// files for its transitive dependencies, must already be loaded via
// addPkg.
func (t *translator) translatePkg(path string) error {
	t.tpkg = t.typPkgs[path]
	astFiles := t.astPkgs[path]
	for file := range astFiles {
		if err := t.translateFile(path, file); err != nil {
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
			if imp.Name != nil && imp.Name.Name == "_" {
				continue
			}
			opath, _ := strconv.Unquote(imp.Path.Value)
			for oname := range t.astPkgs[opath] {
				pfile.Dependency = append(pfile.Dependency, oname)
			}
		}
	}
	return nil
}

// translateFile translates a single gunk file to a proto file.
func (t *translator) translateFile(path, file string) error {
	t.msgIndex = 0
	t.srvIndex = 0
	t.enumIndex = 0
	t.toGen[path][file] = true
	if _, ok := t.allProto[file]; ok {
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
		if err := t.translateDecl(decl); err != nil {
			return err
		}
	}
	t.allProto[file] = t.pfile
	return nil
}

// translateDecl translates a top-level declaration in a gunk file. It
// only acts on type declarations; struct types become proto messages,
// interfaces become services, and basic integer types become enums.
func (t *translator) translateDecl(decl ast.Decl) error {
	gd, ok := decl.(*ast.GenDecl)
	if !ok {
		return fmt.Errorf("invalid declaration %T", decl)
	}
	switch gd.Tok {
	case token.TYPE:
		// continue below
	case token.CONST:
		return nil // used for enums
	case token.IMPORT:
		return nil // imports; ignore
	default:
		return fmt.Errorf("invalid declaration token %v", gd.Tok)
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
		tag := ""
		fn := func(text string) string {
			text, tag = splitGunkTag(text)
			return text
		}
		t.addDoc(method.Doc, fn, servicePath, t.srvIndex,
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
		if tag != "" {
			edesc, val, err := t.interpretTagValue(tag)
			if err != nil {
				return nil, err
			}
			if pmethod.Options == nil {
				pmethod.Options = &desc.MethodOptions{}
			}
			// TODO: actually use the
			// protoc-gen-grpc-gateway to make this do
			// something
			if err := proto.SetExtension(pmethod.Options, edesc, val); err != nil {
				return nil, err
			}
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

func splitGunkTag(text string) (doc, tag string) {
	lines := strings.Split(text, "\n")
	var tagLines []string
	for i, line := range lines {
		if strings.HasPrefix(line, "+gunk ") {
			tagLines = lines[i:]
			tagLines[0] = strings.TrimPrefix(tagLines[0], "+gunk ")
			lines = lines[:i]
			break
		}
	}
	doc = strings.TrimSpace(strings.Join(lines, "\n"))
	tag = strings.TrimSpace(strings.Join(tagLines, "\n"))
	return
}

func (t *translator) interpretTagValue(tag string) (*proto.ExtensionDesc, interface{}, error) {
	// use Eval to resolve the type, and check for any errors in the
	// value expression
	tv, err := types.Eval(t.fset, t.tpkg, t.gfile.End(), tag)
	if err != nil {
		return nil, nil, err
	}
	switch s := tv.Type.String(); s {
	case "github.com/gunk/opt/http.Match":
		// an error would be caught in Eval
		expr, _ := parser.ParseExpr(tag)
		rule := &httpRule{}
		for _, elt := range expr.(*ast.CompositeLit).Elts {
			kv := elt.(*ast.KeyValueExpr)
			val, _ := strconv.Unquote(kv.Value.(*ast.BasicLit).Value)
			method := "GET"
			switch name := kv.Key.(*ast.Ident).Name; name {
			case "Method":
				method = val
			case "Path":
				switch method {
				case "GET":
					rule.Get = val
				case "POST":
					rule.Post = val
				}
			case "Body":
				rule.Body = val
			}
		}
		edesc := &proto.ExtensionDesc{
			Field:         72295728,
			Tag:           "varint,72295728",
			ExtendedType:  (*desc.MethodOptions)(nil),
			ExtensionType: (*httpRule)(nil),
		}
		return edesc, rule, nil
	default:
		return nil, nil, fmt.Errorf("unknown option type: %s", s)
	}
}

type httpRule struct {
	Get  string `protobuf:"bytes,2"`
	Post string `protobuf:"bytes,3"`
	Body string `protobuf:"bytes,7"`
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
