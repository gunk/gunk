package generate

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"io/ioutil"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	desc "github.com/golang/protobuf/protoc-gen-go/descriptor"
	plugin "github.com/golang/protobuf/protoc-gen-go/plugin"
	"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger/options"
	"google.golang.org/genproto/googleapis/api/annotations"

	"github.com/gunk/gunk/config"
	"github.com/gunk/gunk/loader"
	"github.com/gunk/gunk/log"
	"github.com/gunk/gunk/reflectutil"
)

// Run generates the specified Gunk packages via protobuf generators, writing
// the output files in the same directories.
func Run(dir string, args ...string) error {
	g := &Generator{
		Loader: loader.Loader{
			Dir:   dir,
			Fset:  token.NewFileSet(),
			Types: true,
		},

		gunkPkgs: make(map[string]*loader.GunkPackage),
		allProto: make(map[string]*desc.FileDescriptorProto),

		protoLoader: &loader.ProtoLoader{},
	}

	// Check that protoc exists, if not download it.
	pkgs, err := g.Load(args...)
	if err != nil {
		return err
	}
	if len(pkgs) == 0 {
		return fmt.Errorf("no Gunk packages to generate")
	}
	if loader.PrintErrors(pkgs) > 0 {
		return fmt.Errorf("encountered package loading errors")
	}

	// Record the loaded packages in gunkPkgs.
	g.recordPkgs(pkgs...)

	// Cache of a package directory to its gunkconfig.
	pkgConfigs := map[string]*config.Config{}

	// Translate the packages from Gunk to Proto.
	for _, pkg := range pkgs {
		cfg, err := config.Load(pkg.Dir)
		if err != nil {
			return fmt.Errorf("unable to load gunkconfig: %v", err)
		}
		pkgConfigs[pkg.Dir] = cfg
		if err := g.translatePkg(pkg.PkgPath); err != nil {
			return err
		}
	}

	// Load any non-Gunk proto dependencies.
	if err := g.loadProtoDeps(); err != nil {
		return err
	}

	// Finally, run the code generators.
	for _, pkg := range pkgs {
		cfg := pkgConfigs[pkg.Dir]
		protocPath, err := CheckOrDownloadProtoc(cfg.ProtocPath, cfg.ProtocVersion)
		if err != nil {
			return err
		}

		if err := g.GeneratePkg(pkg.PkgPath, cfg.Generators, protocPath); err != nil {
			return err
		}
		log.Verbosef("%s", pkg.PkgPath)
	}
	return nil
}

// FileDescriptorSet will load a single Gunk package, and return the
// proto FileDescriptor set of the Gunk package.
//
// Currently, we only generate a FileDescriptorSet for one Gunk package.
func FileDescriptorSet(dir string, args ...string) (*desc.FileDescriptorSet, error) {
	// TODO: share code with Run; much of this function is identical.
	g := &Generator{
		Loader: loader.Loader{
			Dir:   dir,
			Fset:  token.NewFileSet(),
			Types: true,
		},

		gunkPkgs: make(map[string]*loader.GunkPackage),
		allProto: make(map[string]*desc.FileDescriptorProto),
	}

	pkgs, err := g.Load(args...)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("can only get filedescriptorset for a single Gunk package")
	}
	if loader.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("encountered package loading errors")
	}

	// Record the loaded packages in gunkPkgs.
	g.recordPkgs(pkgs...)

	// Translate the packages from Gunk to Proto.
	for _, pkg := range pkgs {
		if err := g.translatePkg(pkg.PkgPath); err != nil {
			return nil, err
		}
	}

	// Load any non-Gunk proto dependencies.
	if err := g.loadProtoDeps(); err != nil {
		return nil, err
	}

	// Generate the filedescriptorset for the Gunk package.
	req := g.requestForPkg(pkgs[0].PkgPath)
	fds := &desc.FileDescriptorSet{File: req.ProtoFile}
	return fds, nil
}

type Generator struct {
	loader.Loader

	curPkg      *loader.GunkPackage // current package being translated or generated
	curPos      token.Pos           // current position of the token being evaluated
	gfile       *ast.File
	pfile       *desc.FileDescriptorProto
	usedImports map[string]bool // imports being used for the current package

	// Maps from package import path to package information.
	gunkPkgs map[string]*loader.GunkPackage

	// imported proto files will be loaded using protoLoader
	// holds the absolute path passed to -I flag from protoc
	protoLoader *loader.ProtoLoader

	allProto map[string]*desc.FileDescriptorProto

	messageIndex int32
	serviceIndex int32
	enumIndex    int32
}

