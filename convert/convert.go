package convert

import (
	"fmt"
	"go/format"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/scanner"

	"github.com/emicklei/proto"
	"github.com/knq/snaker"
)

// pkgOpt stores a package option and its comments.
// eg: "go_package" or "java_package".
type pkgOpt struct {
	typeName string // go_package or java_package, etc.
	value    string
	comment  *proto.Comment
}

type builder struct {
	// The converted proto to gunk declarations. This only stores
	// the messages, enums and services. These get converted to as
	// they are found.
	translatedDeclarations []string

	// The package, option and imports from the proto file.
	// These are converted to gunk after we have converted
	// the rest of the proto declarations.
	pkg     *proto.Package
	pkgOpts []pkgOpt
	imports []*proto.Import
}

// Run converts a proto file to a gunk file, saving the file in the same folder
// as the proto file.
func Run(path string, overwrite bool) error {
	if filepath.Ext(path) != ".proto" {
		return fmt.Errorf("convert requires a .proto file")
	}
	reader, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("unable to read file %q: %v", path, err)
	}
	defer reader.Close()

	// Parse the proto file.
	parser := proto.NewParser(reader)
	d, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("unable to parse proto file %q: %v", path, err)
	}

	fileToWrite := strings.Replace(filepath.Base(path), ".proto", ".gunk", 1)
	fullpath := filepath.Join(filepath.Dir(path), fileToWrite)

	if _, err := os.Stat(fullpath); !os.IsNotExist(err) && !overwrite {
		return fmt.Errorf("path already exists %q, use --overwrite", fullpath)
	}

	// Start converting the proto declarations to gunk.
	b := builder{}
	for _, e := range d.Elements {
		if err := b.handleProtoType(e); err != nil {
			return fmt.Errorf("%v\n", err)
		}
	}

	// Convert the proto package and imports to gunk.
	translatedPkg := b.handlePackage()
	translatedImports := b.handleImports()

	// Add the converted package and imports, and then
	// add all the rest of the converted types. This will
	// keep the order that things were declared.
	w := &strings.Builder{}
	w.WriteString(translatedPkg)
	w.WriteString("\n\n")
	w.WriteString(translatedImports)
	w.WriteString("\n")
	for _, l := range b.translatedDeclarations {
		w.WriteString("\n")
		w.WriteString(l)
		w.WriteString("\n")
	}

	// TODO: We should run this through the Gunk generator to
	// make sure that it compiles?

	result := []byte(w.String())
	result, err = format.Source(result)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(fullpath, result, 0644); err != nil {
		return fmt.Errorf("unable to write to file %q: %v", fullpath, err)
	}

	return nil
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
	return fmt.Errorf("%d:%d: %v\n", pos.Line, pos.Column, fmt.Errorf(s, args...))
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
		// This is either an unrecognised type, or a custom type.
		return fieldType
	}
}

