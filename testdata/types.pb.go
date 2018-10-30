// Code generated by protoc-gen-go. DO NOT EDIT.
// source: testdata.tld/util/types.gunk

package util

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// Status is a server health status.
type Status int32

const (
	// Status_Unknown is the default, unset status value.
	Status_Unknown Status = 0
	// Status_Error is a status value that implies something went wrong.
	Status_Error Status = 1
	// Status_OK is a status value used when all went well.
	Status_OK Status = 2
)

var Status_name = map[int32]string{
	0: "Unknown",
	1: "Error",
	2: "OK",
}
var Status_value = map[string]int32{
	"Unknown": 0,
	"Error":   1,
	"OK":      2,
}

func (x Status) String() string {
	return proto.EnumName(Status_name, int32(x))
}
func (Status) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_types_1412d5445d6745ba, []int{0}
}

func init() {
	proto.RegisterEnum("testdata.tld/util.Status", Status_name, Status_value)
}

func init() { proto.RegisterFile("testdata.tld/util/types.gunk", fileDescriptor_types_1412d5445d6745ba) }

var fileDescriptor_types_1412d5445d6745ba = []byte{
	// 124 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x92, 0x29, 0x49, 0x2d, 0x2e,
	0x49, 0x49, 0x2c, 0x49, 0xd4, 0x2b, 0xc9, 0x49, 0xd1, 0x2f, 0x2d, 0xc9, 0xcc, 0xd1, 0x2f, 0xa9,
	0x2c, 0x48, 0x2d, 0xd6, 0x4b, 0x2f, 0xcd, 0xcb, 0x16, 0x12, 0xc4, 0x90, 0x95, 0x92, 0xc6, 0xd4,
	0x90, 0x9a, 0x9c, 0x91, 0x0f, 0x56, 0xaf, 0xa5, 0xc1, 0xc5, 0x16, 0x5c, 0x92, 0x58, 0x52, 0x5a,
	0x2c, 0xc4, 0xcd, 0xc5, 0x1e, 0x9a, 0x97, 0x9d, 0x97, 0x5f, 0x9e, 0x27, 0xc0, 0x20, 0xc4, 0xc9,
	0xc5, 0xea, 0x5a, 0x54, 0x94, 0x5f, 0x24, 0xc0, 0x28, 0xc4, 0xc6, 0xc5, 0xe4, 0xef, 0x2d, 0xc0,
	0xe4, 0xc4, 0x16, 0xc5, 0x02, 0xd2, 0x9b, 0xc4, 0x56, 0x50, 0x94, 0x5f, 0x92, 0x6f, 0x0c, 0x08,
	0x00, 0x00, 0xff, 0xff, 0x84, 0xee, 0x48, 0x69, 0x88, 0x00, 0x00, 0x00,
}