func (g *Generator) recordPkgs(pkgs ...*loader.GunkPackage) {
	for _, pkg := range pkgs {
		g.gunkPkgs[pkg.PkgPath] = pkg
		for _, ipkg := range pkg.Imports {
			g.recordPkgs(ipkg)
		}
	}
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
func (g *Generator) GeneratePkg(path string, gens []config.Generator, protocPath string) error {
	req := g.requestForPkg(path)
	// Run any other configured generators.
	for _, gen := range gens {
		if gen.IsProtoc() {
			if err := g.generateProtoc(*req, gen, protocPath); err != nil {
				return err
			}
		} else {
			if err := g.generatePlugin(*req, gen); err != nil {
				return err
			}
		}
	}
	return nil
}

func (g *Generator) generateProtoc(req plugin.CodeGeneratorRequest, gen config.Generator, protocCommandPath string) error {
	fds := &desc.FileDescriptorSet{}
	// Make a copy of the slice, as we may modify the elements within. See
	// the pf2 copying below.
	fds.File = make([]*desc.FileDescriptorProto, len(req.ProtoFile))
	copy(fds.File, req.ProtoFile)

	// The proto files we are asking protoc to generate. This should be
	// a formatted list of what is in req.FileToGenerate.
	protoFilenames := []string{}

	// Default location to output protoc generated files.
	protocOutputPath := ""
	for _, ftg := range req.GetFileToGenerate() {
		pkgPath, basename := filepath.Split(ftg)
		protoFilenames = append(protoFilenames, basename)

		// protoc writes the output files directly, unlike the
		// protoc-gen-* plugin generators.
		// As such, we need to give it the right basenames and output
		// directory, so that it writes the files in the right place.
		for i, pf := range fds.File {
			if pf.GetName() == ftg {
				// Make a copy, to not modify the files for
				// other generators too.
				pf2 := *pf
				pf2.Name = proto.String(basename)
				fds.File[i] = &pf2
			}
		}

		// Because we merge all .gunk files into one 'all.proto' file,
		// we can use that package path on disk as the default location
		// to output generated files.
		pkgPath = filepath.Clean(pkgPath)
		gpkg := g.gunkPkgs[pkgPath]
		protocOutputPath = gpkg.Dir
	}

	bs, err := proto.Marshal(fds)
	if err != nil {
		return err
	}

	// Build up the protoc command line arguments.
	args := []string{
		fmt.Sprintf("--%s_out=%s", gen.ProtocGen, gen.ParamStringWithOut(protocOutputPath)),
		"--descriptor_set_in=/dev/stdin",
	}

	args = append(args, protoFilenames...)

	cmd := log.ExecCommand(protocCommandPath, args...)
	cmd.Stdin = bytes.NewReader(bs)
	if _, err := cmd.Output(); err != nil {
		// TODO: For now, output the command name directly as
		// we actually use the /path/to/protoc when executing
		// the command, but this gives slightly uglier error
		// messages. Not sure what is best to do here, but
		// it should be consistent with running protoc-gen-*
		// errors (which currently don't use the /path/to/protoc-gen).
		return log.ExecError("protoc", err)
	}
	return nil
}

func (g *Generator) generatePlugin(req plugin.CodeGeneratorRequest, gen config.Generator) error {
	// Due to problems with some generators (grpc-gateway),
	// we need to ensure we either send a non-empty string or nil.
	if ps := gen.ParamString(); ps != "" {
		if gen.Out != "" {
			ps += fmt.Sprintf(",out=%s", gen.Out)
		}
		req.Parameter = proto.String(ps)
	}
	bs, err := proto.Marshal(&req)
	if err != nil {
		return err
	}
	cmd := log.ExecCommand(gen.Command)
	cmd.Stdin = bytes.NewReader(bs)
	out, err := cmd.Output()
	if err != nil {
		return log.ExecError(gen.Command, err)
	}
	var resp plugin.CodeGeneratorResponse
	if err := proto.Unmarshal(out, &resp); err != nil {
		return err
	}
	if rerr := resp.GetError(); rerr != "" {
		return fmt.Errorf("error from generator %s: %s", gen.Command, rerr)
	}
	for _, rf := range resp.File {
		// Turn the relative package file path to the absolute
		// on-disk file path.
		pkgPath, basename := filepath.Split(*rf.Name)
		pkgPath = filepath.Clean(pkgPath) // to remove trailing slashes
		gpkg := g.gunkPkgs[pkgPath]
		data := []byte(*rf.Content)
		dir := gen.OutPath(gpkg.Dir)
		outPath := filepath.Join(dir, basename)
		if err := ioutil.WriteFile(outPath, data, 0644); err != nil {
			return fmt.Errorf("unable to write to file %q: %v", outPath, err)
		}
	}
	return nil
}

func (g *Generator) requestForPkg(pkgPath string) *plugin.CodeGeneratorRequest {
	req := &plugin.CodeGeneratorRequest{}
	req.FileToGenerate = append(req.FileToGenerate, unifiedProtoFile(pkgPath))
	for _, pfile := range g.allProto {
		req.ProtoFile = append(req.ProtoFile, pfile)
	}

	// ProtoFile must be sorted in topological order, so that each file's
	// dependencies are satisfied by previous files. This is a requirement
	// of some generators.
	req.ProtoFile = topologicalSort(req.ProtoFile)
	return req
}

// topologicalSort sorts a number of protobuf descriptor files so that each
// file's dependencies can be satisfied by previous files in the list. In other
// words, it sorts the files incrementally by their dependencies.
//
// The algorithm isn't optimal, as it is a form of quadratic insertion sort with
// the help of a map. However, we won't be dealing with large numbers of proto
// files as each Gunk package is a single "all.proto" file, so this will likely
// be enough for a while. The advantage is that the implementation is very
// simple.
func topologicalSort(files []*desc.FileDescriptorProto) []*desc.FileDescriptorProto {
	previous := make(map[string]bool)
	result := make([]*desc.FileDescriptorProto, 0, len(files))

_addLoop:
	for len(result) < len(files) {
	_fileLoop:
		for _, pfile := range files {
			name := *pfile.Name
			if previous[name] {
				// Already part of the result.
				continue
			}
			for _, dep := range pfile.Dependency {
				if !previous[dep] {
					// Depends on files not in result yet.
					continue _fileLoop
				}
			}
			// Add this file.
			previous[name] = true
			result = append(result, pfile)
			continue _addLoop
		}
		// We didn't find a file we could add.
		panic("could not sort proto files by dependencies. dependency cycle?")
	}
	return result
}

// translatePkg translates all the gunk files in a gunk package to the
// proto language. All the files within the package, including all the
// files for its transitive dependencies, must already be loaded.
func (g *Generator) translatePkg(pkgPath string) error {
	gpkg := g.gunkPkgs[pkgPath]
	pfilename := unifiedProtoFile(gpkg.PkgPath)
	if _, ok := g.allProto[pfilename]; ok {
		// Already translated, e.g. as a dependency.
		return nil
	}

	g.curPkg = gpkg
	g.usedImports = make(map[string]bool)

	// Get file options for package
	fo, err := fileOptions(gpkg)
	if err != nil {
		return fmt.Errorf("unable to get file options: %v", err)
	}

	// Set the GoPackage file option to be the gunk package name.
	fo.GoPackage = proto.String(gpkg.Name)

	g.pfile = &desc.FileDescriptorProto{
		Syntax:  proto.String("proto3"),
		Name:    proto.String(pfilename),
		Package: proto.String(gpkg.ProtoName),
		Options: fo,
	}
	g.allProto[pfilename] = g.pfile

	g.messageIndex = 0
	g.serviceIndex = 0
	g.enumIndex = 0

	for i, fpath := range gpkg.GunkNames {
		if err := g.appendFile(fpath, gpkg.GunkSyntax[i]); err != nil {
			return fmt.Errorf("%s: %v", g.Loader.Fset.Position(g.curPos), err)
		}
	}

	var leftToTranslate []string

	for _, gfile := range gpkg.GunkSyntax {
		for _, imp := range gfile.Imports {
			if imp.Name != nil && imp.Name.Name == "_" {
				// An underscore import.
				continue
			}
			opath, _ := strconv.Unquote(imp.Path.Value)
			pkg := g.gunkPkgs[opath]
			if pkg == nil || len(pkg.GunkNames) == 0 {
				// Not a gunk package, so no joint proto file to
				// depend on.
				continue
			}
			if !g.usedImports[opath] {
				// Only include imports that are used.
				continue
			}
			pfile := unifiedProtoFile(opath)
			if _, ok := g.allProto[pfile]; !ok {
				leftToTranslate = append(leftToTranslate, opath)
			}
			g.pfile.Dependency = append(g.pfile.Dependency, pfile)
		}
	}

	// Do the recursive translatePkg calls at the end, since the generator
	// holds the state for the current package.
	for _, pkgPath := range leftToTranslate {
		if err := g.translatePkg(pkgPath); err != nil {
			return err
		}
	}
	return nil
}

// fileOptions will return the proto file options that have been set in the
// gunk package. These include "JavaPackage", "Deprecated", "PhpNamespace", etc.
func fileOptions(pkg *loader.GunkPackage) (*desc.FileOptions, error) {
	fo := &desc.FileOptions{}
	for _, f := range pkg.GunkSyntax {
		for _, tag := range pkg.GunkTags[f] {
			switch s := tag.Type.String(); s {
			case "github.com/gunk/opt/file.OptimizeFor":
				oValue := desc.FileOptions_OptimizeMode(protoEnumValue(tag.Value))
				fo.OptimizeFor = &oValue
			case "github.com/gunk/opt/file.Deprecated":
				fo.Deprecated = proto.Bool(constant.BoolVal(tag.Value))
			// Java package options.
			case "github.com/gunk/opt/file/java.Package":
				fo.JavaPackage = proto.String(constant.StringVal(tag.Value))
			case "github.com/gunk/opt/file/java.OuterClassname":
				fo.JavaOuterClassname = proto.String(constant.StringVal(tag.Value))
			case "github.com/gunk/opt/file/java.MultipleFiles":
				fo.JavaMultipleFiles = proto.Bool(constant.BoolVal(tag.Value))
			case "github.com/gunk/opt/file/java.StringCheckUtf8":
				fo.JavaStringCheckUtf8 = proto.Bool(constant.BoolVal(tag.Value))
			case "github.com/gunk/opt/file/java.GenericServices":
				fo.JavaGenericServices = proto.Bool(constant.BoolVal(tag.Value))
			// Swift package options.
			case "github.com/gunk/opt/file/swift.Prefix":
				fo.SwiftPrefix = proto.String(constant.StringVal(tag.Value))
			// Ruby package options.
			case "github.com/gunk/opt/file/ruby.Package":
				// TODO: This isn't currently in protoc-gen-go decriptor.pb.go
				// fo.RubyPackage = proto.String(constant.StringVal(tag.Value))
			// CSharp package options.
			case "github.com/gunk/opt/file/csharp.Namespace":
				fo.CsharpNamespace = proto.String(constant.StringVal(tag.Value))
			// ObjectiveC package options.
			case "github.com/gunk/opt/file/objc.ClassPrefix":
				fo.ObjcClassPrefix = proto.String(constant.StringVal(tag.Value))
			// PHP package options.
			case "github.com/gunk/opt/file/php.Namespace":
				fo.PhpNamespace = proto.String(constant.StringVal(tag.Value))
			case "github.com/gunk/opt/file/php.ClassPrefix":
				fo.PhpClassPrefix = proto.String(constant.StringVal(tag.Value))
			case "github.com/gunk/opt/file/php.MetadataNamespace":
				// TODO: This isn't currently in protoc-gen-go decriptor.pb.go
				// fo.PhpMetadataNamespace = proto.String(constant.StringVal(tag.Value))
			case "github.com/gunk/opt/file/php.GenericServices":
				fo.PhpGenericServices = proto.Bool(constant.BoolVal(tag.Value))
			case "github.com/gunk/opt/openapiv2.Swagger":
				o := &options.Swagger{}
				reflectutil.UnmarshalAST(o, tag.Expr)
				if err := proto.SetExtension(fo, options.E_Openapiv2Swagger, o); err != nil {
					return nil, fmt.Errorf("cannot set swagger extension: %s", err)
				}
			default:
				return nil, fmt.Errorf("gunk package option %q not supported", s)
			}
		}
	}
	// Set unset protocol buffer fields to their default values.
	proto.SetDefaults(fo)
	return fo, nil
}

// appendFile translates a single gunk file to protobuf, appending its contents
// to the package's proto file.
func (g *Generator) appendFile(fpath string, file *ast.File) error {
	if _, ok := g.allProto[fpath]; ok {
		// already translated
		return nil
	}
	g.gfile = file
	g.addDoc(file.Doc.Text(), packagePath)
	for _, decl := range file.Decls {
		g.curPos = decl.Pos()
		if err := g.translateDecl(decl); err != nil {
			return err
		}
	}
	return nil
}

// translateDecl translates a top-level declaration in a gunk file. It
// only acts on type declarations; struct types become proto messages,
// interfaces become services, and basic integer types become enums.
func (g *Generator) translateDecl(decl ast.Decl) error {
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
		g.curPos = ts.Pos()
		switch ts.Type.(type) {
		case *ast.StructType:
			msg, err := g.convertMessage(ts)
			if err != nil {
				return err
			}
			g.pfile.MessageType = append(g.pfile.MessageType, msg)
		case *ast.InterfaceType:
			srv, err := g.convertService(ts)
			if err != nil {
				return err
			}
			g.pfile.Service = append(g.pfile.Service, srv)
		case *ast.Ident:
			enum, err := g.convertEnum(ts)
			if err != nil {
				return err
			}
			// This can happen if the enum has no values.
			if enum != nil {
				g.pfile.EnumType = append(g.pfile.EnumType, enum)
			}
		default:
			return fmt.Errorf("invalid declaration type %T", ts.Type)
		}
	}
	return nil
}

