// Code generated by protoc-gen-go.
// source: google.golang.org/genproto/googleapis/api/serviceconfig/control.proto
// DO NOT EDIT!

package google_api

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// Selects and configures the service controller used by the service.  The
// service controller handles features like abuse, quota, billing, logging,
// monitoring, etc.
type Control struct {
	// The service control environment to use. If empty, no control plane
	// feature (like quota and billing) will be enabled.
	Environment string `protobuf:"bytes,1,opt,name=environment" json:"environment,omitempty"`
}

func (m *Control) Reset()                    { *m = Control{} }
func (m *Control) String() string            { return proto.CompactTextString(m) }
func (*Control) ProtoMessage()               {}
func (*Control) Descriptor() ([]byte, []int) { return fileDescriptor6, []int{0} }

func init() {
	proto.RegisterType((*Control)(nil), "google.api.Control")
}

func init() {
	proto.RegisterFile("google.golang.org/genproto/googleapis/api/serviceconfig/control.proto", fileDescriptor6)
}

var fileDescriptor6 = []byte{
	// 157 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0xe2, 0x72, 0x4d, 0xcf, 0xcf, 0x4f,
	0xcf, 0x49, 0xd5, 0x4b, 0xcf, 0xcf, 0x49, 0xcc, 0x4b, 0xd7, 0xcb, 0x2f, 0x4a, 0xd7, 0x4f, 0x4f,
	0xcd, 0x2b, 0x28, 0xca, 0x2f, 0xc9, 0xd7, 0x87, 0x48, 0x25, 0x16, 0x64, 0x16, 0xeb, 0x27, 0x16,
	0x64, 0xea, 0x17, 0xa7, 0x16, 0x95, 0x65, 0x26, 0xa7, 0x26, 0xe7, 0xe7, 0xa5, 0x65, 0xa6, 0xeb,
	0x27, 0xe7, 0xe7, 0x95, 0x14, 0xe5, 0xe7, 0xe8, 0x81, 0x95, 0x0a, 0x71, 0x41, 0x8d, 0x49, 0x2c,
	0xc8, 0x54, 0xd2, 0xe6, 0x62, 0x77, 0x86, 0x48, 0x0a, 0x29, 0x70, 0x71, 0xa7, 0xe6, 0x95, 0x65,
	0x16, 0xe5, 0xe7, 0xe5, 0xa6, 0xe6, 0x95, 0x48, 0x30, 0x2a, 0x30, 0x6a, 0x70, 0x06, 0x21, 0x0b,
	0x39, 0xa9, 0x73, 0xf1, 0x25, 0xe7, 0xe7, 0xea, 0x21, 0xb4, 0x3b, 0xf1, 0x40, 0x35, 0x07, 0x80,
	0x0c, 0x0e, 0x60, 0x5c, 0xc4, 0xc4, 0xe2, 0xee, 0x18, 0xe0, 0x99, 0xc4, 0x06, 0xb6, 0xc8, 0x18,
	0x10, 0x00, 0x00, 0xff, 0xff, 0x77, 0x67, 0xbb, 0x6f, 0xb1, 0x00, 0x00, 0x00,
}
