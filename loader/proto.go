package loader

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"text/scanner"
	"unicode"

	"github.com/emicklei/proto"
	"github.com/knq/snaker"

	"github.com/gunk/opt/openapiv2"
)

var urlVarRegexp = regexp.MustCompile(`\{(.*?)\}`)

// ConvertFromProto converts a single proto file read from r, writing the
// generated Gunk file to w. The output isn't canonically formatted, so it's up
// to the caller to use gunk/format.Source on the result if needed.
func ConvertFromProto(w io.Writer, r io.Reader, filename string, importPath string, protocPath string) error {
	// Parse the proto file.
	parser := proto.NewParser(r)
	d, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("unable to parse proto file %q: %v", filename, err)
	}

	// Start converting the proto declarations to gunk.
	b := builder{
		filename:      filename,
		importsUsed:   map[string]string{},
		existingDecls: map[string]bool{},
	}

	if importPath != "" {
		b.protoLoader = &ProtoLoader{
			Dir:        importPath,
			ProtocPath: protocPath,
		}
	}

	for _, e := range d.Elements {
		if err := b.handleProtoType(e); err != nil {
			return err
		}
	}

	// Validate that the package name is a a valid
	// Go package name.
	if err := b.validatePackageName(); err != nil {
		return err
	}

	// Convert the proto package and imports to gunk.
	translatedPkg, err := b.handlePackage()
	if err != nil {
		return err
	}
	translatedImports := b.handleImports()

	// Add the converted package and imports, and then
	// add all the rest of the converted types. This will
	// keep the order that things were declared.
	if _, err := fmt.Fprintf(w, "%s\n\n", translatedPkg); err != nil {
		return err
	}
	// If we have imports, output them.
	if translatedImports != "" {
		if _, err := fmt.Fprintf(w, "%s\n", translatedImports); err != nil {
			return err
		}
	}
	for _, l := range b.translatedDeclarations {
		if _, err := fmt.Fprintf(w, "\n%s\n", l); err != nil {
			return err
		}
	}
	return nil
}

type builder struct {
	// The current filename of file being converted
	filename string

	// The converted proto to gunk declarations. This only stores
	// the messages, enums and services. These get converted to as
	// they are found.
	translatedDeclarations []string

	// The package, option and imports from the proto file.
	// These are converted to gunk after we have converted
	// the rest of the proto declarations.
	pkg     *proto.Package
	pkgOpts []*proto.Option
	imports []*proto.Import

	// Imports that are required to ro generate a valid Gunk file.
	// Mostly these will be Gunk annotations. Import name will be
	// mapped to its possible named import.
	importsUsed map[string]string

	// imported proto files will be loaded using protoLoader
	// holds the absolute path passed to -I flag from protoc
	protoLoader *ProtoLoader

	// Holds existings declaration to avoid duplicate
	existingDecls map[string]bool
}

// format will write output to a string builder, adding in indentation
// where required. It will write the comment first if there is one,
// and then write the rest.
//
// TODO(vishen): this currently doesn't handle inline comments (and each proto
// declaration has an 'InlineComment', as well as a 'Comment' field), only the
// leading comment is currently passed through. This function should take
// the inline comment as well. However, this will require that the this function
// checks for a \n, and add the inline comment before that?
func (b *builder) format(w *strings.Builder, indent int, comments *proto.Comment, s string, args ...interface{}) {
	if comments != nil {
		for _, c := range comments.Lines {
			for i := 0; i < indent; i++ {
				fmt.Fprintf(w, "\t")
			}
			fmt.Fprintf(w, "//%s\n", c)
		}
	}
	// If we are just writing a comment bail out.
	if s == "" {
		return
	}
	for i := 0; i < indent; i++ {
		fmt.Fprintf(w, "\t")
	}
	fmt.Fprintf(w, s, args...)
}

// formatError will return an error formatted to include the current position in
// the file.
func (b *builder) formatError(pos scanner.Position, s string, args ...interface{}) error {
	return fmt.Errorf("%s:%d:%d: %v", b.filename, pos.Line, pos.Column, fmt.Errorf(s, args...))
}

// goType will turn a proto type to a known Go type. If the
// Go type isn't recognised, it is assumed to be a custom type.
func (b *builder) goType(fieldType string) string {
	// https://github.com/golang/protobuf/blob/1918e1ff6ffd2be7bed0553df8650672c3bfe80d/protoc-gen-go/generator/generator.go#L1601
	// https://developers.google.com/protocol-buffers/docs/proto3#scalar
	switch fieldType {
	case "bool":
		return "bool"
	case "string":
		return "string"
	case "bytes":
		return "[]byte"
	case "double":
		return "float64"
	case "float":
		return "float32"
	case "int32":
		return "int"
	case "sint32", "sfixed32":
		return "int32"
	case "int64", "sint64", "sfixed64":
		return "int64"
	case "uint32", "fixed32":
		return "uint32"
	case "uint64", "fixed64":
		return "uint64"
	default:
		// TODO: We return the proto package name unaltered. This
		// causes issues when a package name is imported or contains
		// "." or other invalid characters for a package name.
		// This is either an unrecognised type, or a custom type.
		return fieldType
	}
}

