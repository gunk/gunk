package generate

import (
	"fmt"
	"go/constant"
	"reflect"
	"strconv"

	"google.golang.org/protobuf/proto"
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

// protoNumber returns the protobuf number stored in the struct tag if
// available.
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

// protoNumber returns the JSON field name stored in the struct tag if
// available.
func jsonName(tag reflect.StructTag) *string {
	jsonTag := tag.Get("json")
	if jsonTag == "" {
		return nil
	}
	return proto.String(jsonTag)
}

// protoStringOrNil returns a proto string if the string is non-empty and a nil
// if the string is empty.
func protoStringOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return proto.String(s)
}

// protoEnumValue converts the provided constant.Value to an int32.
// It panics if the value is not a number.
func protoEnumValue(value constant.Value) int32 {
	val, ok := constant.Int64Val(value)
	if !ok {
		panic(fmt.Sprintf("value %#v is unknown", value))
	}
	return int32(val)
}

// unifiedProtoFile returns the proto file name that a Gunk package is
// translated into. Note that the returned name isn't a path on disk; it's
// merely a unique path to identify each package's proto file and its output
// from each of the code generators.
func unifiedProtoFile(pkgPath string) string {
	return pkgPath + "/all.proto"
}
