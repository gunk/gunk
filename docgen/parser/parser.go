package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
	google_protobuf "github.com/golang/protobuf/protoc-gen-go/descriptor"
	"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway/httprule"
	"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger/options"
	method "google.golang.org/genproto/googleapis/api/annotations"
)

const (
	// Each element type (message, service, field ...) has an identifier
	// and combined with an index, they are used to locate them in the proto file.
	// [4, 0, 2, 1] means the second field (1) of the first message (0)
	// see https://github.com/golang/protobuf/blob/master/protoc-gen-go/descriptor/descriptor.pb.go#L2424
	messageFlag = 4
	fieldFlag   = 2
	enumFlag    = 5
)

// ParseFile parses a proto file.
func ParseFile(file *google_protobuf.FileDescriptorProto) (*File, error) {
	comments := parseComments(file.GetSourceCodeInfo())

	pkgName := file.GetPackage()
	messages := parseMessages(pkgName, comments, file.GetMessageType())
	services, err := parseServices(pkgName, messages, file.GetService())
	if err != nil {
		return nil, err
	}

	f := &File{
		Messages: messages,
		Services: services,
		Enums:    parseEnums(pkgName, comments, file.GetEnumType()),
	}

	if proto.HasExtension(file.GetOptions(), options.E_Openapiv2Swagger) {
		ext, err := proto.GetExtension(file.GetOptions(), options.E_Openapiv2Swagger)
		if err != nil {
			return nil, err
		}
		f.Swagger = ext.(*options.Swagger)
	}

	return f, nil
}

func parseEnums(pkgName string, comments map[string]*Comment, enums []*google_protobuf.EnumDescriptorProto) map[string]*Enum {
	res := map[string]*Enum{}
	for i, e := range enums {
		p := fmt.Sprintf("%d.%d", enumFlag, i)
		res[getQualifiedName(pkgName, e.GetName())] = &Enum{
			Name:    e.GetName(),
			Comment: nonNilComment(comments[p]),
			Values:  parseValues(p, comments, e.GetValue()),
		}
	}
	return res
}

func parseValues(path string, comments map[string]*Comment, values []*google_protobuf.EnumValueDescriptorProto) []*Value {
	res := make([]*Value, 0, len(values))
	for i, v := range values {
		if v.GetName() != "_" {
			res = append(res, &Value{
				Name:    v.GetName(),
				Comment: nonNilComment(comments[fmt.Sprintf("%s.%d.%d", path, fieldFlag, i)]),
			})
		}
	}
	return res
}

func parseMessages(pkgName string, comments map[string]*Comment, messages []*google_protobuf.DescriptorProto) map[string]*Message {
	res := map[string]*Message{}

	// convert each proto message to our Message representation
	for i, m := range messages {
		p := fmt.Sprintf("%d.%d", messageFlag, i)
		res[getQualifiedName(pkgName, m.GetName())] = &Message{
			Name:    m.GetName(),
			Comment: nonNilComment(comments[p]),
			Fields:  parseFields(p, comments, m.GetField()),
		}
	}

	// once we've defined all Messages, populate the NestedMessages
	// member of each Message
	for _, m := range res {
		populateNestedMessages(m, res)
	}
	return res
}

// populateNestedMessages populates the NestedMessages field of a Message.
//
// its first argument is the Message to check. an iterative breadth-first search is conducted
// to find any fields that are Messages (whose fields are then searched, and so on).
// these are then added to the original Message's NestedMessages field.
//
// the definitions of the Messages are taken from the second argument,
// which is a map that is expected to contain all Messages parsed from the package.
func populateNestedMessages(message *Message, messages map[string]*Message) {
	var nestedMessages []*Message

	stack := []*Message{message}
	seen := make(map[string]bool)

	// do an iterative breadth-first search on the message's fields to find all
	// fields that are messages (i.e. not primitives/scalar values), all of their
	// fields that are messages, and so on.
	//
	// keep track of messages that we've already seen to prevent duplicates
	for len(stack) > 0 {
		m := stack[0]
		stack = stack[1:]
		for _, f := range m.Fields {
			typ := f.Type
			name := typ.QualifiedName
			if !typ.IsMessage || seen[name] {
				continue
			}

			seen[name] = true

			nm, ok := messages[name]
			if !ok { // shouldn't happen
				panic(fmt.Sprintf("message %q not found", name))
			}

			stack = append(stack, nm)
			nestedMessages = append(nestedMessages, nm)
		}
	}

	message.NestedMessages = nestedMessages
}

func parseFields(path string, comments map[string]*Comment, fields []*google_protobuf.FieldDescriptorProto) []*Field {
	res := make([]*Field, len(fields))
	for i, f := range fields {
		res[i] = &Field{
			Name:     f.GetName(),
			Comment:  nonNilComment(comments[fmt.Sprintf("%s.%d.%d", path, fieldFlag, i)]),
			Type:     getType(f),
			JSONName: f.GetJsonName(),
		}
	}
	return res
}

func getType(f *google_protobuf.FieldDescriptorProto) *Type {
	typ := f.GetType()
	t := &Type{
		IsMessage:     typ == google_protobuf.FieldDescriptorProto_TYPE_MESSAGE,
		IsEnum:        typ == google_protobuf.FieldDescriptorProto_TYPE_ENUM,
		IsArray:       f.GetLabel() == google_protobuf.FieldDescriptorProto_LABEL_REPEATED,
		QualifiedName: f.GetTypeName(),
	}

	if t.IsArray {
		t.Name = "[]"
	}

	if tn := f.GetTypeName(); tn != "" {
		s := strings.Split(tn, ".")
		t.Name += s[len(s)-1]
		return t
	}

	t.Name += strings.ToLower(strings.TrimPrefix(f.GetType().String(), "TYPE_"))
	return t
}

