package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	"github.com/gunk/gunk/httprule"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
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

type OnlyExternalOpt int

const (
	OnlyExternalDisabled OnlyExternalOpt = iota
	OnlyExternalEnabled
)

// ParseFile parses a proto file.
func ParseFile(file *FileDescWrapper, onlyExternal OnlyExternalOpt) (*File, error) {
	pkgName := file.GetPackage()
	messages, enums, err := nestedDescriptorMessages(*file.FileDescriptorProto, file.DependencyMap)
	if err != nil {
		return nil, err
	}
	services, err := parseServices(pkgName, messages, file.GetService(), onlyExternal)
	if err != nil {
		return nil, err
	}
	f := &File{
		Services: services,
		Enums:    enums,
	}
	if proto.HasExtension(file.GetOptions(), options.E_Openapiv2Swagger) {
		f.Swagger = proto.GetExtension(file.GetOptions(), options.E_Openapiv2Swagger).(*options.Swagger)
	}
	return f, nil
}

// GenerateDependencyMap recursively loops through dependencies and creates a map for their names and FileDescriptorProto
func GenerateDependencyMap(source *descriptorpb.FileDescriptorProto, protos []*descriptorpb.FileDescriptorProto) map[string]*descriptorpb.FileDescriptorProto {
	// get dependencies recursively
	returnedDependencies := make(map[string]*descriptorpb.FileDescriptorProto)
	for _, d := range source.GetDependency() {
		for _, p := range protos {
			if p.GetName() == d {
				returnedDependencies[d] = p
				dependencies := GenerateDependencyMap(p, protos)
				for name, dep := range dependencies {
					returnedDependencies[name] = dep
				}
				break
			}
		}
	}
	return returnedDependencies
}

// nestedDescriptorMessages generates a map of messages with a few steps:
// it loops through the file's dependency list and checks if it is needed
// if it is need it recursively runs with the new file to
// it merges the messages generate on each recursion
func nestedDescriptorMessages(
	file descriptorpb.FileDescriptorProto,
	genFiles map[string]*descriptorpb.FileDescriptorProto,
) (
	map[string]*Message,
	map[string]*Enum,
	error,
) {
	// (TODO kofo) this could be improved
	messages := make(map[string]*Message)
	enums := make(map[string]*Enum)
	dependencies := file.GetDependency()
	pkgName := file.GetPackage()
	comments := parseComments(file.GetSourceCodeInfo())
	for _, dep := range dependencies {
		if _, ok := genFiles[dep]; !ok || !isDepNeeded(file, genFiles[dep]) {
			continue
		}
		// get all the messages in this dependency
		dependencyMessages, dependencyEnums, err := nestedDescriptorMessages(*genFiles[dep], genFiles)
		if err != nil {
			return nil, nil, err
		}
		for name, enum := range dependencyEnums {
			enums[name] = enum
		}
		for name, msg := range dependencyMessages {
			messages[name] = msg
		}
	}
	for name, enum := range parseEnums(pkgName, comments, file.GetEnumType()) {
		enums[name] = enum
	}
	for name, msg := range parseMessages(pkgName, comments, file.GetMessageType(), messages, enums) {
		messages[name] = msg
	}
	return messages, enums, nil
}

// isDepNeeded checks if the dependency is truly needed for message types (annotation)
// by going through all messages in the file and checking for a package match
func isDepNeeded(file descriptorpb.FileDescriptorProto, dep *descriptorpb.FileDescriptorProto) bool {
	for _, message := range file.GetMessageType() {
		for _, field := range message.GetField() {
			if field.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE ||
				field.GetType() == descriptorpb.FieldDescriptorProto_TYPE_ENUM {
				if fieldPackage(field.GetTypeName()) == *dep.Package {
					return true
				}
			}
		}
	}
	return false
}

// fieldPackage gets the package owner of a field from the name e.g
// types.CustomDate returns types
func fieldPackage(typeName string) string {
	return strings.Trim(typeName[:strings.LastIndex(typeName, ".")], ".")
}