func (g *Generator) addDoc(text string, path ...int32) {
	if text == "" {
		return
	}
	if g.pfile.SourceCodeInfo == nil {
		g.pfile.SourceCodeInfo = &desc.SourceCodeInfo{}
	}
	g.pfile.SourceCodeInfo.Location = append(g.pfile.SourceCodeInfo.Location,
		&desc.SourceCodeInfo_Location{
			Path:            path,
			LeadingComments: proto.String(" " + text),
		},
	)
}

func (g *Generator) messageOptions(tspec *ast.TypeSpec) (*desc.MessageOptions, error) {
	o := &desc.MessageOptions{}
	for _, tag := range g.curPkg.GunkTags[tspec] {
		switch s := tag.Type.String(); s {
		case "github.com/gunk/opt/message.MessageSetWireFormat":
			o.MessageSetWireFormat = proto.Bool(constant.BoolVal(tag.Value))
		case "github.com/gunk/opt/message.NoStandardDescriptorAccessor":
			o.NoStandardDescriptorAccessor = proto.Bool(constant.BoolVal(tag.Value))
		case "github.com/gunk/opt/message.Deprecated":
			o.Deprecated = proto.Bool(constant.BoolVal(tag.Value))
		default:
			return nil, fmt.Errorf("gunk message option %q not supported", s)
		}
	}
	proto.SetDefaults(o)
	return o, nil
}

