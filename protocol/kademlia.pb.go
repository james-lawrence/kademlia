// Code generated by protoc-gen-go. DO NOT EDIT.
// source: kademlia.proto

/*
Package protocol is a generated protocol buffer package.

It is generated from these files:
	kademlia.proto

It has these top-level messages:
	Node
	PingRequest
	PingResponse
	ProbeRequest
	ProbeResponse
*/
package protocol

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

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

type Node struct {
	// ID is a 20 byte unique identifier
	ID []byte `protobuf:"bytes,1,opt,name=ID,proto3" json:"ID,omitempty"`
	// IP is the public address IP of the node
	IP string `protobuf:"bytes,2,opt,name=IP" json:"IP,omitempty"`
	// Port is the public port of the node
	Port int32 `protobuf:"varint,3,opt,name=Port" json:"Port,omitempty"`
}

func (m *Node) Reset()                    { *m = Node{} }
func (m *Node) String() string            { return proto.CompactTextString(m) }
func (*Node) ProtoMessage()               {}
func (*Node) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *Node) GetID() []byte {
	if m != nil {
		return m.ID
	}
	return nil
}

func (m *Node) GetIP() string {
	if m != nil {
		return m.IP
	}
	return ""
}

func (m *Node) GetPort() int32 {
	if m != nil {
		return m.Port
	}
	return 0
}

type PingRequest struct {
	Sender   *Node `protobuf:"bytes,1,opt,name=Sender" json:"Sender,omitempty"`
	Receiver *Node `protobuf:"bytes,2,opt,name=Receiver" json:"Receiver,omitempty"`
}

func (m *PingRequest) Reset()                    { *m = PingRequest{} }
func (m *PingRequest) String() string            { return proto.CompactTextString(m) }
func (*PingRequest) ProtoMessage()               {}
func (*PingRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *PingRequest) GetSender() *Node {
	if m != nil {
		return m.Sender
	}
	return nil
}

func (m *PingRequest) GetReceiver() *Node {
	if m != nil {
		return m.Receiver
	}
	return nil
}

type PingResponse struct {
	Sender   *Node `protobuf:"bytes,1,opt,name=Sender" json:"Sender,omitempty"`
	Receiver *Node `protobuf:"bytes,2,opt,name=Receiver" json:"Receiver,omitempty"`
}

func (m *PingResponse) Reset()                    { *m = PingResponse{} }
func (m *PingResponse) String() string            { return proto.CompactTextString(m) }
func (*PingResponse) ProtoMessage()               {}
func (*PingResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *PingResponse) GetSender() *Node {
	if m != nil {
		return m.Sender
	}
	return nil
}

func (m *PingResponse) GetReceiver() *Node {
	if m != nil {
		return m.Receiver
	}
	return nil
}

type ProbeRequest struct {
	Sender   *Node  `protobuf:"bytes,1,opt,name=Sender" json:"Sender,omitempty"`
	Receiver *Node  `protobuf:"bytes,2,opt,name=Receiver" json:"Receiver,omitempty"`
	Key      []byte `protobuf:"bytes,3,opt,name=Key,proto3" json:"Key,omitempty"`
}

func (m *ProbeRequest) Reset()                    { *m = ProbeRequest{} }
func (m *ProbeRequest) String() string            { return proto.CompactTextString(m) }
func (*ProbeRequest) ProtoMessage()               {}
func (*ProbeRequest) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

func (m *ProbeRequest) GetSender() *Node {
	if m != nil {
		return m.Sender
	}
	return nil
}

func (m *ProbeRequest) GetReceiver() *Node {
	if m != nil {
		return m.Receiver
	}
	return nil
}

func (m *ProbeRequest) GetKey() []byte {
	if m != nil {
		return m.Key
	}
	return nil
}

type ProbeResponse struct {
	Sender   *Node   `protobuf:"bytes,1,opt,name=Sender" json:"Sender,omitempty"`
	Receiver *Node   `protobuf:"bytes,2,opt,name=Receiver" json:"Receiver,omitempty"`
	Key      []byte  `protobuf:"bytes,3,opt,name=Key,proto3" json:"Key,omitempty"`
	Nearest  []*Node `protobuf:"bytes,4,rep,name=nearest" json:"nearest,omitempty"`
}

func (m *ProbeResponse) Reset()                    { *m = ProbeResponse{} }
func (m *ProbeResponse) String() string            { return proto.CompactTextString(m) }
func (*ProbeResponse) ProtoMessage()               {}
func (*ProbeResponse) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func (m *ProbeResponse) GetSender() *Node {
	if m != nil {
		return m.Sender
	}
	return nil
}

func (m *ProbeResponse) GetReceiver() *Node {
	if m != nil {
		return m.Receiver
	}
	return nil
}

func (m *ProbeResponse) GetKey() []byte {
	if m != nil {
		return m.Key
	}
	return nil
}

func (m *ProbeResponse) GetNearest() []*Node {
	if m != nil {
		return m.Nearest
	}
	return nil
}