func (b *builder) handleProtoType(typ proto.Visitee) error {
	var err error
	switch typ := typ.(type) {
	case *proto.Syntax:
		// Do nothing with syntax
	case *proto.Package:
		// This gets translated at the very end because it is used
		// in conjuction with the option "go_package" when writting
		// a Gunk package decleration.
		b.pkg = typ
	case *proto.Import:
		if b.protoLoader != nil {
			files, err := b.protoLoader.LoadProto(typ.Filename)
			if err != nil {
				return err
			}
			named, source := "", ""
			for _, f := range files {
				if f != nil && f.GetName() == typ.Filename {
					named = strings.Replace(f.GetPackage(), ".", "_", -1)
					if f.GetOptions() != nil && f.GetOptions().GoPackage != nil {
						source = *f.GetOptions().GoPackage
					}
				}
			}
			if source == "" {
				return fmt.Errorf("imported file must contain go_package option %s", typ.Filename)
			}
			if named == "" {
				return fmt.Errorf("imported file must contain package name %s", typ.Filename)
			}
			// Import the go package
			b.importsUsed[source] = named
		} else {
			// All imports need to be grouped and written out together. This
			// happens at the end.
			b.imports = append(b.imports, typ)
		}
	case *proto.Message:
		err = b.handleMessage(typ)
	case *proto.Enum:
		err = b.handleEnum(typ)
	case *proto.Service:
		err = b.handleService(typ)
	case *proto.Option:
		b.pkgOpts = append(b.pkgOpts, typ)
	default:
		return fmt.Errorf("unhandled proto type %T", typ)
	}
	return err
}

// handleMessageField will convert a messages field to gunk.
func (b *builder) handleMessageField(w *strings.Builder, field proto.Visitee) error {
	var (
		name     string
		typ      string
		sequence int
		repeated bool
		comment  *proto.Comment
		options  []*proto.Option
	)

	switch field := field.(type) {
	case *proto.NormalField:
		name = field.Name
		typ = b.goType(field.Type)
		sequence = field.Sequence
		comment = field.Comment
		repeated = field.Repeated
		options = field.Options
	case *proto.MapField:
		name = field.Field.Name
		sequence = field.Field.Sequence
		comment = field.Comment
		keyType := b.goType(field.KeyType)
		fieldType := b.goType(field.Field.Type)
		typ = fmt.Sprintf("map[%s]%s", keyType, fieldType)
		options = field.Options
	default:
		return fmt.Errorf("unhandled message field type %T", field)
	}

	if comment != nil && strings.HasPrefix(strings.TrimSpace(comment.Message()), name) {
		comment.Lines[0] = strings.Replace(comment.Message(), name, snaker.ForceCamelIdentifier(name), 1)
	}
	if repeated {
		typ = "[]" + typ
	}

	for _, o := range options {
		val := o.Constant.Source
		var impt string
		var value string
		switch n := o.Name; n {
		case "packed":
			impt = "github.com/gunk/opt/message"
			value = b.genAnnotation("Packed", val)
		case "lazy":
			impt = "github.com/gunk/opt/message"
			value = b.genAnnotation("Lazy", val)
		case "deprecated":
			impt = "github.com/gunk/opt/message"
			value = b.genAnnotation("Deprecated", val)
		case "cc_type":
			impt = "github.com/gunk/opt/message/cc"
			value = b.genAnnotationString("Type", val)
		case "js_type":
			impt = "github.com/gunk/opt/message/js"
			value = b.genAnnotationString("Type", val)
		}

		pkg := b.addImportUsed(impt)
		b.format(w, 1, nil, fmt.Sprintf("// +gunk %s.%s\n", pkg, value))
	}

	// TODO(vishen): Is this correct to explicitly camelcase the variable name and
	// snakecase the json name???
	// If we do, gunk should probably have an option to set the variable name
	// in the proto to something else? That way we can use best practises for
	// each language???
	b.format(w, 1, comment, "%s %s", snaker.ForceCamelIdentifier(name), typ)
	b.format(w, 0, nil, " `pb:\"%d\" json:\"%s\"`\n", sequence, snaker.CamelToSnake(name))
	return nil
}

func (b *builder) containsImport(ref string) bool {
	for _, v := range b.importsUsed {
		if v == ref {
			return true
		}
	}
	return false
}