func parseServices(pkgName string, messages map[string]*Message, services []*google_protobuf.ServiceDescriptorProto) (map[string]*Service, error) {
	res := map[string]*Service{}
	for _, s := range services {
		methods, err := parseMethods(pkgName, messages, s.GetMethod())
		if err != nil {
			return nil, err
		}
		res[getQualifiedName(pkgName, s.GetName())] = &Service{
			Name:    s.GetName(),
			Methods: methods,
		}
	}
	return res, nil
}

func parseMethods(pkgName string, messages map[string]*Message, methods []*google_protobuf.MethodDescriptorProto) (map[string]*Method, error) {
	res := map[string]*Method{}
	for _, m := range methods {
		extOp, err := proto.GetExtension(m.GetOptions(), options.E_Openapiv2Operation)
		if err != nil {
			if err == proto.ErrMissingExtension {
				continue
			}
			return nil, err
		}

		extHTTP, err := proto.GetExtension(m.GetOptions(), method.E_Http)
		if err != nil {
			if err == proto.ErrMissingExtension {
				continue
			}
			return nil, err
		}

		req, err := parseRequest(extHTTP.(*method.HttpRule), messages, m.GetInputType())
		if err != nil {
			return nil, err
		}

		res[getQualifiedName(pkgName, m.GetName())] = &Method{
			Name:      m.GetName(),
			Response:  messages[m.GetOutputType()],
			Operation: extOp.(*options.Operation),
			Request:   req,
		}
	}
	return res, nil
}

func parseRequest(rule *method.HttpRule, messages map[string]*Message, name string) (*Request, error) {
	var verb, uri string

	switch p := rule.GetPattern().(type) {
	case *method.HttpRule_Get:
		verb = http.MethodGet
		uri = p.Get
	case *method.HttpRule_Post:
		verb = http.MethodPost
		uri = p.Post
	case *method.HttpRule_Put:
		verb = http.MethodPut
		uri = p.Put
	case *method.HttpRule_Delete:
		verb = http.MethodDelete
		uri = p.Delete
	default:
		return nil, fmt.Errorf("%t not supported", p)
	}

	var body *Message
	var example string
	if rule.GetBody() != "" {
		// Body is defined in the gunk annotation with "*",
		// meaning that the operation uses the request
		// message has the request body.
		body = messages[name]

		p := genJSONExample(messages, name)
		b, err := p.MarshalJSON()
		if err != nil {
			return nil, err
		}
		example = string(b)
	}

	query, err := parseQuery(uri, messages[name])
	if err != nil {
		return nil, err
	}
	return &Request{
		Verb:    verb,
		URI:     uri,
		Body:    body,
		Query:   query,
		Example: example,
	}, nil
}

func parseQuery(uri string, message *Message) ([]*Field, error) {
	p, err := httprule.Parse(uri)
	if err != nil {
		return nil, err
	}

	tmpl := p.Compile()
	res := make([]*Field, len(tmpl.Fields))
	for i, tf := range tmpl.Fields {
		for _, mf := range message.Fields {
			if tf == mf.Name {
				res[i] = mf
			}
		}
	}

	return res, nil
}

func parseComments(sci *google_protobuf.SourceCodeInfo) map[string]*Comment {
	comments := map[string]*Comment{}
	for _, l := range sci.GetLocation() {
		k := make([]string, len(l.GetPath()))
		for i, p := range l.GetPath() {
			k[i] = strconv.Itoa(int(p))
		}
		comments[strings.Join(k, ".")] = &Comment{
			Leading:  strings.TrimPrefix(strings.ReplaceAll(l.GetLeadingComments(), "\n", ""), " "),
			Trailing: l.GetTrailingComments(),
			Detached: l.GetLeadingDetachedComments(),
		}
	}
	return comments
}

func nonNilComment(c *Comment) *Comment {
	if c == nil {
		return &Comment{}
	}
	return c
}

func getQualifiedName(pkgName, name string) string {
	return fmt.Sprintf(".%s.%s", pkgName, name)
}

func genJSONExample(messages map[string]*Message, path string) properties {
	m := messages[path]
	op := properties{}
	for _, f := range m.Fields {
		if f.Type.QualifiedName == "" || f.Type.IsEnum {
			op = append(op, keyVal{Key: f.JSONName, Value: f.Type.Name})
			continue
		}

		var v interface{} = genJSONExample(messages, f.Type.QualifiedName)
		if f.Type.IsArray {
			// Create an slice of type v and append v to it as an example.
			b := reflect.New(reflect.SliceOf(reflect.TypeOf(v)))
			v = reflect.Append(b.Elem(), reflect.ValueOf(v)).Interface()
		}
		op = append(op, keyVal{
			Key:   f.JSONName,
			Value: v,
		})
	}
	return op
}

type keyVal struct {
	Key   string
	Value interface{}
}

type properties []keyVal

// MarshalJSON returns an JSON encoding of properties
// in form of "key": "value"
func (p properties) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString("{\n")
	for i, kv := range p {
		if i != 0 {
			buf.WriteString(",\n")
		}
		key, err := json.Marshal(kv.Key)
		if err != nil {
			return nil, err
		}
		buf.WriteString("\t\t")
		buf.Write(key)
		buf.WriteString(": ")
		val, err := json.MarshalIndent(kv.Value, "\t\t", "\t")
		if err != nil {
			return nil, err
		}
		buf.Write(val)
	}

	buf.WriteString("\n\t}")
	return buf.Bytes(), nil
}