func (g *Generator) fieldOptions(field *ast.Field) (*desc.FieldOptions, error) {
	o := &desc.FieldOptions{}
	for _, tag := range g.curPkg.GunkTags[field] {
		switch s := tag.Type.String(); s {
		case "github.com/gunk/opt/field.Packed":
			o.Packed = proto.Bool(constant.BoolVal(tag.Value))
		case "github.com/gunk/opt/field.Lazy":
			o.Lazy = proto.Bool(constant.BoolVal(tag.Value))
		case "github.com/gunk/opt/field.Deprecated":
			o.Deprecated = proto.Bool(constant.BoolVal(tag.Value))
		case "github.com/gunk/opt/field/cc.Type":
			oValue := desc.FieldOptions_CType(protoEnumValue(tag.Value))
			o.Ctype = &oValue
		case "github.com/gunk/opt/field/js.Type":
			oValue := desc.FieldOptions_JSType(protoEnumValue(tag.Value))
			o.Jstype = &oValue
		case "github.com/gunk/opt/openapiv2.Schema":
			for _, elt := range tag.Expr.(*ast.CompositeLit).Elts {
				kv := elt.(*ast.KeyValueExpr)
				switch kv.Key.(*ast.Ident).Name {
				case "JSONSchema":
					jsonSchema := &options.JSONSchema{}
					reflectutil.UnmarshalAST(jsonSchema, kv.Value)
					if err := proto.SetExtension(o, options.E_Openapiv2Field, jsonSchema); err != nil {
						return nil, err
					}
				}
			}
		default:
			return nil, fmt.Errorf("gunk field option %q not supported", s)
		}
	}
	proto.SetDefaults(o)
	return o, nil
}

