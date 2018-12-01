// Code generated by protoc-gen-go. DO NOT EDIT.
// source: testdata.tld/util/all.proto

package util

/*
package util contains a simple Echo service.
*/

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import duration "github.com/golang/protobuf/ptypes/duration"
import empty "github.com/golang/protobuf/ptypes/empty"
import timestamp "github.com/golang/protobuf/ptypes/timestamp"
import imported "testdata.tld/util/imported"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

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
	return fileDescriptor_all_a604d30130a3b3a1, []int{0}
}

// CheckStatusResponse is the response for a check status.
type CheckStatusResponse struct {
	Status               Status   `protobuf:"varint,1,opt,name=Status,json=status,proto3,enum=testdata.v1.util.Status" json:"Status,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CheckStatusResponse) Reset()         { *m = CheckStatusResponse{} }
func (m *CheckStatusResponse) String() string { return proto.CompactTextString(m) }
func (*CheckStatusResponse) ProtoMessage()    {}
func (*CheckStatusResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_all_a604d30130a3b3a1, []int{0}
}
func (m *CheckStatusResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CheckStatusResponse.Unmarshal(m, b)
}
func (m *CheckStatusResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CheckStatusResponse.Marshal(b, m, deterministic)
}
func (dst *CheckStatusResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CheckStatusResponse.Merge(dst, src)
}
func (m *CheckStatusResponse) XXX_Size() int {
	return xxx_messageInfo_CheckStatusResponse.Size(m)
}
func (m *CheckStatusResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_CheckStatusResponse.DiscardUnknown(m)
}

var xxx_messageInfo_CheckStatusResponse proto.InternalMessageInfo

func (m *CheckStatusResponse) GetStatus() Status {
	if m != nil {
		return m.Status
	}
	return Status_Unknown
}

type UtilTestRequest struct {
	Ints                 []int32                        `protobuf:"varint,1,rep,packed,name=Ints,proto3" json:"Ints,omitempty"`
	Structs              []*imported.Message            `protobuf:"bytes,2,rep,name=Structs,proto3" json:"Structs,omitempty"`
	Bool                 bool                           `protobuf:"varint,3,opt,name=Bool,proto3" json:"Bool,omitempty"`
	Int32                int32                          `protobuf:"varint,4,opt,name=Int32,proto3" json:"Int32,omitempty"`
	Timestamp            *timestamp.Timestamp           `protobuf:"bytes,5,opt,name=Timestamp,proto3" json:"Timestamp,omitempty"`
	Duration             *duration.Duration             `protobuf:"bytes,6,opt,name=Duration,proto3" json:"Duration,omitempty"`
	MapSimple            map[string]int32               `protobuf:"bytes,7,rep,name=MapSimple,proto3" json:"MapSimple,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"varint,2,opt,name=value,proto3"`
	MapComplex           map[int32]*CheckStatusResponse `protobuf:"bytes,8,rep,name=MapComplex,proto3" json:"MapComplex,omitempty" protobuf_key:"varint,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	XXX_NoUnkeyedLiteral struct{}                       `json:"-"`
	XXX_unrecognized     []byte                         `json:"-"`
	XXX_sizecache        int32                          `json:"-"`
}

func (m *UtilTestRequest) Reset()         { *m = UtilTestRequest{} }
func (m *UtilTestRequest) String() string { return proto.CompactTextString(m) }
func (*UtilTestRequest) ProtoMessage()    {}
func (*UtilTestRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_all_a604d30130a3b3a1, []int{1}
}
func (m *UtilTestRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_UtilTestRequest.Unmarshal(m, b)
}
func (m *UtilTestRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_UtilTestRequest.Marshal(b, m, deterministic)
}
func (dst *UtilTestRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_UtilTestRequest.Merge(dst, src)
}
func (m *UtilTestRequest) XXX_Size() int {
	return xxx_messageInfo_UtilTestRequest.Size(m)
}
func (m *UtilTestRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_UtilTestRequest.DiscardUnknown(m)
}

var xxx_messageInfo_UtilTestRequest proto.InternalMessageInfo

func (m *UtilTestRequest) GetInts() []int32 {
	if m != nil {
		return m.Ints
	}
	return nil
}

func (m *UtilTestRequest) GetStructs() []*imported.Message {
	if m != nil {
		return m.Structs
	}
	return nil
}

func (m *UtilTestRequest) GetBool() bool {
	if m != nil {
		return m.Bool
	}
	return false
}

func (m *UtilTestRequest) GetInt32() int32 {
	if m != nil {
		return m.Int32
	}
	return 0
}

func (m *UtilTestRequest) GetTimestamp() *timestamp.Timestamp {
	if m != nil {
		return m.Timestamp
	}
	return nil
}

func (m *UtilTestRequest) GetDuration() *duration.Duration {
	if m != nil {
		return m.Duration
	}
	return nil
}

func (m *UtilTestRequest) GetMapSimple() map[string]int32 {
	if m != nil {
		return m.MapSimple
	}
	return nil
}

func (m *UtilTestRequest) GetMapComplex() map[int32]*CheckStatusResponse {
	if m != nil {
		return m.MapComplex
	}
	return nil
}

func init() {
	proto.RegisterType((*CheckStatusResponse)(nil), "testdata.v1.util.CheckStatusResponse")
	proto.RegisterType((*UtilTestRequest)(nil), "testdata.v1.util.UtilTestRequest")
	proto.RegisterMapType((map[int32]*CheckStatusResponse)(nil), "testdata.v1.util.UtilTestRequest.MapComplexEntry")
	proto.RegisterMapType((map[string]int32)(nil), "testdata.v1.util.UtilTestRequest.MapSimpleEntry")
	proto.RegisterEnum("testdata.v1.util.Status", Status_name, Status_value)
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// UtilClient is the client API for Util service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type UtilClient interface {
	// Echo echoes a message.
	Echo(ctx context.Context, in *imported.Message, opts ...grpc.CallOption) (*imported.Message, error)
	// CheckStatus sends the server health status.
	CheckStatus(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*CheckStatusResponse, error)
}

type utilClient struct {
	cc *grpc.ClientConn
}

func NewUtilClient(cc *grpc.ClientConn) UtilClient {
	return &utilClient{cc}
}

func (c *utilClient) Echo(ctx context.Context, in *imported.Message, opts ...grpc.CallOption) (*imported.Message, error) {
	out := new(imported.Message)
	err := c.cc.Invoke(ctx, "/testdata.v1.util.Util/Echo", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *utilClient) CheckStatus(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*CheckStatusResponse, error) {
	out := new(CheckStatusResponse)
	err := c.cc.Invoke(ctx, "/testdata.v1.util.Util/CheckStatus", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UtilServer is the server API for Util service.
type UtilServer interface {
	// Echo echoes a message.
	Echo(context.Context, *imported.Message) (*imported.Message, error)
	// CheckStatus sends the server health status.
	CheckStatus(context.Context, *empty.Empty) (*CheckStatusResponse, error)
}

func RegisterUtilServer(s *grpc.Server, srv UtilServer) {
	s.RegisterService(&_Util_serviceDesc, srv)
}

func _Util_Echo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(imported.Message)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UtilServer).Echo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/testdata.v1.util.Util/Echo",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UtilServer).Echo(ctx, req.(*imported.Message))
	}
	return interceptor(ctx, in, info, handler)
}

func _Util_CheckStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(empty.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UtilServer).CheckStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/testdata.v1.util.Util/CheckStatus",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UtilServer).CheckStatus(ctx, req.(*empty.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

var _Util_serviceDesc = grpc.ServiceDesc{
	ServiceName: "testdata.v1.util.Util",
	HandlerType: (*UtilServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Echo",
			Handler:    _Util_Echo_Handler,
		},
		{
			MethodName: "CheckStatus",
			Handler:    _Util_CheckStatus_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "testdata.tld/util/all.proto",
}

// UtilTestsClient is the client API for UtilTests service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type UtilTestsClient interface {
	UtilTest(ctx context.Context, in *UtilTestRequest, opts ...grpc.CallOption) (*CheckStatusResponse, error)
}

type utilTestsClient struct {
	cc *grpc.ClientConn
}

func NewUtilTestsClient(cc *grpc.ClientConn) UtilTestsClient {
	return &utilTestsClient{cc}
}

func (c *utilTestsClient) UtilTest(ctx context.Context, in *UtilTestRequest, opts ...grpc.CallOption) (*CheckStatusResponse, error) {
	out := new(CheckStatusResponse)
	err := c.cc.Invoke(ctx, "/testdata.v1.util.UtilTests/UtilTest", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UtilTestsServer is the server API for UtilTests service.
type UtilTestsServer interface {
	UtilTest(context.Context, *UtilTestRequest) (*CheckStatusResponse, error)
}

func RegisterUtilTestsServer(s *grpc.Server, srv UtilTestsServer) {
	s.RegisterService(&_UtilTests_serviceDesc, srv)
}

func _UtilTests_UtilTest_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UtilTestRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(UtilTestsServer).UtilTest(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/testdata.v1.util.UtilTests/UtilTest",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(UtilTestsServer).UtilTest(ctx, req.(*UtilTestRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _UtilTests_serviceDesc = grpc.ServiceDesc{
	ServiceName: "testdata.v1.util.UtilTests",
	HandlerType: (*UtilTestsServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "UtilTest",
			Handler:    _UtilTests_UtilTest_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "testdata.tld/util/all.proto",
}

func init() { proto.RegisterFile("testdata.tld/util/all.proto", fileDescriptor_all_a604d30130a3b3a1) }

var fileDescriptor_all_a604d30130a3b3a1 = []byte{
	// 601 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x8c, 0x53, 0x4f, 0x6b, 0xd4, 0x5e,
	0x14, 0x7d, 0xef, 0x4d, 0x26, 0xcd, 0xdc, 0xa1, 0xed, 0xf0, 0x7e, 0x50, 0x5e, 0x53, 0xf8, 0x75,
	0x0c, 0x0a, 0xa1, 0x8b, 0xa4, 0x4d, 0x05, 0x6b, 0x11, 0x84, 0xd6, 0x01, 0x8b, 0x94, 0xd6, 0xfe,
	0x01, 0x51, 0x37, 0xe9, 0xcc, 0xb3, 0x0d, 0xcd, 0xe4, 0xc5, 0xbc, 0x97, 0x6a, 0x77, 0xe2, 0xaa,
	0xcb, 0x82, 0x5f, 0xc6, 0x2f, 0x21, 0x08, 0x6e, 0x5c, 0xb8, 0xf4, 0x43, 0xb8, 0x94, 0x64, 0x26,
	0xe3, 0x24, 0x11, 0x9c, 0xdd, 0x24, 0xe7, 0x9e, 0x73, 0xee, 0x3d, 0x73, 0x02, 0x2b, 0x8a, 0x4b,
	0x35, 0xf0, 0x95, 0xef, 0xa8, 0x70, 0xe0, 0xa6, 0x2a, 0x08, 0x5d, 0x3f, 0x0c, 0x9d, 0x38, 0x11,
	0x4a, 0xd0, 0xce, 0x04, 0xbc, 0xda, 0x70, 0x32, 0xcc, 0x5c, 0x39, 0x17, 0xe2, 0x3c, 0xe4, 0x6e,
	0x8e, 0x9f, 0xa5, 0x6f, 0x5c, 0x3e, 0x8c, 0xd5, 0xf5, 0x68, 0xdc, 0x5c, 0xad, 0x82, 0x2a, 0x18,
	0x72, 0xa9, 0xfc, 0x61, 0x3c, 0x1e, 0xf8, 0xbf, 0x3a, 0x30, 0x48, 0x13, 0x5f, 0x05, 0x22, 0x1a,
	0xe3, 0x77, 0xeb, 0xcb, 0x04, 0xc3, 0x58, 0x24, 0x8a, 0x0f, 0xfe, 0x6c, 0x65, 0xbd, 0x82, 0xff,
	0x76, 0x2f, 0x78, 0xff, 0xf2, 0x58, 0xf9, 0x2a, 0x95, 0x47, 0x5c, 0xc6, 0x22, 0x92, 0x9c, 0x3e,
	0x02, 0x7d, 0xf4, 0x86, 0xe1, 0x2e, 0xb6, 0x17, 0x3c, 0xe6, 0x54, 0xb7, 0x77, 0x46, 0xf8, 0x0e,
	0x18, 0x88, 0x21, 0x1b, 0xad, 0xa3, 0x43, 0x74, 0xa4, 0xcb, 0xfc, 0xdd, 0xb6, 0x6e, 0xa0, 0x0e,
	0x62, 0xc8, 0xfa, 0xa2, 0xc1, 0xe2, 0xa9, 0x0a, 0xc2, 0x13, 0x2e, 0xd5, 0x11, 0x7f, 0x9b, 0x72,
	0xa9, 0x28, 0x03, 0x6d, 0x2f, 0x52, 0x99, 0x6e, 0xc3, 0x6e, 0x4e, 0xb3, 0xe9, 0x63, 0x98, 0x3b,
	0x56, 0x49, 0xda, 0x57, 0x92, 0x91, 0x6e, 0xc3, 0x6e, 0x7b, 0x56, 0xdd, 0xb4, 0xb8, 0xc0, 0xd9,
	0xe7, 0x52, 0xfa, 0xe7, 0xbc, 0x24, 0xc0, 0x40, 0xdb, 0x11, 0x22, 0x64, 0x8d, 0x2e, 0xb6, 0x8d,
	0x12, 0xb2, 0x0c, 0xcd, 0xbd, 0x48, 0x6d, 0x7a, 0x4c, 0xeb, 0xe2, 0x8a, 0xeb, 0x43, 0x68, 0x9d,
	0x14, 0xc9, 0xb2, 0x66, 0x17, 0xdb, 0x6d, 0xcf, 0x74, 0x46, 0xd1, 0x3a, 0x45, 0xb4, 0xce, 0x64,
	0xa2, 0x44, 0x7d, 0x00, 0xc6, 0x93, 0x71, 0xe6, 0x4c, 0xcf, 0x99, 0xcb, 0x35, 0x66, 0x31, 0x50,
	0x22, 0x1e, 0x40, 0x6b, 0xdf, 0x8f, 0x8f, 0x83, 0x61, 0x1c, 0x72, 0x36, 0x97, 0xdf, 0xba, 0x5e,
	0xbf, 0xb5, 0x92, 0x9c, 0x33, 0xa1, 0xf4, 0x22, 0x95, 0x5c, 0x97, 0x04, 0x9f, 0x03, 0xec, 0xfb,
	0xf1, 0xae, 0xc8, 0xd0, 0xf7, 0xcc, 0xc8, 0x15, 0x37, 0x66, 0x52, 0x1c, 0x73, 0x6a, 0x92, 0xe6,
	0x3a, 0x2c, 0x94, 0x0d, 0x69, 0x1b, 0x1a, 0x97, 0xfc, 0x3a, 0x2f, 0x44, 0x8b, 0xce, 0x43, 0xf3,
	0xca, 0x0f, 0x53, 0xce, 0x48, 0x96, 0xe8, 0x36, 0xd9, 0xc2, 0xe6, 0x0b, 0x58, 0xac, 0x08, 0x4e,
	0x53, 0x9a, 0xf4, 0xfe, 0x34, 0xa5, 0xed, 0xdd, 0xab, 0xef, 0xf7, 0x97, 0x26, 0x66, 0xca, 0x45,
	0x9f, 0xd6, 0xb6, 0x8a, 0x56, 0xd2, 0x45, 0x98, 0x3b, 0x8d, 0x2e, 0x23, 0xf1, 0x2e, 0xea, 0x20,
	0x93, 0x18, 0x28, 0xdb, 0xa7, 0x97, 0x24, 0x22, 0xe9, 0xe0, 0xfc, 0x11, 0x80, 0x1c, 0x3c, 0xeb,
	0x90, 0xec, 0xb7, 0x49, 0x18, 0xf2, 0x7e, 0x60, 0xd0, 0xb2, 0xeb, 0x69, 0x00, 0x5a, 0xaf, 0x7f,
	0x21, 0xe8, 0x0c, 0xdd, 0x32, 0x67, 0x98, 0xb1, 0x96, 0x3f, 0x7e, 0xfb, 0xf9, 0x89, 0xcc, 0x6f,
	0xe3, 0x35, 0xcb, 0x70, 0xaf, 0x36, 0x5c, 0xde, 0xbf, 0x10, 0x37, 0x04, 0xdd, 0x12, 0x44, 0x07,
	0xd0, 0x9e, 0x3a, 0x88, 0x2e, 0xd5, 0xba, 0xd1, 0xcb, 0x3e, 0x77, 0x73, 0xb6, 0x1c, 0xac, 0xa5,
	0xdc, 0x08, 0x68, 0xc5, 0xc5, 0x6c, 0xdc, 0x10, 0xe4, 0x45, 0xd0, 0x2a, 0xfe, 0x5b, 0x49, 0x5f,
	0x83, 0x51, 0x3c, 0xd0, 0x3b, 0xff, 0x2c, 0xc1, 0xac, 0xfe, 0xfa, 0x94, 0xdf, 0xce, 0xea, 0x53,
	0x7c, 0x88, 0x5e, 0x6a, 0xd9, 0xf0, 0x07, 0x8c, 0x6e, 0x30, 0xba, 0xc5, 0xe8, 0x33, 0x46, 0xdf,
	0x31, 0xfa, 0x85, 0xd1, 0x57, 0x82, 0xce, 0xf4, 0xfc, 0xc8, 0xcd, 0xdf, 0x01, 0x00, 0x00, 0xff,
	0xff, 0xf0, 0xc6, 0xff, 0xa7, 0x12, 0x05, 0x00, 0x00,
}