// handleMessage will convert a proto message to Gunk.
func (b *builder) handleMessage(m *proto.Message) error {
	w := &strings.Builder{}
	// check if there is no existing struct with the same name already
	if _, ok := b.existingDecls[m.Name]; ok {
		return b.formatError(m.Position, "%s redeclared in this block", m.Name)
	}
	b.existingDecls[m.Name] = true
	b.format(w, 0, m.Comment, "type %s struct {\n", m.Name)
	for _, e := range m.Elements {
		switch e := e.(type) {
		case *proto.NormalField:
			// Check if the type must be renamed in case
			// of declaration of nested message
			newType := fmt.Sprintf("%s_%s", m.Name, e.Type)
			if _, ok := b.existingDecls[newType]; ok {
				e.Type = newType
			}
			if strings.Contains(e.Type, ".") {
				ref := strings.Split(e.Type, ".")[0]
				if !b.containsImport(ref) {
					tmp := strings.Replace(e.Type, ".", "_", -1)
					// the type is neither found in import and existing decls
					if _, ok := b.existingDecls[tmp]; !ok {
						return b.formatError(e.Position, "%s is undefined", e.Type)
					}
					// Handle the use of nested field referenced outside
					// of its parent; Parent.Type is renamed to Parent_Type in a Go-Derived way
					e.Type = tmp
				}
			}
			if err := b.handleMessageField(w, e); err != nil {
				return b.formatError(e.Position, "error with message field: %v", err)
			}
		case *proto.Enum:
			// Handle the nested enum. This will create a new
			// top level enum as Gunk doesn't currently support
			// nested data structures.
			b.handleEnum(e)
		case *proto.Comment:
			b.format(w, 1, e, "")
		case *proto.MapField:
			if err := b.handleMessageField(w, e); err != nil {
				return b.formatError(e.Position, "error with message field: %v", err)
			}
		case *proto.Option:
			if err := b.handleOption(w, e); err != nil {
				return b.formatError(e.Position, "error with option field: %v", err)
			}
		case *proto.Message:
			// Handle the nested message. The struct is created at
			// the top level and renamed in the form Parent_Child
			e.Name = fmt.Sprintf("%s_%s", m.Name, e.Name)
			if err := b.handleMessage(e); err != nil {
				return b.formatError(e.Position, "error with nested message %v", err)
			}
		default:
			return b.formatError(m.Position, "unexpected type %T in message", e)
		}
	}
	b.format(w, 0, nil, "}")
	b.translatedDeclarations = append(b.translatedDeclarations, w.String())
	return nil
}

func (b *builder) handleOption(w *strings.Builder, opt *proto.Option) error {
	switch n := opt.Name; n {
	case "(grpc.gateway.protoc_gen_swagger.options.openapiv2_schema)":
		literal := opt.Constant
		schema, err := b.convertSchema(&literal)
		if err != nil {
			return err
		}
		pkg := b.addImportUsed("github.com/gunk/opt/openapiv2")
		b.format(w, 1, nil, "// +gunk %s.Schema{\n", pkg)
		if schema.JSONSchema != nil {
			b.format(w, 1, nil, "// JSONSchema: %s.JSONSchema{Title:%s, Description:%s}, \n", pkg, schema.JSONSchema.Title, schema.JSONSchema.Description)
		}
		if schema.Example != nil {
			b.format(w, 1, nil, "// Example: map[string]string{\"value\":`%s`}, \n", schema.Example.Value)
		}
		b.format(w, 1, nil, "// }\n")
	default:
		fmt.Fprintln(os.Stderr, fmt.Errorf("unhandled message option %q", opt.Name))
	}
	return nil
}

// handleEnum will output a proto enum as a Go const. It will output
// the enum using Go iota if each enum value is incrementing by 1
// (starting from 0). Otherwise we output each enum value as a straight
// conversion.
func (b *builder) handleEnum(e *proto.Enum) error {
	w := &strings.Builder{}
	b.format(w, 0, e.Comment, "type %s int\n", e.Name)
	b.format(w, 0, nil, "\nconst (\n")

	// Check to see if we can output the enum using an iota. This is
	// currently only possible if every enum value is an increment of 1
	// from the previous enum value.
	outputIota := true
	for i, c := range e.Elements {
		switch c := c.(type) {
		case *proto.EnumField:
			if i != c.Integer {
				outputIota = false
			}
		case *proto.Option:
			fmt.Fprintln(os.Stderr, b.formatError(c.Position, "unhandled enum option %q", c.Name))
		default:
			return b.formatError(e.Position, "unexpected type %T in enum, expected enum field", c)
		}
	}

	// Now we can output the enum as a const.
	for i, c := range e.Elements {
		ef, ok := c.(*proto.EnumField)
		if !ok {
			// We should have caught any errors when checking if we can output as
			// iota (above).
			// TODO(vishen): handle enum option
			continue
		}

		// Check if there is already an existing enum field with this name
		if ok := b.existingDecls[ef.Name]; ok {
			// prefix with the enum type name
			ef.Name = e.Name + "_" + ef.Name
		}
		b.existingDecls[ef.Name] = true

		for _, e := range ef.Elements {
			if o, ok := e.(*proto.Option); ok && o != nil {
				fmt.Fprintln(os.Stderr, b.formatError(o.Position, "unhandled enumvalue option %q", o.Name))
			}
		}

		// If we can't output as an iota.
		if !outputIota {
			b.format(w, 1, ef.Comment, "%s %s = %d\n", ef.Name, e.Name, ef.Integer)
			continue
		}

		// If we can output as an iota, output the first element as the
		// iota and output the rest as just the enum field name.
		if i == 0 {
			b.format(w, 1, ef.Comment, "%s %s = iota\n", ef.Name, e.Name)
		} else {
			b.format(w, 1, ef.Comment, "%s\n", ef.Name)
		}
	}
	b.format(w, 0, nil, ")")
	b.translatedDeclarations = append(b.translatedDeclarations, w.String())
	return nil
}