func parseEnums(pkgName string, comments map[string]*Comment, enums []*descriptorpb.EnumDescriptorProto) map[string]*Enum {
	res := map[string]*Enum{}
	for i, e := range enums {
		p := fmt.Sprintf("%d.%d", enumFlag, i)
		res[getQualifiedName(pkgName, e.GetName())] = &Enum{
			Name:    e.GetName(),
			Comment: nonNilComment(comments[p]),
			Values:  parseValues(p, comments, e.GetValue(), e.GetName()),
		}
	}
	return res
}

func parseValues(
	path string,
	comments map[string]*Comment,
	values []*descriptorpb.EnumValueDescriptorProto,
	typeName string,
) []*Value {
	res := make([]*Value, 0, len(values))
	for i, v := range values {
		if v.GetName() != "_" {
			p := fmt.Sprintf("%s.%d.%d", path, fieldFlag, i)
			comment := comments[p]
			comment = nonNilComment(comment)
			comment.Leading = strings.TrimPrefix(comment.Leading, typeName+"_")
			res = append(res, &Value{
				Name:    v.GetName(),
				Comment: comment,
			})
		}
	}
	return res
}

// extractMapFields generates an array of fields that correspond to map type fields for messages, which are stored in
// the NestedType field of the DescriptorProto struct
func extractMapFields(message *descriptorpb.DescriptorProto, path string, comments map[string]*Comment) []*Field {
	fields := make([]*Field, len(message.NestedType))
	for i, n := range message.GetNestedType() {
		// a nested type of map will always have two fields for key and value
		if !n.GetOptions().GetMapEntry() {
			continue
		}
		if len(n.GetField()) != 2 {
			panic("a map type message should always have two fields")
		}
		nestedTypeName := strings.TrimSuffix(n.GetName(), "Entry")
		// find position and fieldDescriptorProto of the map type from the message fields
		pos := -1
		var fieldDescriptorProto *descriptorpb.FieldDescriptorProto
		for i, f := range message.GetField() {
			if f.GetName() == nestedTypeName {
				pos = i
				fieldDescriptorProto = f
				break
			}
		}
		if pos == -1 || fieldDescriptorProto == nil {
			continue
		}
		// get key type. key can only be string or int32
		var keyType string
		switch n.GetField()[0].GetType() {
		case descriptorpb.FieldDescriptorProto_TYPE_INT32:
			keyType = "int"
		case descriptorpb.FieldDescriptorProto_TYPE_STRING:
			keyType = "string"
		default:
			panic("key should either be a string or an int32")
		}
		field := Field{
			Name:    nestedTypeName,
			Comment: nonNilComment(comments[fmt.Sprintf("%s.%d.%d", path, fieldFlag, pos)]),
			Type: &Type{
				Name:    fmt.Sprintf("map[%s]%s", keyType, getType(n.GetField()[1]).Name),
				IsArray: true,
			},
		}
		fields[i] = &field
	}
	return fields
}

func parseMessages(
	pkgName string,
	comments map[string]*Comment,
	messages []*descriptorpb.DescriptorProto,
	merge map[string]*Message,
	allEnums map[string]*Enum,
) map[string]*Message {
	// convert each proto message to our Message representation
	for i, m := range messages {

		var fixedExample map[string]interface{}
		if proto.HasExtension(m.GetOptions(), options.E_Openapiv2Schema) {
			ext := proto.GetExtension(m.GetOptions(), options.E_Openapiv2Schema)
			if ext != nil {
				if schema, ok := ext.(*options.Schema); ok &&
					schema.Example != "" {

					fixedExample = map[string]interface{}{}
					if err := json.Unmarshal([]byte(schema.Example), &fixedExample); err != nil {
						panic(fmt.Sprintf("Invalid message example value, value should be a json object: %v", err))
					}
				}
			}
		}

		p := fmt.Sprintf("%d.%d", messageFlag, i)
		mapFields := extractMapFields(m, p, comments)
		fields := parseFields(p, comments, m.GetField())
		fields = append(fields, mapFields...)
		merge[getQualifiedName(pkgName, m.GetName())] = &Message{
			Name:         m.GetName(),
			Comment:      nonNilComment(comments[p]),
			Fields:       fields,
			FixedExample: fixedExample,
		}
	}
	// once we've defined all Messages, populate the NestedMessages
	// member of each Message
	for _, m := range merge {
		populateNestedMessages(m, merge, allEnums)
	}
	return merge
}

