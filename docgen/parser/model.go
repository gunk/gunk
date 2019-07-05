package parser

import (
	"github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger/options"
)

// File is a proto parsed file.
type File struct {
	Swagger  *options.Swagger
	Messages map[string]*Message
	Services map[string]*Service
}

// Message describes a proto message.
type Message struct {
	Name    string
	Comment *Comment
	Fields  []*Field
}

// Field describes a proto field.
type Field struct {
	Name     string
	Comment  *Comment
	Type     *Type
	JSONName string
}

// Type describes a field type.
type Type struct {
	Name          string
	QualifiedName string
	IsArray       bool
	IsEnum        bool
}

// Service describes a proto service.
type Service struct {
	Name    string
	Comment *Comment
	Methods map[string]*Method
}

// Method describes a proto method.
type Method struct {
	Name      string
	Comment   *Comment
	Request   *Request
	Response  *Message
	Operation *options.Operation
	Curl      string
}

// Request describes an HTTP request.
type Request struct {
	Verb    string
	URI     string
	Body    *Message
	Query   []*Field
	Example string
}

// Comment describes a comment.
type Comment struct {
	Leading  string
	Trailing string
	Detached []string
}
