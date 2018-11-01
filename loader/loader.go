package loader

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"html/template"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/golang/protobuf/proto"
	desc "github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
	"google.golang.org/genproto/googleapis/api/annotations"
)

// Load loads the Gunk packages on the provided patterns from the given dir, and
// generates the corresponding proto files. Similar to Go, if a path begins with
// ".", it is interpreted as a file system path where a package is located, and
// "..." patterns are supported.
func Load(dir string, patterns ...string) error {
	// First, translate the patterns to package paths.
	cfg := &packages.Config{
		Dir:  dir,
		Mode: packages.LoadFiles,
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return err
	}
	pkgPaths := make([]string, len(pkgs))
	for i, pkg := range pkgs {
		pkgPaths[i] = pkg.PkgPath
	}

	l, err := New(dir, pkgPaths...)
	if err != nil {
		return err
	}

	for _, path := range pkgPaths {
		if err := l.GeneratePkg(path); err != nil {
			return err
		}
	}

	return nil
}

// Loader
type Loader struct {
	dir string // if empty, uses the current directory

	gfile *ast.File
	pfile *desc.FileDescriptorProto
	tpkg  *types.Package

	fset    *token.FileSet
	tconfig *types.Config
	info    *types.Info

	// Maps from package import path to package information.
	loadPkgs map[string]*packages.Package
	astPkgs  map[string]map[string]*ast.File
	typePkgs map[string]*types.Package

	toGen     map[string]map[string]bool
	allProto  map[string]*desc.FileDescriptorProto
	origPaths map[string]string

	messageIndex int32
	serviceIndex int32
	enumIndex    int32
}

// New creates a Gunk loader for the specified working directory.
func New(dir string, paths ...string) (*Loader, error) {
	l := &Loader{
		dir:  dir,
		fset: token.NewFileSet(),
		tconfig: &types.Config{
			DisableUnusedImportCheck: true,
		},
		info: &types.Info{
			Types: make(map[ast.Expr]types.TypeAndValue),
			Defs:  make(map[*ast.Ident]types.Object),
			Uses:  make(map[*ast.Ident]types.Object),
		},

		loadPkgs:  make(map[string]*packages.Package),
		typePkgs:  make(map[string]*types.Package),
		astPkgs:   make(map[string]map[string]*ast.File),
		toGen:     make(map[string]map[string]bool),
		allProto:  make(map[string]*desc.FileDescriptorProto),
		origPaths: make(map[string]string),
	}
	l.tconfig.Importer = l

	for _, path := range paths {
		if err := l.addPkg(path); err != nil {
			return nil, err
		}
		if err := l.translatePkg(path); err != nil {
			return nil, err
		}
	}
	if err := l.loadProtoDeps(); err != nil {
		return nil, err
	}

	return l, nil
}

// GeneratePkg runs the proto files resulting from translating gunk packages
// through a code generator, such as protoc-gen-go to generate Go packages.
//
// Generated files are written to the same directory, next to the source gunk
// files.
//
// It is fine to pass the plugin.CodeGeneratorRequest to every protoc generator
// unaltered; this is what protoc does when calling out to the generators and
// the generators should already handle the case where they have nothing to do.
func (l *Loader) GeneratePkg(path string) error {
	req := l.requestForPkg(path)
	if err := l.generatePluginGo(*req); err != nil {
		return fmt.Errorf("error generating plugin go: %v", err)
	}
	if err := l.generatePluginGrpcGateway(*req); err != nil {
		return fmt.Errorf("error generating plugin grpc-gateway: %v", err)
	}
	return nil
}

func (l *Loader) generatePluginGo(req plugin.CodeGeneratorRequest) error {
	req.Parameter = proto.String("plugins=grpc")
	bs, err := proto.Marshal(&req)
	if err != nil {
		return err
	}

	cmd := exec.Command("protoc-gen-go")
	cmd.Stdin = bytes.NewReader(bs)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error executing protoc-gen-go: %s, %v", out, err)
	}
	var resp plugin.CodeGeneratorResponse
	if err := proto.Unmarshal(out, &resp); err != nil {
		return err
	}
	for _, rf := range resp.File {
		// to turn foo.gunk.pb.go into foo.pb.go
		inPath := strings.Replace(*rf.Name, ".pb.go", "", 1)
		outPath := l.origPaths[inPath]
		outPath = strings.Replace(outPath, ".gunk", ".pb.go", 1)
		data := []byte(*rf.Content)
		if err := ioutil.WriteFile(outPath, data, 0644); err != nil {
			return fmt.Errorf("unable to write to file %q: %v", outPath, err)
		}
	}
	return nil
}