func (g *Generator) convertMessage(tspec *ast.TypeSpec) (*desc.DescriptorProto, error) {
	g.addDoc(tspec.Doc.Text(), messagePath, g.messageIndex)

	msg := &desc.DescriptorProto{
		Name: proto.String(tspec.Name.Name),
	}
	messageOptions, err := g.messageOptions(tspec)
	if err != nil {
		return nil, fmt.Errorf("error getting message options: %v", err)
	}
	msg.Options = messageOptions
	stype := tspec.Type.(*ast.StructType)
	for i, field := range stype.Fields.List {
		if len(field.Names) != 1 {
			return nil, fmt.Errorf("need all fields to have one name")
		}
		fieldName := field.Names[0].Name
		g.addDoc(field.Doc.Text(), messagePath, g.messageIndex, messageFieldPath, int32(i))
		ftype := g.curPkg.TypesInfo.TypeOf(field.Type)
		g.curPos = field.Pos()

		var ptype desc.FieldDescriptorProto_Type
		var plabel desc.FieldDescriptorProto_Label
		var tname string
		var msgNestedType *desc.DescriptorProto

		// Check to see if the type is a map. Maps need to be made into a
		// repeated nested message containing key and value fields.
		if mtype, ok := ftype.(*types.Map); ok {
			ptype = desc.FieldDescriptorProto_TYPE_MESSAGE
			plabel = desc.FieldDescriptorProto_LABEL_REPEATED
			tname, msgNestedType = g.convertMap(tspec.Name.Name, fieldName, mtype)
			msg.NestedType = append(msg.NestedType, msgNestedType)
		} else {
			ptype, plabel, tname = g.convertType(ftype)
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
		fieldOptions, err := g.fieldOptions(field)
		if err != nil {
			return nil, fmt.Errorf("error getting field options: %v", err)
		}
		msg.Field = append(msg.Field, &desc.FieldDescriptorProto{
			Name:     proto.String(fieldName),
			Number:   num,
			TypeName: protoStringOrNil(tname),
			Type:     &ptype,
			Label:    &plabel,
			JsonName: jsonName(tag),
			Options:  fieldOptions,
		})
	}
	g.messageIndex++
	return msg, nil
}

func (g *Generator) serviceOptions(tspec *ast.TypeSpec) (*desc.ServiceOptions, error) {
	o := &desc.ServiceOptions{}
	for _, tag := range g.curPkg.GunkTags[tspec] {
		switch s := tag.Type.String(); s {
		case "github.com/gunk/opt/service.Deprecated":
			o.Deprecated = proto.Bool(constant.BoolVal(tag.Value))
		default:
			return nil, fmt.Errorf("gunk service option %q not supported", s)
		}
	}
	proto.SetDefaults(o)
	return o, nil
}

func (g *Generator) methodOptions(method *ast.Field) (*desc.MethodOptions, error) {
	o := &desc.MethodOptions{}
	for _, tag := range g.curPkg.GunkTags[method] {
		switch s := tag.Type.String(); s {
		case "github.com/gunk/opt/method.Deprecated":
			o.Deprecated = proto.Bool(constant.BoolVal(tag.Value))
		case "github.com/gunk/opt/method.IdempotencyLevel":
			oValue := desc.MethodOptions_IdempotencyLevel(protoEnumValue(tag.Value))
			o.IdempotencyLevel = &oValue
		case "github.com/gunk/opt/http.Match":
			// Capture the values required to use in annotations.HttpRule.
			// We need to evaluate the entire expression, and then we can
			// create an annotations.HttpRule.
			var path string
			var body string
			method := "GET"
			for _, elt := range tag.Expr.(*ast.CompositeLit).Elts {
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
					return nil, fmt.Errorf("unknown expression key %q", name)
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
				return nil, fmt.Errorf("unknown method type: %q", method)
			}
			if err := proto.SetExtension(o, annotations.E_Http, rule); err != nil {
				return nil, err
			}
			g.addProtoDep("google/api/annotations.proto")
		case "github.com/gunk/opt/openapiv2.Operation":
			op := &options.Operation{}
			reflectutil.UnmarshalAST(op, tag.Expr)
			if err := proto.SetExtension(o, options.E_Openapiv2Operation, op); err != nil {
				return nil, err
			}
			g.addProtoDep("protoc-gen-swagger/options/annotations.proto")
		default:
			return nil, fmt.Errorf("gunk method option %q not supported", s)
		}
	}
	proto.SetDefaults(o)
	return o, nil
}

func (g *Generator) convertService(tspec *ast.TypeSpec) (*desc.ServiceDescriptorProto, error) {
	srv := &desc.ServiceDescriptorProto{
		Name: proto.String(tspec.Name.Name),
	}
	serviceOptions, err := g.serviceOptions(tspec)
	if err != nil {
		return nil, fmt.Errorf("error getting service options: %v", err)
	}
	srv.Options = serviceOptions
	itype := tspec.Type.(*ast.InterfaceType)
	for i, method := range itype.Methods.List {
		if len(method.Names) != 1 {
			return nil, fmt.Errorf("need all methods to have one name")
		}
		g.addDoc(method.Doc.Text(), servicePath, g.serviceIndex, serviceMethodPath, int32(i))
		g.curPos = method.Pos()
		pmethod := &desc.MethodDescriptorProto{
			Name: proto.String(method.Names[0].Name),
		}
		methodOptions, err := g.methodOptions(method)
		if err != nil {
			return nil, fmt.Errorf("error getting method options: %v", err)
		}
		pmethod.Options = methodOptions
		sign := g.curPkg.TypesInfo.TypeOf(method.Type).(*types.Signature)
		pmethod.InputType, pmethod.ClientStreaming, err = g.convertParameter(sign.Params())
		if err != nil {
			return nil, err
		}
		pmethod.OutputType, pmethod.ServerStreaming, err = g.convertParameter(sign.Results())
		if err != nil {
			return nil, err
		}
		srv.Method = append(srv.Method, pmethod)
	}
	g.serviceIndex++
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
func (g *Generator) convertMap(parentName, fieldName string, mapTyp *types.Map) (string, *desc.DescriptorProto) {
	mapName := fieldName + "Entry"
	typeName := g.qualifiedTypeName(parentName+"."+mapName, nil)

	keyType, _, keyTypeName := g.convertType(mapTyp.Key())
	if keyType == 0 {
		return "", nil
	}
	elemType, _, elemTypeName := g.convertType(mapTyp.Elem())
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
				TypeName: protoStringOrNil(keyTypeName),
			},
			{
				Name:     proto.String("value"),
				Number:   proto.Int32(2),
				Label:    &fieldLabel,
				Type:     &elemType,
				TypeName: protoStringOrNil(elemTypeName),
			},
		},
	}
	return typeName, nestedType
}