func (b *builder) handleService(s *proto.Service) error {
	w := &strings.Builder{}
	b.format(w, 0, s.Comment, "type %s interface {\n", s.Name)
	for i, e := range s.Elements {
		var r *proto.RPC
		switch e := e.(type) {
		case *proto.RPC:
			r = e
		case *proto.Option:
			fmt.Fprintln(os.Stderr, b.formatError(e.Position, "unhandled service option %q", e.Name))
			continue
		default:
			return b.formatError(s.Position, "unexpected type %T in service, expected rpc", e)
		}
		// Add a newline between each new function declaration on the interface, only
		// if there is comments or gunk annotations seperating them. We can assume that
		// anything in `Elements` will be a gunk annotation, otherwise an error is
		// returned below.
		if i > 0 && (r.Comment != nil || len(r.Elements) > 0) {
			b.format(w, 0, nil, "\n")
		}
		// The comment to translate. It is possible that when we write
		// the gunk annotations out we also write the comment above the
		// gunk annotation. If that happens we set the comment to nil
		// so it doesn't get written out when translating the field.
		comment := r.Comment
		for _, o := range r.Elements {
			opt, ok := o.(*proto.Option)
			if !ok {
				return b.formatError(r.Position, "unexpected type %T in service rpc, expected option", o)
			}
			switch n := opt.Name; n {
			case "(grpc.gateway.protoc_gen_swagger.options.openapiv2_operation)":
				literal := opt.Constant
				if len(literal.OrderedMap) == 0 {
					return b.formatError(opt.Position, "expected option to be a map")
				}
				var err error
				summary, description := "", ""
				tags := []string{}
				for _, l := range literal.OrderedMap {
					switch n := l.Name; n {
					case "tags":
						tag, err := b.handleLiteralString(*l.Literal)
						if err != nil {
							return b.formatError(opt.Position, "option for tags should be a string")
						}
						tags = append(tags, tag)
					case "summary":
						summary, err = b.handleLiteralString(*l.Literal)
						if err != nil {
							return b.formatError(opt.Position, "option for summary should be a string")
						}
					case "description":
						description, err = b.handleLiteralString(*l.Literal)
						if err != nil {
							return b.formatError(opt.Position, "option for description should be a string")
						}
					}
				}

				pkg := b.addImportUsed("github.com/gunk/opt/openapiv2")
				if comment != nil {
					b.format(w, 1, comment, "//\n")
					comment = nil
				}
				b.format(w, 1, nil, "// +gunk %s.Operation{\n", pkg)
				if len(tags) > 0 {
					b.format(w, 1, nil, "// Tags: []string{%q}, \n", strings.Join(tags, "\",\""))
				}
				if description != "" {
					b.format(w, 1, nil, "// Description: %q, \n", description)
				}
				if summary != "" {
					b.format(w, 1, nil, "// Summary: %q, \n", summary)
				}
				b.format(w, 1, nil, "// }\n")
			case "(google.api.http)":
				var err error
				method := ""
				url := ""
				body := ""
				literal := opt.Constant
				if len(literal.OrderedMap) == 0 {
					return b.formatError(opt.Position, "expected option to be a map")
				}
				for _, l := range literal.OrderedMap {
					switch n := l.Name; n {
					case "body":
						body, err = b.handleLiteralString(*l.Literal)
						if err != nil {
							return b.formatError(opt.Position, "option for body should be a string")
						}
					default:
						method = n
						tmp, err := b.handleLiteralString(*l.Literal)
						if err != nil {
							return b.formatError(opt.Position, "option for %q should be a string (url)", method)
						}
						url = urlVarRegexp.ReplaceAllStringFunc(tmp, func(id string) string {
							return "{" + snaker.ForceCamelIdentifier(id) + "}"
						})
					}
				}

				// Check if we received a valid google http annotation. If
				// so we will convert it to gunk http match.
				if method != "" && url != "" {
					pkg := b.addImportUsed("github.com/gunk/opt/http")
					if comment != nil {
						b.format(w, 1, comment, "//\n")
						comment = nil
					}
					b.format(w, 1, nil, "// +gunk %s.Match{\n", pkg)
					b.format(w, 1, nil, "// Method: %q,\n", strings.ToUpper(method))
					b.format(w, 1, nil, "// Path: %q,\n", url)
					if body != "" {
						b.format(w, 1, nil, "// Body: %q,\n", body)
					}
					b.format(w, 1, nil, "// }\n")
				}
			default:
				fmt.Fprintln(os.Stderr, b.formatError(opt.Position, "unhandled method option %q", n))
			}
		}
		// If the request type is the known empty parameter we can convert
		// this to gunk as an empty function parameter.
		requestType := r.RequestType
		returnsType := r.ReturnsType
		if requestType == "google.protobuf.Empty" {
			requestType = ""
		}
		if returnsType == "google.protobuf.Empty" {
			returnsType = ""
		}

		// If the request is a stream, add chan
		if r.StreamsRequest {
			requestType = "chan " + requestType
		}

		// If the response is a stream, add chan
		if r.StreamsReturns {
			returnsType = "chan " + returnsType
		}

		b.format(w, 1, comment, "%s(%s) %s\n", r.Name, requestType, returnsType)
	}
	b.format(w, 0, nil, "}")
	b.translatedDeclarations = append(b.translatedDeclarations, w.String())
	return nil
}