func (l *Loader) generatePluginGrpcGateway(req plugin.CodeGeneratorRequest) error {
	bs, err := proto.Marshal(&req)
	if err != nil {
		return err
	}

	cmd := exec.Command("protoc-gen-grpc-gateway")
	cmd.Stdin = bytes.NewReader(bs)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("error executing protoc-gen-grpc-gateway: %s, %v", out, err)
	}
	var resp plugin.CodeGeneratorResponse
	if err := proto.Unmarshal(out, &resp); err != nil {
		return err
	}
	if rerr := resp.GetError(); rerr != "" {
		return fmt.Errorf("error executing protoc-gen-grpc-gateway: %v", rerr)
	}
	for _, rf := range resp.File {
		// to turn foo.gunk.pb.gw.go into foo.pb.gw.go
		inPath := strings.Replace(*rf.Name, ".pb.gw.go", ".gunk", 1)
		outPath := l.origPaths[inPath]
		outPath = strings.Replace(outPath, ".gunk", ".pb.gw.go", 1)
		data := []byte(*rf.Content)
		if err := ioutil.WriteFile(outPath, data, 0644); err != nil {
			return fmt.Errorf("unable to write to file %q: %v", outPath, err)
		}
	}
	return nil
}

func (l *Loader) requestForPkg(path string) *plugin.CodeGeneratorRequest {
	req := &plugin.CodeGeneratorRequest{}
	for file := range l.toGen[path] {
		req.FileToGenerate = append(req.FileToGenerate, file)
	}
	for _, pfile := range l.allProto {
		req.ProtoFile = append(req.ProtoFile, pfile)
	}

	// Sort all the files by name to get deterministic output. For example,
	// the first file in each package gets an extra package godoc from
	// protoc-gen-go. And protoc-gen-grpc-gateway cares about the order of
	// the ProtoFile list.
	sort.Strings(req.FileToGenerate)
	sort.Slice(req.ProtoFile, func(i, j int) bool {
		return *req.ProtoFile[i].Name < *req.ProtoFile[j].Name
	})

	return req
}

// translatePkg translates all the gunk files in a gunk package to the
// proto language. All the files within the package, including all the
// files for its transitive dependencies, must already be loaded via
// addPkg.
func (l *Loader) translatePkg(path string) error {
	l.tpkg = l.typePkgs[path]
	astFiles := l.astPkgs[path]
	for file := range astFiles {
		if err := l.translateFile(path, file); err != nil {
			return err
		}
	}

	for name, gfile := range astFiles {
		pfile := l.allProto[name]
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
			for oname := range l.astPkgs[opath] {
				pfile.Dependency = append(pfile.Dependency, oname)
			}
		}
	}
	return nil
}

// translateFile translates a single gunk file to a proto file.
func (l *Loader) translateFile(path, file string) error {
	l.messageIndex = 0
	l.serviceIndex = 0
	l.enumIndex = 0
	l.toGen[path][file] = true
	if _, ok := l.allProto[file]; ok {
		return nil
	}
	lpkg := l.loadPkgs[path]
	astFiles := l.astPkgs[path]
	l.gfile = astFiles[file]
	l.pfile = &desc.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String(file),
		Package: proto.String(lpkg.PkgPath),
		Options: &desc.FileOptions{
			GoPackage: proto.String(lpkg.Name),
		},
	}
	l.addDoc(l.gfile.Doc, nil, packagePath)
	for _, decl := range l.gfile.Decls {
		if err := l.translateDecl(decl); err != nil {
			return err
		}
	}
	l.allProto[file] = l.pfile
	return nil
}

