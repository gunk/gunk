package parser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2/options"
	"google.golang.org/protobuf/types/descriptorpb"
)

// FileDescWrapper is a wrapper for FileDescriptorProto for holding a map of dependencies.
type FileDescWrapper struct {
	*descriptorpb.FileDescriptorProto
	DependencyMap map[string]*descriptorpb.FileDescriptorProto
}

// File is a proto parsed file.
type File struct {
	Swagger  *options.Swagger
	Services map[string]*Service
	Enums    map[string]*Enum
}

// SwaggerScheme gets scheme that you most probably want.
// The order is https - http - wss - ws.
// Includes "://" string if present, returns empty string if there is no valid scheme.
func (f *File) SwaggerScheme() string {
	if len(f.Swagger.Schemes) == 0 {
		return ""
	}
	hasScheme := map[options.Scheme]bool{}
	for _, scheme := range f.Swagger.Schemes {
		hasScheme[scheme] = true
	}
	if hasScheme[options.Scheme_HTTPS] {
		return "https://"
	}
	if hasScheme[options.Scheme_HTTP] {
		return "http://"
	}
	// TODO - should we include websocket at all? CURL won't work anyway
	if hasScheme[options.Scheme_WSS] {
		return "wss://"
	}
	if hasScheme[options.Scheme_WS] {
		return "ws://"
	}
	return ""
}

// HasServices returns true when file contains service definitions.
func (f *File) HasServices() bool {
	return len(f.Services) > 0
}

// CheckSwagger checks existence of Swagger annotations, that are needed
// for proper generation of the markdown documentation.
func (f *File) CheckSwagger() error {
	if f.Swagger == nil {
		return fmt.Errorf("needs an openapiv2.Swagger annotation")
	}
	if f.Swagger.Info == nil {
		return fmt.Errorf("needs a swagger.info")
	}
	if f.Swagger.Info.Title == "" {
		return fmt.Errorf("needs a swagger.info.title")
	}
	if f.Swagger.Info.Version == "" {
		return fmt.Errorf("needs a swagger.info.version")
	}
	for _, s := range f.Services {
		for _, m := range s.Methods {
			if m.Operation == nil {
				return fmt.Errorf("method %s needs a swagger operation", m.Name)
			}
			if m.Operation.Summary == "" {
				return fmt.Errorf("method %s needs a swagger operation summary", m.Name)
			}
		}
	}
	return nil
}

// Message describes a proto message.
type Message struct {
	Name           string
	Comment        *Comment
	Fields         []*Field
	NestedMessages []*Message
	Enums          []*Enum
	FixedExample   map[string]interface{}
}

// Response describes a response of a service.
type Response struct {
	*Message
	Example string
}

// Field describes a proto field.
type Field struct {
	Name         string
	Comment      *Comment
	Type         *Type
	JSONName     string
	FixedExample json.RawMessage
}

// Type describes a field type.
type Type struct {
	Name          string
	QualifiedName string
	IsMessage     bool
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
	Response  *Response
	Operation *options.Operation
}

func (m *Method) HeaderID() string {
	name := "method-"
	name = name + strings.ToLower(m.Request.Verb) + "-"
	name = name + strings.ToLower(m.Name)
	return name
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

// Enum describes an enumeration type.
type Enum struct {
	Name    string
	Comment *Comment
	Values  []*Value
}

// Value describes an enum possible value.
type Value struct {
	Name    string
	Comment *Comment
}