// populateNestedMessages populates the NestedMessages field of a Message.
//
// its first argument is the Message to check. an iterative breadth-first search is conducted
// to find any fields that are Messages (whose fields are then searched, and so on).
// these are then added to the original Message's NestedMessages field.
//
// the definitions of the Messages are taken from the second argument,
// which is a map that is expected to contain all Messages parsed from the package.
func populateNestedMessages(
	message *Message,
	messages map[string]*Message,
	allEnums map[string]*Enum,
) {
	var nestedMessages []*Message
	var enums []*Enum
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
			if !(typ.IsMessage || typ.IsEnum) || seen[name] {
				continue
			}
			seen[name] = true
			if typ.IsMessage {
				nm, ok := messages[name]
				if !ok { // shouldn't happen
					panic(fmt.Sprintf("message %q not found", name))
				}
				stack = append(stack, nm)
				nestedMessages = append(nestedMessages, nm)
			} else {
				en, ok := allEnums[name]
				if !ok { // shouldn't happen
					panic(fmt.Sprintf("enum %q not found", name))
				}
				enums = append(enums, en)
			}
		}
	}
	message.NestedMessages = nestedMessages
	message.Enums = enums
}

func parseFields(path string, comments map[string]*Comment, fields []*descriptorpb.FieldDescriptorProto) []*Field {
	var res []*Field
	for i, f := range fields {
		// check if this field is a map which should be handled by the extractMapField
		if strings.Contains(f.GetTypeName(), f.GetName()+"Entry") {
			continue
		}

		var fixedExample json.RawMessage
		if proto.HasExtension(f.GetOptions(), options.E_Openapiv2Field) {
			ext := proto.GetExtension(f.GetOptions(), options.E_Openapiv2Field)
			if ext != nil {
				if jsonSchema, ok := ext.(*options.JSONSchema); ok &&
					jsonSchema.Example != "" {

					fixedExample = json.RawMessage(jsonSchema.Example)
				}
			}
		}

		field := &Field{
			Name:         f.GetName(),
			Comment:      nonNilComment(comments[fmt.Sprintf("%s.%d.%d", path, fieldFlag, i)]),
			Type:         getType(f),
			JSONName:     f.GetJsonName(),
			FixedExample: fixedExample,
		}
		res = append(res, field)
	}
	return res
}