// translateDecl translates a top-level declaration in a gunk file. It
// only acts on type declarations; struct types become proto messages,
// interfaces become services, and basic integer types become enums.
func (l *Loader) translateDecl(decl ast.Decl) error {
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
			msg, err := l.convertMessage(ts)
			if err != nil {
				return err
			}
			l.pfile.MessageType = append(l.pfile.MessageType, msg)
		case *ast.InterfaceType:
			srv, err := l.convertService(ts)
			if err != nil {
				return err
			}
			l.pfile.Service = append(l.pfile.Service, srv)
		case *ast.Ident:
			enum, err := l.convertEnum(ts)
			if err != nil {
				return err
			}
			l.pfile.EnumType = append(l.pfile.EnumType, enum)
		default:
			return fmt.Errorf("invalid declaration type %T", ts.Type)
		}
	}
	return nil
}

func (l *Loader) addDoc(doc *ast.CommentGroup, transform func(string) string, path ...int32) {
	if doc == nil {
		return
	}
	if l.pfile.SourceCodeInfo == nil {
		l.pfile.SourceCodeInfo = &desc.SourceCodeInfo{}
	}
	text := doc.Text()
	if transform != nil {
		text = transform(text)
	}
	l.pfile.SourceCodeInfo.Location = append(l.pfile.SourceCodeInfo.Location,
		&desc.SourceCodeInfo_Location{
			Path:            path,
			LeadingComments: proto.String(text),
		},
	)
}

func (l *Loader) convertMessage(tspec *ast.TypeSpec) (*desc.DescriptorProto, error) {
	l.addDoc(tspec.Doc, nil, messagePath, l.messageIndex)
	msg := &desc.DescriptorProto{
		Name: proto.String(tspec.Name.Name),
	}
	stype := tspec.Type.(*ast.StructType)
	for i, field := range stype.Fields.List {
		if len(field.Names) != 1 {
			return nil, fmt.Errorf("need all fields to have one name")
		}
		fieldName := field.Names[0].Name
		l.addDoc(field.Doc, nil, messagePath, l.messageIndex, messageFieldPath, int32(i))
		ftype := l.info.TypeOf(field.Type)

		var ptype desc.FieldDescriptorProto_Type
		var plabel desc.FieldDescriptorProto_Label
		var tname string
		var msgNestedType *desc.DescriptorProto

		// Check to see if the type is a map. Maps need to be made into a
		// repeated nested message containing key and value fields.
		if mtype, ok := ftype.(*types.Map); ok {
			ptype = desc.FieldDescriptorProto_TYPE_MESSAGE
			plabel = desc.FieldDescriptorProto_LABEL_REPEATED
			tname, msgNestedType = l.convertMap(tspec.Name.Name, fieldName, mtype)
			msg.NestedType = append(msg.NestedType, msgNestedType)
		} else {
			ptype, plabel, tname = l.convertType(ftype)
		}
		if ptype == 0 {
			return nil, fmt.Errorf("unsupported field type: %v", ftype)
		}

		// Check that the struct field has a tag. We currently
		// require all struct fields to have a tag; this is used
		// to assign the position number for a field, ie: `pb:"1"`
		if field.Tag == nil {
			return nil, fmt.Errorf("missing required tag on %s", fieldName)
		}
		// Can skip the error here because we've already parsed the file.
		str, _ := strconv.Unquote(field.Tag.Value)
		tag := reflect.StructTag(str)
		// TODO: record the position numbers used so we can return an
		// error if position number is used more than once? This would
		// also allow us to automatically assign fields a position
		// number if it is missing one.
		num, err := protoNumber(tag)
		if err != nil {
			return nil, fmt.Errorf("unable to convert tag to number on %s: %v", fieldName, err)
		}

		// TODO: We aren't currently setting the JsonName field, not sure what we want
		// to set them to? Possibly pull out a json tag or something?
		msg.Field = append(msg.Field, &desc.FieldDescriptorProto{
			Name:     proto.String(fieldName),
			Number:   num,
			TypeName: &tname,
			Type:     &ptype,
			Label:    &plabel,
		})
	}
	l.messageIndex++
	return msg, nil
}

