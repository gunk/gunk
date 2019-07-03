package parser

import (
	"fmt"
	"net/http"
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

func parseMessages(pkgName string, comments map[string]*Comment, messages []*google_protobuf.DescriptorProto) map[string]*Message {
	res := map[string]*Message{}
	for i, m := range messages {
		p := fmt.Sprintf("%d.%d", messageFlag, i)
		res[getQualifiedName(pkgName, m.GetName())] = &Message{
			Name:           m.GetName(),
			Comment:        nonNilComment(comments[p]),
			Fields:         parseFields(p, comments, m.GetField()),
			NestedMessages: parseMessages(pkgName, comments, m.GetNestedType()),
		}
	}
	return res
}

func parseFields(path string, comments map[string]*Comment, fields []*google_protobuf.FieldDescriptorProto) []*Field {
	res := make([]*Field, len(fields))
	for i, f := range fields {

		res[i] = &Field{
			Name:    f.GetName(),
			Comment: nonNilComment(comments[fmt.Sprintf("%s.%d.%d", path, fieldFlag, i)]),
			Type:    getType(f),
		}
	}
	return res
}

func getType(f *google_protobuf.FieldDescriptorProto) string {
	if t := f.GetTypeName(); t != "" {
		s := strings.Split(t, ".")
		return s[len(s)-1]
	}
	return strings.ToLower(strings.TrimPrefix(f.GetType().String(), "TYPE_"))
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

		req, err := parseRequest(extHTTP.(*method.HttpRule), messages[m.GetInputType()])
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

func parseRequest(rule *method.HttpRule, message *Message) (*Request, error) {
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
	if rule.GetBody() != "" {
		// Body is defined in the gunk annotation with "*",
		// meaning that the operation uses the request
		// message has the request body.
		body = message
	}

	query, err := parseQuery(uri, message)
	if err != nil {
		return nil, err
	}
	return &Request{
		Verb:  verb,
		URI:   uri,
		Body:  body,
		Query: query,
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