func (b *builder) handleProtoType(typ proto.Visitee) error {
	var err error
	switch typ.(type) {
	case *proto.Syntax:
		// Do nothing with syntax
	case *proto.Package:
		// This gets translated at the very end because it is used
		// in conjuction with the option "go_package" when writting
		// a Gunk package decleration.
		b.pkg = typ.(*proto.Package)
	case *proto.Import:
		// All imports need to be grouped and written out together. This
		// happens at the end.
		b.imports = append(b.imports, typ.(*proto.Import))
	case *proto.Message:
		err = b.handleMessage(typ.(*proto.Message))
	case *proto.Enum:
		err = b.handleEnum(typ.(*proto.Enum))
	case *proto.Service:
		err = b.handleService(typ.(*proto.Service))
	case *proto.Option:
		o := typ.(*proto.Option)
		lit := o.Constant
		value, err := b.handleLiteralString(lit)
		if err != nil {
			return b.formatError(lit.Position, "expected string literal in top level option: %v", err)
		}
		b.pkgOpts = append(b.pkgOpts, pkgOpt{
			typeName: o.Name,
			value:    value,
			comment:  o.Comment,
		})
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
	)

	switch field.(type) {
	case *proto.NormalField:
		ft := field.(*proto.NormalField)
		name = ft.Name
		typ = b.goType(ft.Type)
		sequence = ft.Sequence
		comment = ft.Comment
		repeated = ft.Repeated
	case *proto.MapField:
		ft := field.(*proto.MapField)
		name = ft.Field.Name
		sequence = ft.Field.Sequence
		comment = ft.Comment
		keyType := b.goType(ft.KeyType)
		fieldType := b.goType(ft.Field.Type)
		typ = fmt.Sprintf("map[%s]%s", keyType, fieldType)
	default:
		return fmt.Errorf("unhandled message field type %T", field)
	}

	if repeated {
		typ = "[]" + typ
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

// handleMessage will convert a proto message to Gunk.
func (b *builder) handleMessage(m *proto.Message) error {
	w := &strings.Builder{}
	b.format(w, 0, m.Comment, "type %s struct {\n", m.Name)
	for _, e := range m.Elements {
		switch e.(type) {
		case *proto.NormalField:
			f := e.(*proto.NormalField)
			if err := b.handleMessageField(w, f); err != nil {
				return b.formatError(f.Position, "error with message field: %v", err)
			}
		case *proto.Enum:
			// Handle the nested enum. This will create a new
			// top level enum as Gunk doesn't currently support
			// nested data structures.
			b.handleEnum(e.(*proto.Enum))
		case *proto.Comment:
			b.format(w, 1, e.(*proto.Comment), "")
		case *proto.MapField:
			mf := e.(*proto.MapField)
			if err := b.handleMessageField(w, mf); err != nil {
				return b.formatError(mf.Position, "error with message field: %v", err)
			}
		default:
			return b.formatError(m.Position, "unexpected type %T in message", e)
		}
	}
	b.format(w, 0, nil, "}")
	b.translatedDeclarations = append(b.translatedDeclarations, w.String())
	return nil
}

func (b *builder) handleEnum(e *proto.Enum) error {
	// TODO(vishen): Output as iota
	// Currently we output the entire constant decleration for each element, eg:
	// const (
	//     Element0 int = 0
	//     Element1 int = 1
	//     Element2 int = 2
	// )
	// It should be possible to output as an iota if the element starts at zero
	// and increments by 1.
	w := &strings.Builder{}
	b.format(w, 0, e.Comment, "type %s int\n", e.Name)
	b.format(w, 0, nil, "\nconst (\n")
	for _, c := range e.Elements {
		ef, ok := c.(*proto.EnumField)
		if !ok {
			return b.formatError(e.Position, "unexpected type %T in enum, expected enum field", c)
		}
		b.format(w, 1, ef.Comment, "%s %s = %d\n", ef.Name, e.Name, ef.Integer)
	}
	b.format(w, 0, nil, ")")
	b.translatedDeclarations = append(b.translatedDeclarations, w.String())
	return nil
}

func (b *builder) handleService(s *proto.Service) error {
	w := &strings.Builder{}
	b.format(w, 0, s.Comment, "type %s interface {\n", s.Name)
	for i, e := range s.Elements {
		r, ok := e.(*proto.RPC)
		if !ok {
			return b.formatError(s.Position, "unexpected type %T in service, expected rpc", e)
		}
		// Add a newline between each new function declaration on the interface.
		if i > 0 {
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
						url, err = b.handleLiteralString(*l.Literal)
						if err != nil {
							return b.formatError(opt.Position, "option for %q should be a string (url)", method)
						}
					}
				}

				// Check if we received a valid google http annotation. If
				// so we will convert it to gunk http match.
				if method != "" && url != "" {
					if comment != nil {
						b.format(w, 1, comment, "//\n")
						comment = nil
					}
					b.format(w, 1, nil, "// +gunk http.Match{\n")
					b.format(w, 1, nil, "//     Method: %q,\n", strings.ToUpper(method))
					b.format(w, 1, nil, "//     Path: %q,\n", url)
					if body != "" {
						b.format(w, 1, nil, "//     Body: %q,\n", body)
					}
					b.format(w, 1, nil, "// }\n")
				}
			default:
				// TODO(vishen): Should this emit an error? Or should we ignore
				// options that aren't handled by Gunk yet, or just log an error
				// for now?
				return b.formatError(r.Position, "%s option is not yet handled", n)
			}

			// If the request type is the known empty parameter we can convert
			// this to gunk as an empty function parameter.
			if r.RequestType != "google.protobuf.Empty" {
				b.format(w, 1, comment, "%s(%s) %s\n", r.Name, r.RequestType, r.ReturnsType)
			} else {
				b.format(w, 1, comment, "%s() %s\n", r.Name, r.ReturnsType)
			}
		}
	}
	b.format(w, 0, nil, "}")
	b.translatedDeclarations = append(b.translatedDeclarations, w.String())
	return nil
}

func (b *builder) handlePackage() string {
	w := &strings.Builder{}
	var opt pkgOpt
	for _, o := range b.pkgOpts {
		if o.typeName == "go_package" {
			opt = o
			break
		}
	}
	// TODO(vishen): Handle other package options when Gunk can handle other
	// options.
	p := b.pkg
	b.format(w, 0, p.Comment, "")
	b.format(w, 0, opt.comment, "")
	b.format(w, 0, nil, "package %s", p.Name)
	if opt.value != "" {
		b.format(w, 0, nil, " // proto %s", opt.value)
	}

	return w.String()
}

func (b *builder) handleImports() string {
	w := &strings.Builder{}
	b.format(w, 0, nil, `import (
	"github.com/gunk/opt"
	"github.com/gunk/opt/http"
`)

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
