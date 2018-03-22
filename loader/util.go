package loader

import (
	"go/ast"
	"reflect"
	"strconv"
	"strings"

	"github.com/golang/protobuf/proto"
)

const (
	packagePath       = 2 // FileDescriptorProto.Package
	messagePath       = 4 // FileDescriptorProto.MessageType
	enumPath          = 5 // FileDescriptorProto.EnumType
	servicePath       = 6 // FileDescriptorProto.Service
	messageFieldPath  = 2 // DescriptorProto.Field
	enumValuePath     = 2 // EnumDescriptorProto.Value
	serviceMethodPath = 2 // ServiceDescriptorProto.Method
)

// namePrefix returns a func which prefixes the supplied name.
func namePrefix(name string) func(string) string {
	return func(text string) string {
		return name + "_" + text
	}
}

// splitGunkTag splits a '+gunk' tag.
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

func protoNumber(fieldTag *ast.BasicLit) *int32 {
	if fieldTag == nil {
		return nil
	}
	str, _ := strconv.Unquote(fieldTag.Value)
	tag := reflect.StructTag(str)
	number, _ := strconv.Atoi(tag.Get("pb"))
	return proto.Int32(int32(number))
}