func (l *Loader) convertService(tspec *ast.TypeSpec) (*desc.ServiceDescriptorProto, error) {
	srv := &desc.ServiceDescriptorProto{
		Name: proto.String(tspec.Name.Name),
	}
	itype := tspec.Type.(*ast.InterfaceType)
	for i, method := range itype.Methods.List {
		if len(method.Names) != 1 {
			return nil, fmt.Errorf("need all methods to have one name")
		}
		tag := ""
		stripTag := func(text string) string {
			text, tag = splitGunkTag(text)
			return text
		}
		l.addDoc(method.Doc, stripTag, servicePath, l.serviceIndex,
			serviceMethodPath, int32(i))
		pmethod := &desc.MethodDescriptorProto{
			Name: proto.String(method.Names[0].Name),
		}
		sign := l.info.TypeOf(method.Type).(*types.Signature)
		var err error
		pmethod.InputType, err = l.convertParameter(sign.Params())
		if err != nil {
			return nil, err
		}
		pmethod.OutputType, err = l.convertParameter(sign.Results())
		if err != nil {
			return nil, err
		}
		if tag != "" {
			edesc, val, err := l.interpretTagValue(tag)
			if err != nil {
				return nil, err
			}
			if pmethod.Options == nil {
				pmethod.Options = &desc.MethodOptions{}
			}
			if err := proto.SetExtension(pmethod.Options, edesc, val); err != nil {
				return nil, err
			}
		}
		srv.Method = append(srv.Method, pmethod)
	}
	l.serviceIndex++
	return srv, nil
}

// convertMap will translate a Go map to a Protobuf respresentation of a map,
// returning the nested type name and definition.
//
// Protobuf represents a map as a nested message on the parent message. This
// nested message contains two fields; key and value (map[key]value), and has
// the MapEntry option set to true.
//
// https://developers.google.com/protocol-buffers/docs/proto#maps
func (l *Loader) convertMap(parentName, fieldName string, mapTyp *types.Map) (string, *desc.DescriptorProto) {
	mapName := fieldName + "Entry"
	typeName := "." + l.tpkg.Path() + "." + parentName + "." + mapName

	keyType, _, keyTypeName := l.convertType(mapTyp.Key())
	if keyType == 0 {
		return "", nil
	}
	elemType, _, elemTypeName := l.convertType(mapTyp.Elem())
	if elemType == 0 {
		return "", nil
	}

	fieldLabel := desc.FieldDescriptorProto_LABEL_OPTIONAL
	nestedType := &desc.DescriptorProto{
		Name: proto.String(mapName),
		Options: &desc.MessageOptions{
			MapEntry: proto.Bool(true),
		},
		Field: []*desc.FieldDescriptorProto{
			{
				Name:     proto.String("key"),
				Number:   proto.Int32(1),
				Label:    &fieldLabel,
				Type:     &keyType,
				TypeName: &keyTypeName,
			},
			{
				Name:     proto.String("value"),
				Number:   proto.Int32(2),
				Label:    &fieldLabel,
				Type:     &elemType,
				TypeName: &elemTypeName,
			},
		},
	}
	return typeName, nestedType
}

func (l *Loader) interpretTagValue(tag string) (*proto.ExtensionDesc, interface{}, error) {
	// use Eval to resolve the type, and check for any errors in the
	// value expression
	tv, err := types.Eval(l.fset, l.tpkg, l.gfile.End(), tag)
	if err != nil {
		return nil, nil, err
	}
	switch s := tv.Type.String(); s {
	case "github.com/gunk/opt/http.Match":
		// an error would be caught in Eval
		expr, _ := parser.ParseExpr(tag)

		// Capture the values required to use in annotations.HttpRule.
		// We need to evaluate the entire expression, and then we can
		// create an annotations.HttpRule.
		var path string
		var body string
		method := "GET"
		for _, elt := range expr.(*ast.CompositeLit).Elts {
			kv := elt.(*ast.KeyValueExpr)
			val, _ := strconv.Unquote(kv.Value.(*ast.BasicLit).Value)
			switch name := kv.Key.(*ast.Ident).Name; name {
			case "Method":
				method = val
			case "Path":
				path = val
				// TODO: grpc-gateway doesn't allow paths with a trailing "/", should
				// we return an error here, because the error from grpc-gateway is very
				// cryptic and unhelpful?
				// https://github.com/grpc-ecosystem/grpc-gateway/issues/472
			case "Body":
				body = val
			default:
				return nil, nil, fmt.Errorf("unknown expression key %q", name)
			}
		}
		rule := &annotations.HttpRule{
			Body: body,
		}
		switch method {
		case "GET":
			rule.Pattern = &annotations.HttpRule_Get{Get: path}
		case "POST":
			rule.Pattern = &annotations.HttpRule_Post{Post: path}
		case "DELETE":
			rule.Pattern = &annotations.HttpRule_Delete{Delete: path}
		case "PUT":
			rule.Pattern = &annotations.HttpRule_Put{Put: path}
		case "PATCH":
			rule.Pattern = &annotations.HttpRule_Patch{Patch: path}
		default:
			return nil, nil, fmt.Errorf("unknown method type: %q", method)
		}
		// TODO: Add support for custom rules - HttpRule_Custom?
		return annotations.E_Http, rule, nil
	default:
		return nil, nil, fmt.Errorf("unknown option type: %s", s)
	}
}