func (b *builder) genAnnotation(name, value string) string {
	return fmt.Sprintf("%s(%s)", name, value)
}

func (b *builder) genAnnotationString(name, value string) string {
	return fmt.Sprintf("%s(%q)", name, value)
}

func (b *builder) handlePackage() (string, error) {
	w := &strings.Builder{}
	var opt *proto.Option
	gunkAnnotations := []string{}
	for _, o := range b.pkgOpts {
		val := o.Constant.Source
		var impt string
		var value string
		switch n := o.Name; n {
		case "go_package":
			opt = o
			continue
		case "deprecated":
			impt = "github.com/gunk/opt/file"
			value = b.genAnnotation("Deprecated", val)
		case "optimize_for":
			impt = "github.com/gunk/opt/file"
			value = b.genAnnotation("OptimizeFor", val)
		case "java_package":
			impt = "github.com/gunk/opt/file/java"
			value = b.genAnnotationString("Package", val)
		case "java_outer_classname":
			impt = "github.com/gunk/opt/file/java"
			value = b.genAnnotationString("OuterClassname", val)
		case "java_multiple_files":
			impt = "github.com/gunk/opt/file/java"
			value = b.genAnnotation("MultipleFiles", val)
		case "java_string_check_utf8":
			impt = "github.com/gunk/opt/file/java"
			value = b.genAnnotation("StringCheckUtf8", val)
		case "java_generic_services":
			impt = "github.com/gunk/opt/file/java"
			value = b.genAnnotation("GenericServices", val)
		case "swift_prefix":
			impt = "github.com/gunk/opt/file/swift"
		case "csharp_namespace":
			impt = "github.com/gunk/opt/file/csharp"
			value = b.genAnnotationString("Namespace", val)
		case "objc_class_prefix":
			impt = "github.com/gunk/opt/file/objc"
			value = b.genAnnotationString("ClassPrefix", val)
		case "php_generic_services":
			impt = "github.com/gunk/opt/file/php"
			value = b.genAnnotation("GenericServices", val)
		case "cc_generic_services":
			impt = "github.com/gunk/opt/file/cc"
			value = b.genAnnotation("GenericServices", val)
		case "cc_enable_arenas":
			impt = "github.com/gunk/opt/file/cc"
			value = b.genAnnotation("EnableArenas", val)
		case "(grpc.gateway.protoc_gen_swagger.options.openapiv2_swagger)":
			impt = "github.com/gunk/opt/openapiv2"
			s, err := b.convertSwagger(o.Constant)
			if err != nil {
				return "", b.formatError(o.Position, "cannot convert swagger option: %s", err)
			}
			res := &strings.Builder{}
			b.format(res, 0, nil, "Swagger {\n")
			b.format(res, 0, nil, b.fromStructToAnnotation(*s))
			b.format(res, 0, nil, "// }")
			value = res.String()
		default:
			return "", b.formatError(o.Position, "%q is an unhandled proto file option", n)
		}

		pkg := b.addImportUsed(impt)
		gunkAnnotations = append(gunkAnnotations, fmt.Sprintf("%s.%s", pkg, value))
	}

	// Output the gunk annotations above the package comment. This
	// should be first lines in the file.
	for _, ga := range gunkAnnotations {
		b.format(w, 0, nil, fmt.Sprintf("// +gunk %s\n", ga))
	}

	p := b.pkg
	b.format(w, 0, p.Comment, "")
	if opt != nil {
		b.format(w, 0, opt.Comment, "")
	}
	b.format(w, 0, nil, "package %s", p.Name)
	if opt != nil && opt.Constant.Source != "" {
		b.format(w, 0, nil, " // proto %s", opt.Constant.Source)
	}

	return w.String(), nil
}

// indirectType is like reflect.Indirect, but it works on a reflect.Type.
func indirectType(t reflect.Type) reflect.Type {
	if t.Kind() != reflect.Ptr {
		return t
	}
	return t.Elem()
}