func getType(f *descriptorpb.FieldDescriptorProto) *Type {
	typ := f.GetType()
	t := &Type{
		IsMessage:     typ == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE,
		IsEnum:        typ == descriptorpb.FieldDescriptorProto_TYPE_ENUM,
		IsArray:       f.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED,
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

func parseServices(pkgName string, messages map[string]*Message, services []*descriptorpb.ServiceDescriptorProto, onlyExternal OnlyExternalOpt) (map[string]*Service, error) {
	res := map[string]*Service{}
	for _, s := range services {
		methods, err := parseMethods(pkgName, messages, s.GetMethod(), onlyExternal)
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

func hasExternalTag(tags []string) bool {
	const externalTag = "external"
	for _, tag := range tags {
		if tag == externalTag {
			return true
		}
	}
	return false
}

func parseMethods(pkgName string, messages map[string]*Message, methods []*descriptorpb.MethodDescriptorProto, onlyExternal OnlyExternalOpt) (map[string]*Method, error) {
	res := map[string]*Method{}
	for _, m := range methods {
		extOp := proto.GetExtension(m.GetOptions(), options.E_Openapiv2Operation)
		if extOp == nil {
			continue
		}
		operation := extOp.(*options.Operation)
		if operation == nil {
			continue
		}
		if onlyExternal == OnlyExternalEnabled && !hasExternalTag(operation.Tags) {
			continue
		}
		extHTTP := proto.GetExtension(m.GetOptions(), annotations.E_Http)
		if extHTTP == nil {
			continue
		}
		req, err := parseRequest(extHTTP.(*annotations.HttpRule), messages, m.GetInputType())
		if err != nil {
			return nil, err
		}
		var rsp *Response
		rspMsg, ok := messages[m.GetOutputType()]
		if ok {
			example, err := generateResponseExample(messages, m.GetOutputType())
			if err != nil {
				return nil, err
			}
			rsp = &Response{
				Message: rspMsg,
				Example: example,
			}
		}
		res[getQualifiedName(pkgName, m.GetName())] = &Method{
			Name:      m.GetName(),
			Request:   req,
			Response:  rsp,
			Operation: operation,
		}
	}
	return res, nil
}

func generateResponseExample(messages map[string]*Message, name string) (string, error) {
	_, ok := messages[name]
	if !ok {
		return "", nil
	}
	b, err := genJSONExample(messages, name)
	if err != nil {
		return "", err
	}
	var indented bytes.Buffer
	if err = json.Indent(&indented, b, "", "  "); err != nil {
		return "", err
	}
	return indented.String(), nil
}

// normalizeRequestFields is used to filter out all fields marked with an predefined indicator of a request message.
func normalizeRequestFields(messages map[string]*Message) {
	const hidingIndicator = "docgen: hide"
	for _, message := range messages {
		var filtered []*Field
		for _, f := range message.Fields {
			if !strings.Contains(f.Comment.Leading, hidingIndicator) {
				filtered = append(filtered, f)
			}
		}
		message.Fields = filtered
	}
}

func parseRequest(rule *annotations.HttpRule, messages map[string]*Message, name string) (*Request, error) {
	var verb, uri string
	switch p := rule.GetPattern().(type) {
	case *annotations.HttpRule_Get:
		verb = http.MethodGet
		uri = p.Get
	case *annotations.HttpRule_Post:
		verb = http.MethodPost
		uri = p.Post
	case *annotations.HttpRule_Put:
		verb = http.MethodPut
		uri = p.Put
	case *annotations.HttpRule_Delete:
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
		normalizeRequestFields(messages)
		body = messages[name]
		b, err := genJSONExample(messages, name)
		if err != nil {
			return nil, err
		}
		var indented bytes.Buffer
		if err = json.Indent(&indented, b, "\t", "\t"); err != nil {
			return nil, err
		}
		example = indented.String()
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
		if message == nil {
			return nil, fmt.Errorf("the URL %s has no request type, field %s cannot be matched", uri, tf)
		}
		for _, mf := range message.Fields {
			if tf == mf.Name {
				res[i] = mf
			}
		}
		if res[i] == nil {
			return nil, fmt.Errorf("the URL %s field %s cannot be matched in %s", uri, tf, message.Name)
		}
	}
	return res, nil
}

func parseComments(sci *descriptorpb.SourceCodeInfo) map[string]*Comment {
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

func genJSONExample(messages map[string]*Message, path string) ([]byte, error) {
	p := genJSONExampleIn(messages, path)
	b, err := p.MarshalJSON()
	if err != nil {
		return nil, err
	}
	return (b), nil
}

func genJSONExampleIn(messages map[string]*Message, path string) properties {
	op := properties{}

	m := messages[path]
	if len(m.FixedExample) > 0 {
		for _, k := range sortKeys(m.FixedExample) {
			op = append(op, keyVal{Key: k, Value: m.FixedExample[k]})
		}
		return op
	}

	for _, f := range m.Fields {
		if f.Type.QualifiedName == "" || f.Type.IsEnum {
			if f.JSONName != "-" {
				var value interface{} = f.Type.Name
				if len(f.FixedExample) > 0 {
					value = f.FixedExample
				}
				op = append(op, keyVal{Key: f.JSONName, Value: value})
			}
			continue
		}

		if f.JSONName != "-" {
			if len(f.FixedExample) > 0 {
				op = append(op, keyVal{Key: f.JSONName, Value: f.FixedExample})
				continue
			}

			var v interface{} = genJSONExampleIn(messages, f.Type.QualifiedName)
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
	buf.WriteString("{")
	for i, kv := range p {
		if i != 0 {
			buf.WriteString(",")
		}
		key, err := json.Marshal(kv.Key)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteString(": ")
		val, err := json.Marshal(kv.Value)
		if err != nil {
			return nil, err
		}
		buf.Write(val)
	}
	buf.WriteString("}")
	return buf.Bytes(), nil
}

func sortKeys(m map[string]interface{}) []string {
	ks := make([]string, len(m))

	i := 0
	for k := range m {
		ks[i] = k
		i++
	}

	sort.Strings(ks)
	return ks
}