func init() {
	proto.RegisterType((*Node)(nil), "protocol.Node")
	proto.RegisterType((*PingRequest)(nil), "protocol.PingRequest")
	proto.RegisterType((*PingResponse)(nil), "protocol.PingResponse")
	proto.RegisterType((*ProbeRequest)(nil), "protocol.ProbeRequest")
	proto.RegisterType((*ProbeResponse)(nil), "protocol.ProbeResponse")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for Kademlia service

type KademliaClient interface {
	Ping(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error)
	Probe(ctx context.Context, in *ProbeRequest, opts ...grpc.CallOption) (*ProbeResponse, error)
}

type kademliaClient struct {
	cc *grpc.ClientConn
}

func NewKademliaClient(cc *grpc.ClientConn) KademliaClient {
	return &kademliaClient{cc}
}

func (c *kademliaClient) Ping(ctx context.Context, in *PingRequest, opts ...grpc.CallOption) (*PingResponse, error) {
	out := new(PingResponse)
	err := grpc.Invoke(ctx, "/protocol.kademlia/Ping", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *kademliaClient) Probe(ctx context.Context, in *ProbeRequest, opts ...grpc.CallOption) (*ProbeResponse, error) {
	out := new(ProbeResponse)
	err := grpc.Invoke(ctx, "/protocol.kademlia/Probe", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for Kademlia service

type KademliaServer interface {
	Ping(context.Context, *PingRequest) (*PingResponse, error)
	Probe(context.Context, *ProbeRequest) (*ProbeResponse, error)
}

func RegisterKademliaServer(s *grpc.Server, srv KademliaServer) {
	s.RegisterService(&_Kademlia_serviceDesc, srv)
}

func _Kademlia_Ping_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KademliaServer).Ping(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/protocol.kademlia/Ping",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KademliaServer).Ping(ctx, req.(*PingRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Kademlia_Probe_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProbeRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(KademliaServer).Probe(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/protocol.kademlia/Probe",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(KademliaServer).Probe(ctx, req.(*ProbeRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _Kademlia_serviceDesc = grpc.ServiceDesc{
	ServiceName: "protocol.kademlia",
	HandlerType: (*KademliaServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Ping",
			Handler:    _Kademlia_Ping_Handler,
		},
		{
			MethodName: "Probe",
			Handler:    _Kademlia_Probe_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "kademlia.proto",
}

func init() { proto.RegisterFile("kademlia.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 274 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xb4, 0x92, 0xc1, 0x4e, 0x83, 0x40,
	0x10, 0x86, 0x5d, 0xa0, 0x15, 0xa7, 0x48, 0xcc, 0x24, 0x2a, 0xe9, 0x89, 0xec, 0xc1, 0x10, 0x0f,
	0x1c, 0xea, 0xc1, 0xa4, 0xe7, 0x5e, 0x88, 0x89, 0xd9, 0xac, 0x4f, 0x00, 0x65, 0x62, 0x88, 0x95,
	0xad, 0xcb, 0x6a, 0xe2, 0xc9, 0x47, 0xf1, 0x55, 0x0d, 0x5b, 0x50, 0x22, 0xbd, 0x72, 0xda, 0xd9,
	0xf9, 0x67, 0xf6, 0xcb, 0x3f, 0xb3, 0x10, 0xbe, 0xe4, 0x25, 0xbd, 0xee, 0xaa, 0x3c, 0xdd, 0x6b,
	0x65, 0x14, 0xfa, 0xf6, 0xd8, 0xaa, 0x1d, 0x5f, 0x83, 0xf7, 0xa8, 0x4a, 0xc2, 0x10, 0x9c, 0x6c,
	0x13, 0xb1, 0x98, 0x25, 0x81, 0x74, 0xb2, 0x8d, 0xbd, 0x8b, 0xc8, 0x89, 0x59, 0x72, 0x26, 0x9d,
	0x4c, 0x20, 0x82, 0x27, 0x94, 0x36, 0x91, 0x1b, 0xb3, 0x64, 0x26, 0x6d, 0xcc, 0x73, 0x58, 0x88,
	0xaa, 0x7e, 0x96, 0xf4, 0xf6, 0x4e, 0x8d, 0xc1, 0x1b, 0x98, 0x3f, 0x51, 0x5d, 0x92, 0xb6, 0xcf,
	0x2c, 0x56, 0x61, 0xda, 0x53, 0xd2, 0x16, 0x21, 0x3b, 0x15, 0x6f, 0xc1, 0x97, 0xb4, 0xa5, 0xea,
	0x83, 0xb4, 0x05, 0x8c, 0x2b, 0x7f, 0x75, 0x5e, 0x40, 0x70, 0x40, 0x34, 0x7b, 0x55, 0x37, 0x34,
	0x09, 0xc3, 0x40, 0x20, 0xb4, 0x2a, 0x68, 0x42, 0x1f, 0x78, 0x01, 0xee, 0x03, 0x7d, 0xda, 0xe9,
	0x05, 0xb2, 0x0d, 0xf9, 0x37, 0x83, 0xf3, 0x0e, 0x3b, 0x9d, 0xb7, 0x31, 0x17, 0x13, 0x38, 0xad,
	0x29, 0xd7, 0xd4, 0x98, 0xc8, 0x8b, 0xdd, 0x23, 0xcd, 0xbd, 0xbc, 0xfa, 0x02, 0xbf, 0xff, 0x36,
	0x78, 0x0f, 0x5e, 0xbb, 0x07, 0xbc, 0xfc, 0x2b, 0x1e, 0xac, 0x7e, 0x79, 0xf5, 0x3f, 0x7d, 0xb0,
	0xc4, 0x4f, 0x70, 0x0d, 0x33, 0xeb, 0x12, 0x87, 0x25, 0x83, 0x69, 0x2f, 0xaf, 0x47, 0xf9, 0xbe,
	0xb7, 0x98, 0x5b, 0xe5, 0xee, 0x27, 0x00, 0x00, 0xff, 0xff, 0x57, 0x55, 0xd5, 0xde, 0xbe, 0x02,
	0x00, 0x00,
}