func (b *builder) fromStructToAnnotation(val interface{}) string {
	w := &strings.Builder{}

	v := reflect.ValueOf(val)
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		name := t.Field(i).Name
		value := v.FieldByName(name)
		if !isNilOrEmpty(value.Interface()) {
			switch value.Kind() {
			case reflect.Invalid,
				reflect.Uintptr,
				reflect.Complex64,
				reflect.Complex128,
				reflect.Array,
				reflect.Chan,
				reflect.Func,
				reflect.Interface,
				reflect.Struct,
				reflect.UnsafePointer:
				// Ignore
			case reflect.Map:
				mapKey := value.Type().Key()
				mapValue := indirectType(value.Type().Elem())
				b.format(w, 0, nil, "// %s: map[%s]%s{\n", name, mapKey, mapValue)
				for _, key := range value.MapKeys() {
					c := value.MapIndex(key)
					switch c.Type().Kind() {
					case reflect.Ptr:
						t := reflect.Indirect(c).Interface()
						b.format(w, 0, nil, "// %s: %T{\n", key, t)
						b.format(w, 0, nil, b.fromStructToAnnotation(t))
						b.format(w, 0, nil, "// },\n")
					default:
						b.format(w, 0, nil, "// %s: %s,\n", key, c)
					}
				}
				b.format(w, 0, nil, "// },\n")
			case reflect.Ptr:
				t := reflect.Indirect(value).Interface()
				b.format(w, 0, nil, "// %s: %s{\n", name, reflect.TypeOf(t))
				w.WriteString(b.fromStructToAnnotation(t))
				b.format(w, 0, nil, "// },\n")
			case reflect.Slice:
				b.format(w, 0, nil, "// %s: %s{\n", name, reflect.TypeOf(value.Interface()))
				for i := 0; i < value.Len(); i++ {
					val := value.Index(i).Interface()
					switch reflect.TypeOf(val).Kind() {
					case reflect.Ptr:
						// TODO (ced)
					default:
						if pkgPath := reflect.TypeOf(val).PkgPath(); pkgPath == "" {
							b.format(w, 0, nil, "// %v,\n", val)
						} else {
							pkg := strings.Split(pkgPath, "/")
							b.format(w, 0, nil, "// %s.%v,\n", pkg[len(pkg)-1], val)
						}
					}
				}
				b.format(w, 0, nil, "// },\n")
			default:
				if pkgPath := value.Type().PkgPath(); pkgPath == "" {
					b.format(w, 0, nil, "// %s: %v,\n", name, value)
				} else {
					pkg := strings.Split(pkgPath, "/")
					b.format(w, 0, nil, "// %s: %s.%v,\n", name, pkg[len(pkg)-1], value)
				}
			}
		}
	}
	return w.String()
}

func isNilOrEmpty(x interface{}) bool {
	return reflect.DeepEqual(x, reflect.Zero(reflect.TypeOf(x)).Interface())
}

func (*builder) setValue(name string, dest interface{}, value string) error {
	tmp := reflect.Indirect(reflect.ValueOf(dest)).FieldByName(name)
	if !tmp.IsValid() {
		return fmt.Errorf("%s was not found in %T", name, dest)
	}
	if !tmp.CanAddr() {
		return fmt.Errorf("%s from %T cannot be addressed", name, dest)
	}
	var v interface{}
	var err error
	switch k := reflect.TypeOf(tmp.Interface()).Kind(); k {
	case reflect.String:
		v = value
	case reflect.Float64:
		v, err = strconv.ParseFloat(value, 64)
	case reflect.Bool:
		v, err = strconv.ParseBool(value)
	case reflect.Uint64:
		v, err = strconv.ParseUint(value, 10, 64)
	default:
		return fmt.Errorf("%s not supported", k.String())
	}
	if err != nil {
		return err
	}
	tmp.Set(reflect.ValueOf(v))
	return nil
}

func (b *builder) convertSwagger(lit proto.Literal) (*openapiv2.Swagger, error) {
	if len(lit.OrderedMap) == 0 {
		return nil, fmt.Errorf("expected swagger option to be a map")
	}
	swagger := &openapiv2.Swagger{
		Responses: map[string]*openapiv2.Response{},
	}
	for _, v := range lit.OrderedMap {
		switch v.Name {
		case "responses":
			key, resp, err := b.convertResponse(v)
			if err != nil {
				return nil, err
			}
			swagger.Responses[key] = resp
		case "swagger", "base_path", "host":
			b.setValue(snaker.ForceCamelIdentifier(v.Name), swagger, v.SourceRepresentation())
		case "security_definitions":
			var err error
			swagger.SecurityDefinitions, err = b.convertSecurityDefinitions(v)
			if err != nil {
				return nil, err
			}
		case "schemes":
			s := openapiv2.SwaggerScheme_value[v.SourceRepresentation()]
			swagger.Schemes = append(swagger.Schemes, openapiv2.SwaggerScheme(s))
		case "consumes":
			swagger.Consumes = append(swagger.Consumes, v.SourceRepresentation())
		case "produces":
			swagger.Produces = append(swagger.Produces, v.SourceRepresentation())
		case "external_docs":
			var err error
			swagger.ExternalDocs, err = b.convertExternalDocs(v)
			if err != nil {
				return nil, err
			}
		case "info":
			var err error
			swagger.Info, err = b.convertInfo(v)
			if err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unexpected swagger option element %s", v.Name)
		}
	}
	return swagger, nil
}