func (l *Loader) convertParameter(tuple *types.Tuple) (*string, error) {
	switch tuple.Len() {
	case 0:
		l.addProtoDep("google/protobuf/empty.proto")
		return proto.String(".google.protobuf.Empty"), nil
	case 1:
		// below
	default:
		return nil, fmt.Errorf("multiple parameters are not supported")
	}
	param := tuple.At(0).Type()
	_, _, tname := l.convertType(param)
	if tname == "" {
		return nil, fmt.Errorf("unsupported parameter type: %v", param)
	}
	return &tname, nil
}

func (l *Loader) convertEnum(tspec *ast.TypeSpec) (*desc.EnumDescriptorProto, error) {
	l.addDoc(tspec.Doc, nil, enumPath, l.enumIndex)
	enum := &desc.EnumDescriptorProto{
		Name: proto.String(tspec.Name.Name),
	}
	enumType := l.info.TypeOf(tspec.Name)
	for _, decl := range l.gfile.Decls {
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
			if l.info.TypeOf(name) != enumType {
				continue
			}
			// SomeVal will be exported as SomeType_SomeVal
			l.addDoc(vs.Doc, namePrefix(tspec.Name.Name),
				enumPath, l.enumIndex, enumValuePath, int32(i))
			val := l.info.Defs[name].(*types.Const).Val()
			ival, _ := constant.Int64Val(val)
			enum.Value = append(enum.Value, &desc.EnumValueDescriptorProto{
				Name:   proto.String(name.Name),
				Number: proto.Int32(int32(ival)),
			})
		}
	}
	l.enumIndex++
	return enum, nil
}

// convertType converts a Go field or parameter type to Protobuf, returning its
// type descriptor, a label such as "repeated", and a name, if the final type is
// an enum or a message.
func (l *Loader) convertType(typ types.Type) (desc.FieldDescriptorProto_Type, desc.FieldDescriptorProto_Label, string) {
	switch typ := typ.(type) {
	case *types.Basic:
		// Map Go types to proto types via
		// https://developers.google.com/protocol-buffers/docs/proto3#scalar
		switch typ.Kind() {
		case types.String:
			return desc.FieldDescriptorProto_TYPE_STRING, desc.FieldDescriptorProto_LABEL_OPTIONAL, ""
		case types.Int, types.Int32:
			return desc.FieldDescriptorProto_TYPE_INT32, desc.FieldDescriptorProto_LABEL_OPTIONAL, ""
		case types.Bool:
			return desc.FieldDescriptorProto_TYPE_BOOL, desc.FieldDescriptorProto_LABEL_OPTIONAL, ""
		}
	case *types.Named:
		switch typ.String() {
		case "time.Time":
			l.addProtoDep("google/protobuf/timestamp.proto")
			return desc.FieldDescriptorProto_TYPE_MESSAGE, desc.FieldDescriptorProto_LABEL_OPTIONAL, ".google.protobuf.Timestamp"
		case "time.Duration":
			l.addProtoDep("google/protobuf/duration.proto")
			return desc.FieldDescriptorProto_TYPE_MESSAGE, desc.FieldDescriptorProto_LABEL_OPTIONAL, ".google.protobuf.Duration"
		}
		fullName := "." + typ.String()
		switch u := typ.Underlying().(type) {
		case *types.Basic:
			switch u.Kind() {
			case types.Int, types.Int32:
				return desc.FieldDescriptorProto_TYPE_ENUM, desc.FieldDescriptorProto_LABEL_OPTIONAL, fullName
			}
		case *types.Struct:
			return desc.FieldDescriptorProto_TYPE_MESSAGE, desc.FieldDescriptorProto_LABEL_OPTIONAL, fullName
		}
	case *types.Slice:
		dtyp, _, name := l.convertType(typ.Elem())
		if dtyp == 0 {
			return 0, 0, ""
		}
		return dtyp, desc.FieldDescriptorProto_LABEL_REPEATED, name
	}
	return 0, 0, ""
}

