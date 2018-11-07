package generate

import (
	"fmt"
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

func protoNumber(tag reflect.StructTag) (*int32, error) {
	pbTag := tag.Get("pb")
	if pbTag == "" {
		return nil, fmt.Errorf("pb tag must be set")
	}
	number, err := strconv.Atoi(pbTag)
	if err != nil {
		return nil, err
	}
	return proto.Int32(int32(number)), nil
}

func jsonName(tag reflect.StructTag) *string {
	jsonTag := tag.Get("json")
	if jsonTag == "" {
		return nil
	}
	return proto.String(jsonTag)
}