func (b *builder) convertSecurityDefinitions(lit *proto.NamedLiteral) (*openapiv2.SecurityDefinitions, error) {
	if len(lit.OrderedMap) == 0 {
		return nil, fmt.Errorf("expected security_definitions option to be a map")
	}
	securityDefinitions := &openapiv2.SecurityDefinitions{
		Security: map[string]*openapiv2.SecurityScheme{},
	}
	for _, valueInfo := range lit.OrderedMap {
		switch valueInfo.Name {
		case "security":
			if len(lit.OrderedMap) == 0 {
				return nil, fmt.Errorf("expected security option to be a map")
			}
			var key string
			var value *openapiv2.SecurityScheme
			for _, secVal := range valueInfo.OrderedMap {
				switch secVal.Name {
				case "key":
					key = secVal.SourceRepresentation()
				case "value":
					value = &openapiv2.SecurityScheme{}
					if len(secVal.OrderedMap) == 0 {
						return nil, fmt.Errorf("expected security value to be a map")
					}
					for _, val := range secVal.OrderedMap {
						switch val.Name {
						case "type":
							t := openapiv2.Type_value[val.SourceRepresentation()]
							value.Type = openapiv2.Type(t)
						case "description", "name", "authorization_url", "token_url":
							if err := b.setValue(snaker.ForceCamelIdentifier(val.Name), value, val.SourceRepresentation()); err != nil {
								return nil, err
							}
						case "in":
							in := openapiv2.In_value[val.SourceRepresentation()]
							value.In = openapiv2.In(in)
						case "flow":
							f := openapiv2.Flow_value[val.SourceRepresentation()]
							value.Flow = openapiv2.Flow(f)
						case "scopes":
							scope, err := b.convertScopes(val)
							if err != nil {
								return nil, err
							}
							value.Scopes = &openapiv2.Scopes{
								Scope: scope,
							}
						default:
							return nil, fmt.Errorf("unexpected security value element")
						}
					}
				default:
					return nil, fmt.Errorf("unexpected security option element")
				}
			}
			securityDefinitions.Security[key] = value
		default:
			return nil, fmt.Errorf("unexpected security_definitions option element")
		}
	}
	return securityDefinitions, nil
}

func (b *builder) convertInfo(lit *proto.NamedLiteral) (*openapiv2.Info, error) {
	if len(lit.OrderedMap) == 0 {
		return nil, fmt.Errorf("expected info option to be a map")
	}
	info := &openapiv2.Info{}
	for _, valueInfo := range lit.OrderedMap {
		switch valueInfo.Name {
		case "title", "description", "terms_of_service", "version":
			if err := b.setValue(snaker.ForceCamelIdentifier(valueInfo.Name), info, valueInfo.SourceRepresentation()); err != nil {
				return nil, err
			}
		case "contact":
			contact, err := b.convertContact(valueInfo)
			if err != nil {
				return nil, err
			}
			info.Contact = contact
		case "license":
			license, err := b.convertLicense(valueInfo)
			if err != nil {
				return nil, err
			}
			info.License = license
		default:
			return nil, fmt.Errorf("unexpected info option element")
		}
	}
	return info, nil
}

func (b *builder) convertContact(lit *proto.NamedLiteral) (*openapiv2.Contact, error) {
	if len(lit.OrderedMap) == 0 {
		return nil, fmt.Errorf("expected contact option to be a map")
	}
	contact := &openapiv2.Contact{}
	for _, v := range lit.OrderedMap {
		switch v.Name {
		case "name", "url", "email":
			if err := b.setValue(snaker.ForceCamelIdentifier(v.Name), contact, v.SourceRepresentation()); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unexpected contact option element %s", v.Name)
		}
	}
	return contact, nil
}

func (b *builder) convertLicense(lit *proto.NamedLiteral) (*openapiv2.License, error) {
	if len(lit.OrderedMap) == 0 {
		return nil, fmt.Errorf("expected license option to be a map")
	}
	license := &openapiv2.License{}
	for _, v := range lit.OrderedMap {
		switch v.Name {
		case "name", "url":
			if err := b.setValue(snaker.ForceCamelIdentifier(v.Name), license, v.SourceRepresentation()); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unexpected license option element %s", v.Name)
		}
	}
	return license, nil
}

func (b *builder) convertScopes(scopes *proto.NamedLiteral) (map[string]string, error) {
	if len(scopes.OrderedMap) == 0 {
		return nil, fmt.Errorf("expected scopes to be a map")
	}
	res := map[string]string{}
	key, value := "", ""
	for _, scope := range scopes.OrderedMap {
		if len(scope.OrderedMap) == 0 {
			return nil, fmt.Errorf("expected scope to be a map")
		}
		for _, v := range scope.OrderedMap {
			switch v.Name {
			case "key":
				key = v.SourceRepresentation()
			case "value":
				value = v.SourceRepresentation()
			}
		}
		res[key] = value
	}
	return res, nil
}

func (b *builder) convertResponse(response *proto.NamedLiteral) (string, *openapiv2.Response, error) {
	key := ""
	res := &openapiv2.Response{}
	if tmp, ok := response.Literal.OrderedMap.Get("key"); ok {
		key = tmp.SourceRepresentation()
	} else {
		return "", nil, fmt.Errorf("could not find key in response")
	}
	if tmp, ok := response.Literal.OrderedMap.Get("value"); ok {
		if desc, ok := tmp.OrderedMap.Get("description"); ok {
			res.Description = desc.SourceRepresentation()
		}
		if schema, ok := tmp.OrderedMap.Get("schema"); ok {
			var err error
			res.Schema, err = b.convertSchema(schema)
			if err != nil {
				return "", nil, err
			}
		}
	} else {
		return "", nil, fmt.Errorf("could not find value in response")
	}
	return key, res, nil
}

func (b *builder) convertExternalDocs(docs *proto.NamedLiteral) (*openapiv2.ExternalDocumentation, error) {
	if len(docs.OrderedMap) == 0 {
		return nil, fmt.Errorf("expected external_docs option to be a map")
	}
	res := &openapiv2.ExternalDocumentation{}
	for _, valueInfo := range docs.OrderedMap {
		switch valueInfo.Name {
		case "description", "url":
			if err := b.setValue(snaker.ForceCamelIdentifier(valueInfo.Name), res, valueInfo.SourceRepresentation()); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unexpected external_docs option element")
		}
	}
	return res, nil
}