// addPkg sets up a gunk package to be translated and generated. It is
// parsed from the gunk files on disk and type-checked, gathering all
// the info needed later on.
func (l *Loader) addPkg(pkgPath string) error {
	// First, translate the patterns to package paths.
	cfg := &packages.Config{
		Dir:  l.dir,
		Mode: packages.LoadFiles,
	}
	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil {
		return err
	}
	if len(pkgs) != 1 {
		panic("expected go/packages.Load to return exactly one package")
	}
	lpkg := pkgs[0]
	if len(lpkg.Errors) > 0 {
		return lpkg.Errors[0]
	}

	pkgDir := ""
	for _, gofile := range lpkg.GoFiles {
		dir := filepath.Dir(gofile)
		if pkgDir == "" {
			pkgDir = dir
		} else if dir != pkgDir {
			return fmt.Errorf("multiple dirs for %s: %s %s",
				pkgPath, pkgDir, dir)
		}
	}

	matches, err := filepath.Glob(filepath.Join(pkgDir, "*.gunk"))
	if err != nil {
		return err
	}
	// TODO: support multiple packages
	var list []*ast.File
	pkgName := "default"
	astFiles := make(map[string]*ast.File)
	for _, match := range matches {
		file, err := parser.ParseFile(l.fset, match, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		pkgName = file.Name.Name
		// to make the generated code independent of the current
		// directory when running gunk
		relPath := pkgPath + "/" + filepath.Base(match)
		astFiles[relPath] = file
		l.origPaths[relPath] = match
		list = append(list, file)
	}
	tpkg := types.NewPackage(pkgPath, pkgName)
	check := types.NewChecker(l.tconfig, l.fset, tpkg, l.info)
	if err := check.Files(list); err != nil {
		return err
	}
	l.loadPkgs[pkgPath] = lpkg
	l.typePkgs[pkgPath] = tpkg
	l.astPkgs[pkgPath] = astFiles
	l.toGen[pkgPath] = make(map[string]bool)
	return nil
}

// Import satisfies the go/types.Importer interface.
//
// Unlike standard Go ones like go/importer and x/tools/go/packages, this one
// uses our own addPkg to instead load gunk packages.
//
// Aside from that, it is very similar to standard Go importers that load from
// source. It too uses a cache to avoid loading packages multiple times.
func (l *Loader) Import(path string) (*types.Package, error) {
	// Has it been imported and loaded before?
	if tpkg := l.typePkgs[path]; tpkg != nil {
		return tpkg, nil
	}

	// Loading a standard library package for the first time.
	if !strings.Contains(path, ".") {
		cfg := &packages.Config{Mode: packages.LoadTypes}
		pkgs, err := packages.Load(cfg, path)
		if err != nil {
			return nil, err
		}
		if len(pkgs) != 1 {
			panic("expected go/packages.Load to return exactly one package")
		}
		tpkg := pkgs[0].Types
		l.typePkgs[path] = tpkg
		return tpkg, nil
	}

	// Loading a Gunk package for the first time.
	if err := l.addPkg(path); err != nil {
		return nil, err
	}
	if err := l.translatePkg(path); err != nil {
		return nil, err
	}
	if tpkg := l.typePkgs[path]; tpkg != nil {
		return tpkg, nil
	}

	// Not found.
	return nil, nil
}

// addProtoDep is called when a gunk file is known to require importing of a
// proto file, such as when using google.protobuf.Empty.
func (l *Loader) addProtoDep(protoPath string) {
	for _, dep := range l.pfile.Dependency {
		if dep == protoPath {
			return // already in there
		}
	}
	l.pfile.Dependency = append(l.pfile.Dependency, protoPath)
}

// loadProtoDeps loads all the proto dependencies added with addProtoDep.
//
// It does so with protoc, to leverage protoc's features such as locating the
// files, and the protoc parser to get a FileDescriptorProto out of the proto file
// content.
func (l *Loader) loadProtoDeps() error {
	missing := make(map[string]bool)
	for _, pfile := range l.allProto {
		for _, dep := range pfile.Dependency {
			if _, e := l.allProto[dep]; !e {
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
		l.allProto[*pfile.Name] = pfile
	}
	return nil
}
