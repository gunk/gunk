// Code generated by protoc-gen-go. DO NOT EDIT.
// source: testdata.tld/util/imported/imp.gunk

package imported

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

// Message is a Echo message.
type Message struct {
	// Msg holds a message.
	Msg                  string   `protobuf:"bytes,0,,name=Msg,proto3" json:"Msg,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Message) Reset()         { *m = Message{} }
func (m *Message) String() string { return proto.CompactTextString(m) }
func (*Message) ProtoMessage()    {}
func (*Message) Descriptor() ([]byte, []int) {
	return fileDescriptor_imp_cbdff67a767f5050, []int{0}
}
func (m *Message) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Message.Unmarshal(m, b)
}
func (m *Message) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Message.Marshal(b, m, deterministic)
}
func (dst *Message) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Message.Merge(dst, src)
}
func (m *Message) XXX_Size() int {
	return xxx_messageInfo_Message.Size(m)
}
func (m *Message) XXX_DiscardUnknown() {
	xxx_messageInfo_Message.DiscardUnknown(m)
}

var xxx_messageInfo_Message proto.InternalMessageInfo

func (m *Message) GetMsg() string {
	if m != nil {
		return m.Msg
	}
	return ""
}

func init() {
	proto.RegisterType((*Message)(nil), "testdata.tld/util/imported.Message")
}

func init() {
	proto.RegisterFile("testdata.tld/util/imported/imp.gunk", fileDescriptor_imp_cbdff67a767f5050)
}

var fileDescriptor_imp_cbdff67a767f5050 = []byte{
	// 101 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x52, 0x2e, 0x49, 0x2d, 0x2e,
	0x49, 0x49, 0x2c, 0x49, 0xd4, 0x2b, 0xc9, 0x49, 0xd1, 0x2f, 0x2d, 0xc9, 0xcc, 0xd1, 0xcf, 0xcc,
	0x2d, 0xc8, 0x2f, 0x2a, 0x49, 0x4d, 0x01, 0x31, 0xf4, 0xd2, 0x4b, 0xf3, 0xb2, 0x85, 0xa4, 0x70,
	0x2b, 0x52, 0x12, 0xe3, 0x62, 0xf7, 0x4d, 0x2d, 0x2e, 0x4e, 0x4c, 0x4f, 0x15, 0xe2, 0xe6, 0x62,
	0xf6, 0x2d, 0x4e, 0x97, 0x60, 0x50, 0x60, 0xd0, 0xe0, 0x74, 0xe2, 0x8a, 0xe2, 0x80, 0xa9, 0x49,
	0x62, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x37, 0x06, 0x04, 0x00, 0x00, 0xff, 0xff, 0x05, 0xde, 0x34,
	0xa3, 0x6d, 0x00, 0x00, 0x00,
}