func (g *Generator) convertParameter(tuple *types.Tuple) (*string, *bool, error) {
	switch tuple.Len() {
	case 0:
		g.addProtoDep("google/protobuf/empty.proto")
		return proto.String(".google.protobuf.Empty"), nil, nil
	case 1:
		// below
	default:
		return nil, nil, fmt.Errorf("multiple parameters are not supported")
	}
	param := tuple.At(0).Type()
	_, label, tname := g.convertType(param)
	if tname == "" {
		return nil, nil, fmt.Errorf("unsupported parameter type: %v", param)
	}
	if label == desc.FieldDescriptorProto_LABEL_REPEATED {
		return nil, nil, fmt.Errorf("parameter type should not be repeated")
	}

	isStream := proto.Bool(false)
	if _, ok := param.(*types.Chan); ok {
		isStream = proto.Bool(true)
	}

	return &tname, isStream, nil
}

func (g *Generator) enumOptions(tspec *ast.TypeSpec) (*desc.EnumOptions, error) {
	o := &desc.EnumOptions{}
	for _, tag := range g.curPkg.GunkTags[tspec] {
		switch s := tag.Type.String(); s {
		case "github.com/gunk/opt/enum.AllowAlias":
			o.AllowAlias = proto.Bool(constant.BoolVal(tag.Value))
		case "github.com/gunk/opt/enum.Deprecated":
			o.Deprecated = proto.Bool(constant.BoolVal(tag.Value))
		default:
			return nil, fmt.Errorf("gunk enum option %q not supported", s)
		}
	}
	proto.SetDefaults(o)
	return o, nil
}