func (b *builder) convertSchema(literal *proto.Literal) (*openapiv2.Schema, error) {
	schema := &openapiv2.Schema{}
	if len(literal.OrderedMap) == 0 {
		return nil, fmt.Errorf("expected option to be a map")
	}
	for _, l := range literal.OrderedMap {
		switch n := l.Name; n {
		case "discriminator":
			schema.Discriminator = l.SourceRepresentation()
		case "read_only":
			var err error
			schema.ReadOnly, err = strconv.ParseBool(l.SourceRepresentation())
			if err != nil {
				return nil, err
			}
		case "external_docs":
			var err error
			schema.ExternalDocs, err = b.convertExternalDocs(l)
			if err != nil {
				return nil, err
			}
		case "example":
			if len(l.Literal.Map) == 0 {
				return nil, fmt.Errorf("expected option to be a map")
			}
			value := l.Literal.Map["value"]
			val, err := b.handleLiteralString(*value)
			if err != nil {
				return nil, fmt.Errorf("error with litteral string %q", err)
			}
			example := &openapiv2.Any{
				Value: []byte(val),
			}
			schema.Example = example
		case "json_schema":
			if len(l.Literal.Map) == 0 {
				return nil, fmt.Errorf("exepected option to be a map")
			}
			jsonSchema := &openapiv2.JSONSchema{}
			for k, v := range l.Literal.Map {
				switch k {
				case "ref",
					"title",
					"description",
					"default",
					"pattern",
					"multiple_of",
					"maximum",
					"minimum",
					"exclusive_maximum",
					"exclusive_minimum",
					"unique_items",
					"max_length",
					"min_length",
					"max_items",
					"min_items",
					"max_properties",
					"min_properties":
					if err := b.setValue(snaker.ForceCamelIdentifier(k), jsonSchema, v.SourceRepresentation()); err != nil {
						return nil, err
					}
				case "required":
					for _, r := range v.Array {
						jsonSchema.Required = append(jsonSchema.Required, r.SourceRepresentation())
					}
				case "array":
					for _, r := range v.Array {
						jsonSchema.Array = append(jsonSchema.Array, r.SourceRepresentation())
					}
				case "type":
					t := openapiv2.JSONSchemaSimpleTypes_value[v.SourceRepresentation()]
					jsonSchema.Type = append(jsonSchema.Type, openapiv2.JSONSchemaSimpleTypes(t))
				}
			}
			schema.JSONSchema = jsonSchema
		}
	}
	return schema, nil
}

// addImportUsed will record the import so that we can
// output the import string when generating the output file.
// We return the package name to use. Also check to see if
// we need to use a named import for this import.
func (b *builder) addImportUsed(i string) string {
	r := strings.NewReplacer("github.com/gunk/opt", "", "/", "")
	namedImport := r.Replace(i)
	pkg := filepath.Base(i)

	// Determine if there is a package with the same name
	// as this one, if there is give this current one a
	// named import with "/" replaced, eg:
	// "github.com/gunk/opt/file/java" becomes "filejava"
	useNamedImport := false
	for imp := range b.importsUsed {
		if imp != i && pkg == filepath.Base(imp) {
			useNamedImport = true
			break
		}
	}

	// If the import requires a named import, record it and
	// return the named import as the new package name.
	if useNamedImport {
		b.importsUsed[i] = namedImport
		return namedImport
	}

	// If this is the first time that package name has been
	// seen then we can keep the import as is.
	b.importsUsed[i] = ""
	return pkg
}

func (b *builder) handleImports() string {
	if len(b.importsUsed) == 0 && len(b.imports) == 0 {
		return ""
	}

	w := &strings.Builder{}
	b.format(w, 0, nil, "import (")

	// Imports that have been used during convert.
	for i, named := range b.importsUsed {
		b.format(w, 0, nil, "\n")
		if named != "" {
			b.format(w, 1, nil, fmt.Sprintf("%s %q", named, i))
		} else {
			b.format(w, 1, nil, fmt.Sprintf("%q", i))
		}
	}

	// Add any proto imports as comments.
	for _, i := range b.imports {
		b.format(w, 0, nil, "\n")
		b.format(w, 1, nil, "// %q", i.Filename)
	}
	b.format(w, 0, nil, "\n)")
	return w.String()
}

func (b *builder) handleLiteralString(lit proto.Literal) (string, error) {
	if !lit.IsString {
		return "", fmt.Errorf("literal was expected to be a string")
	}
	return lit.Source, nil
}

func (b *builder) validatePackageName() error {
	pkgName := b.pkg.Name
	for _, c := range pkgName {
		// A package name is a normal identifier in Go, but it
		// cannot be the blank identifier.
		// https://golang.org/ref/spec#Package_clause
		if unicode.IsLetter(c) || c == '_' || unicode.IsDigit(c) {
			continue
		}
		return fmt.Errorf("invalid character %q in package name %q", c, pkgName)
	}
	return nil
}