func (g *Generator) enumValueOptions(vspec *ast.ValueSpec) (*desc.EnumValueOptions, error) {
	o := &desc.EnumValueOptions{}
	for _, tag := range g.curPkg.GunkTags[vspec] {
		switch s := tag.Type.String(); s {
		case "github.com/gunk/opt/enumvalues.Deprecated":
			o.Deprecated = proto.Bool(constant.BoolVal(tag.Value))
		default:
			return nil, fmt.Errorf("gunk enumvalue option %q not supported", s)
		}
	}
	proto.SetDefaults(o)
	return o, nil
}

func (g *Generator) convertEnum(tspec *ast.TypeSpec) (*desc.EnumDescriptorProto, error) {
	g.addDoc(tspec.Doc.Text(), enumPath, g.enumIndex)
	enum := &desc.EnumDescriptorProto{
		Name: proto.String(tspec.Name.Name),
	}
	enumOptions, err := g.enumOptions(tspec)
	if err != nil {
		return nil, fmt.Errorf("error getting enum options: %v", err)
	}
	enum.Options = enumOptions
	enumType := g.curPkg.TypesInfo.TypeOf(tspec.Name)
	for _, decl := range g.gfile.Decls {
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
			if g.curPkg.TypesInfo.TypeOf(name) != enumType {
				continue
			}
			g.curPos = vs.Pos()
			docText := vs.Doc.Text()
			switch {
			case docText == "":
				// The original comment only had gunk tags, and
				// no actual documentation for us to keep.
			case strings.HasPrefix(docText, name.Name):
				// SomeVal will be exported as SomeType_SomeVal
				docText = tspec.Name.Name + "_" + vs.Doc.Text()
				fallthrough
			default:
				g.addDoc(docText, enumPath, g.enumIndex,
					enumValuePath, int32(i))
			}

			val := g.curPkg.TypesInfo.Defs[name].(*types.Const).Val()
			ival, _ := constant.Int64Val(val)
			enumValueOptions, err := g.enumValueOptions(vs)
			if err != nil {
				return nil, fmt.Errorf("error getting enum value options: %v", err)
			}
			// To avoid duplicate prefix (protoc-gen-go),
			// we remove the enum type name if present
			// TODO: not covered by any test?
			prefix := *enum.Name + "_"
			name.Name = strings.Replace(name.Name, prefix, "", 1)

			enum.Value = append(enum.Value, &desc.EnumValueDescriptorProto{
				Name:    proto.String(name.Name),
				Number:  proto.Int32(int32(ival)),
				Options: enumValueOptions,
			})
		}
	}
	g.enumIndex++
	// If an enum doesn't have any values
	if len(enum.Value) == 0 {
		return nil, nil
	}
	return enum, nil
}

// qualifiedTypeName will format the type name for that package. If the
// package is nil, it will format the type for the current package that is
// being processed.
//
// Currently we format the type as ".<pkg_name>.<type_name>"
func (g *Generator) qualifiedTypeName(typeName string, pkg *types.Package) string {
	// If pkg is nil, we should format the type for the current package.
	if pkg == nil {
		return "." + g.curPkg.ProtoName + "." + typeName
	}
	gpkg := g.gunkPkgs[pkg.Path()]
	return "." + gpkg.ProtoName + "." + typeName
}

// convertType converts a Go field or parameter type to Protobuf, returning its
// type descriptor, a label such as "repeated", and a name, if the final type is
// an enum or a message.
func (g *Generator) convertType(typ types.Type) (desc.FieldDescriptorProto_Type, desc.FieldDescriptorProto_Label, string) {
	switch typ := typ.(type) {
	case *types.Chan:
		return g.convertType(typ.Elem())
	case *types.Basic:
		// Map Go types to proto types:
		// https://developers.google.com/protocol-buffers/docs/proto3#scalar
		switch typ.Kind() {
		case types.String:
			return desc.FieldDescriptorProto_TYPE_STRING, desc.FieldDescriptorProto_LABEL_OPTIONAL, ""
		case types.Int, types.Int32:
			return desc.FieldDescriptorProto_TYPE_INT32, desc.FieldDescriptorProto_LABEL_OPTIONAL, ""
		case types.Uint, types.Uint32:
			return desc.FieldDescriptorProto_TYPE_UINT32, desc.FieldDescriptorProto_LABEL_OPTIONAL, ""
		case types.Int64:
			return desc.FieldDescriptorProto_TYPE_INT64, desc.FieldDescriptorProto_LABEL_OPTIONAL, ""
		case types.Uint64:
			return desc.FieldDescriptorProto_TYPE_UINT64, desc.FieldDescriptorProto_LABEL_OPTIONAL, ""
		case types.Float32:
			return desc.FieldDescriptorProto_TYPE_FLOAT, desc.FieldDescriptorProto_LABEL_OPTIONAL, ""
		case types.Float64:
			return desc.FieldDescriptorProto_TYPE_DOUBLE, desc.FieldDescriptorProto_LABEL_OPTIONAL, ""
		case types.Bool:
			return desc.FieldDescriptorProto_TYPE_BOOL, desc.FieldDescriptorProto_LABEL_OPTIONAL, ""
		}
	case *types.Named:
		switch typ.String() {
		case "time.Time":
			g.addProtoDep("google/protobuf/timestamp.proto")
			return desc.FieldDescriptorProto_TYPE_MESSAGE, desc.FieldDescriptorProto_LABEL_OPTIONAL, ".google.protobuf.Timestamp"
		case "time.Duration":
			g.addProtoDep("google/protobuf/duration.proto")
			return desc.FieldDescriptorProto_TYPE_MESSAGE, desc.FieldDescriptorProto_LABEL_OPTIONAL, ".google.protobuf.Duration"
		}
		fullName := g.qualifiedTypeName(typ.Obj().Name(), typ.Obj().Pkg())
		g.usedImports[typ.Obj().Pkg().Path()] = true
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
		if eTyp, ok := typ.Elem().(*types.Basic); ok {
			if eTyp.Kind() == types.Byte {
				return desc.FieldDescriptorProto_TYPE_BYTES, desc.FieldDescriptorProto_LABEL_OPTIONAL, ""
			}
		}
		dtyp, _, name := g.convertType(typ.Elem())
		if dtyp == 0 {
			return 0, 0, ""
		}
		return dtyp, desc.FieldDescriptorProto_LABEL_REPEATED, name
	}
	return 0, 0, ""
}

// addProtoDep is called when a gunk file is known to require importing of a
// proto file, such as when using google.protobuf.Empty.
func (g *Generator) addProtoDep(protoPath string) {
	for _, dep := range g.pfile.Dependency {
		if dep == protoPath {
			return // already in there
		}
	}
	g.pfile.Dependency = append(g.pfile.Dependency, protoPath)
}

// loadProtoDeps loads all the missing proto dependencies added with
// addProtoDep.
func (g *Generator) loadProtoDeps() error {
	loaded := make(map[string]bool)
	var list []string
	for _, pfile := range g.allProto {
		for _, dep := range pfile.Dependency {
			if _, e := g.allProto[dep]; !e && !loaded[dep] {
				loaded[dep] = true
				list = append(list, dep)
			}
		}
	}

	files, err := g.protoLoader.LoadProto(list...)
	if err != nil {
		return err
	}

	for _, pfile := range files {
		g.allProto[*pfile.Name] = pfile
	}
	return nil
}
