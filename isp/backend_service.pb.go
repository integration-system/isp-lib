// Code generated by protoc-gen-go. DO NOT EDIT.
// source: backend_service.proto

/*
Package isp is a generated protocol buffer package.

It is generated from these files:
	backend_service.proto

It has these top-level messages:
	Message
*/
package isp

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import google_protobuf "github.com/golang/protobuf/ptypes/struct"

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

type Message struct {
	// Types that are valid to be assigned to Body:
	//	*Message_StructBody
	//	*Message_ListBody
	//	*Message_NullBody
	//	*Message_BytesBody
	Body isMessage_Body `protobuf_oneof:"body"`
}

func (m *Message) Reset()                    { *m = Message{} }
func (m *Message) String() string            { return proto.CompactTextString(m) }
func (*Message) ProtoMessage()               {}
func (*Message) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

type isMessage_Body interface {
	isMessage_Body()
}

type Message_StructBody struct {
	StructBody *google_protobuf.Struct `protobuf:"bytes,1,opt,name=structBody,oneof"`
}
type Message_ListBody struct {
	ListBody *google_protobuf.ListValue `protobuf:"bytes,2,opt,name=listBody,oneof"`
}
type Message_NullBody struct {
	NullBody google_protobuf.NullValue `protobuf:"varint,3,opt,name=NullBody,enum=google.protobuf.NullValue,oneof"`
}
type Message_BytesBody struct {
	BytesBody []byte `protobuf:"bytes,4,opt,name=BytesBody,proto3,oneof"`
}

func (*Message_StructBody) isMessage_Body() {}
func (*Message_ListBody) isMessage_Body()   {}
func (*Message_NullBody) isMessage_Body()   {}
func (*Message_BytesBody) isMessage_Body()  {}

func (m *Message) GetBody() isMessage_Body {
	if m != nil {
		return m.Body
	}
	return nil
}

func (m *Message) GetStructBody() *google_protobuf.Struct {
	if x, ok := m.GetBody().(*Message_StructBody); ok {
		return x.StructBody
	}
	return nil
}

func (m *Message) GetListBody() *google_protobuf.ListValue {
	if x, ok := m.GetBody().(*Message_ListBody); ok {
		return x.ListBody
	}
	return nil
}

func (m *Message) GetNullBody() google_protobuf.NullValue {
	if x, ok := m.GetBody().(*Message_NullBody); ok {
		return x.NullBody
	}
	return google_protobuf.NullValue_NULL_VALUE
}

func (m *Message) GetBytesBody() []byte {
	if x, ok := m.GetBody().(*Message_BytesBody); ok {
		return x.BytesBody
	}
	return nil
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*Message) XXX_OneofFuncs() (func(msg proto.Message, b *proto.Buffer) error, func(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error), func(msg proto.Message) (n int), []interface{}) {
	return _Message_OneofMarshaler, _Message_OneofUnmarshaler, _Message_OneofSizer, []interface{}{
		(*Message_StructBody)(nil),
		(*Message_ListBody)(nil),
		(*Message_NullBody)(nil),
		(*Message_BytesBody)(nil),
	}
}

func _Message_OneofMarshaler(msg proto.Message, b *proto.Buffer) error {
	m := msg.(*Message)
	// body
	switch x := m.Body.(type) {
	case *Message_StructBody:
		b.EncodeVarint(1<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.StructBody); err != nil {
			return err
		}
	case *Message_ListBody:
		b.EncodeVarint(2<<3 | proto.WireBytes)
		if err := b.EncodeMessage(x.ListBody); err != nil {
			return err
		}
	case *Message_NullBody:
		b.EncodeVarint(3<<3 | proto.WireVarint)
		b.EncodeVarint(uint64(x.NullBody))
	case *Message_BytesBody:
		b.EncodeVarint(4<<3 | proto.WireBytes)
		b.EncodeRawBytes(x.BytesBody)
	case nil:
	default:
		return fmt.Errorf("Message.Body has unexpected type %T", x)
	}
	return nil
}

func _Message_OneofUnmarshaler(msg proto.Message, tag, wire int, b *proto.Buffer) (bool, error) {
	m := msg.(*Message)
	switch tag {
	case 1: // body.structBody
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(google_protobuf.Struct)
		err := b.DecodeMessage(msg)
		m.Body = &Message_StructBody{msg}
		return true, err
	case 2: // body.listBody
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		msg := new(google_protobuf.ListValue)
		err := b.DecodeMessage(msg)
		m.Body = &Message_ListBody{msg}
		return true, err
	case 3: // body.NullBody
		if wire != proto.WireVarint {
			return true, proto.ErrInternalBadWireType
		}
		x, err := b.DecodeVarint()
		m.Body = &Message_NullBody{google_protobuf.NullValue(x)}
		return true, err
	case 4: // body.BytesBody
		if wire != proto.WireBytes {
			return true, proto.ErrInternalBadWireType
		}
		x, err := b.DecodeRawBytes(true)
		m.Body = &Message_BytesBody{x}
		return true, err
	default:
		return false, nil
	}
}

func _Message_OneofSizer(msg proto.Message) (n int) {
	m := msg.(*Message)
	// body
	switch x := m.Body.(type) {
	case *Message_StructBody:
		s := proto.Size(x.StructBody)
		n += proto.SizeVarint(1<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(s))
		n += s
	case *Message_ListBody:
		s := proto.Size(x.ListBody)
		n += proto.SizeVarint(2<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(s))
		n += s
	case *Message_NullBody:
		n += proto.SizeVarint(3<<3 | proto.WireVarint)
		n += proto.SizeVarint(uint64(x.NullBody))
	case *Message_BytesBody:
		n += proto.SizeVarint(4<<3 | proto.WireBytes)
		n += proto.SizeVarint(uint64(len(x.BytesBody)))
		n += len(x.BytesBody)
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

func init() {
	proto.RegisterType((*Message)(nil), "isp.Message")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for BackendService service

type BackendServiceClient interface {
	// ===== SYSTEM =====
	Request(ctx context.Context, in *Message, opts ...grpc.CallOption) (*Message, error)
	RequestStream(ctx context.Context, opts ...grpc.CallOption) (BackendService_RequestStreamClient, error)
}

type backendServiceClient struct {
	cc *grpc.ClientConn
}

func NewBackendServiceClient(cc *grpc.ClientConn) BackendServiceClient {
	return &backendServiceClient{cc}
}

func (c *backendServiceClient) Request(ctx context.Context, in *Message, opts ...grpc.CallOption) (*Message, error) {
	out := new(Message)
	err := grpc.Invoke(ctx, "/isp.BackendService/Request", in, out, c.cc, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *backendServiceClient) RequestStream(ctx context.Context, opts ...grpc.CallOption) (BackendService_RequestStreamClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_BackendService_serviceDesc.Streams[0], c.cc, "/isp.BackendService/RequestStream", opts...)
	if err != nil {
		return nil, err
	}
	x := &backendServiceRequestStreamClient{stream}
	return x, nil
}

type BackendService_RequestStreamClient interface {
	Send(*Message) error
	Recv() (*Message, error)
	grpc.ClientStream
}

type backendServiceRequestStreamClient struct {
	grpc.ClientStream
}

func (x *backendServiceRequestStreamClient) Send(m *Message) error {
	return x.ClientStream.SendMsg(m)
}

func (x *backendServiceRequestStreamClient) Recv() (*Message, error) {
	m := new(Message)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Server API for BackendService service

type BackendServiceServer interface {
	// ===== SYSTEM =====
	Request(context.Context, *Message) (*Message, error)
	RequestStream(BackendService_RequestStreamServer) error
}

func RegisterBackendServiceServer(s *grpc.Server, srv BackendServiceServer) {
	s.RegisterService(&_BackendService_serviceDesc, srv)
}

func _BackendService_Request_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Message)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BackendServiceServer).Request(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/isp.BackendService/Request",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BackendServiceServer).Request(ctx, req.(*Message))
	}
	return interceptor(ctx, in, info, handler)
}

func _BackendService_RequestStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(BackendServiceServer).RequestStream(&backendServiceRequestStreamServer{stream})
}

type BackendService_RequestStreamServer interface {
	Send(*Message) error
	Recv() (*Message, error)
	grpc.ServerStream
}

type backendServiceRequestStreamServer struct {
	grpc.ServerStream
}

func (x *backendServiceRequestStreamServer) Send(m *Message) error {
	return x.ServerStream.SendMsg(m)
}

func (x *backendServiceRequestStreamServer) Recv() (*Message, error) {
	m := new(Message)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _BackendService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "isp.BackendService",
	HandlerType: (*BackendServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Request",
			Handler:    _BackendService_Request_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "RequestStream",
			Handler:       _BackendService_RequestStream_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "backend_service.proto",
}

func init() { proto.RegisterFile("backend_service.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 280 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x90, 0x4f, 0x4b, 0xc3, 0x30,
	0x18, 0xc6, 0x1b, 0x37, 0x36, 0x7d, 0x9d, 0x3b, 0x44, 0xfc, 0xc3, 0x10, 0x19, 0xbb, 0xd8, 0x53,
	0xa6, 0xf3, 0xa2, 0xd7, 0x78, 0xe9, 0x41, 0x65, 0xb4, 0xe0, 0x55, 0xfa, 0xe7, 0xb5, 0x14, 0xd3,
	0xa5, 0x26, 0xe9, 0xa0, 0xdf, 0xd6, 0x8f, 0x22, 0x4d, 0xba, 0xe9, 0x50, 0x3c, 0xbe, 0x4f, 0x7e,
	0x3f, 0xc2, 0xf3, 0xc0, 0x49, 0x12, 0xa7, 0xef, 0xb8, 0xca, 0x5e, 0x35, 0xaa, 0x75, 0x91, 0x22,
	0xab, 0x94, 0x34, 0x92, 0xf6, 0x4a, 0x5d, 0x4d, 0x2e, 0x72, 0x29, 0x73, 0x81, 0x73, 0x1b, 0x25,
	0xf5, 0xdb, 0x5c, 0x1b, 0x55, 0xa7, 0xc6, 0x21, 0xb3, 0x4f, 0x02, 0xc3, 0x27, 0xd4, 0x3a, 0xce,
	0x91, 0xde, 0x03, 0xb8, 0x37, 0x2e, 0xb3, 0xe6, 0x9c, 0x4c, 0x89, 0x7f, 0xb8, 0x38, 0x63, 0x4e,
	0x67, 0x1b, 0x9d, 0x45, 0x16, 0x09, 0xbc, 0xf0, 0x07, 0x4c, 0xef, 0x60, 0x5f, 0x14, 0xda, 0x89,
	0x7b, 0x56, 0x9c, 0xfc, 0x12, 0x1f, 0x0b, 0x6d, 0x5e, 0x62, 0x51, 0x63, 0xe0, 0x85, 0x5b, 0xba,
	0x35, 0x9f, 0x6b, 0x21, 0xac, 0xd9, 0x9b, 0x12, 0x7f, 0xfc, 0x87, 0xd9, 0x02, 0x5b, 0x73, 0x43,
	0xd3, 0x4b, 0x38, 0xe0, 0x8d, 0x41, 0x6d, 0xd5, 0xfe, 0x94, 0xf8, 0xa3, 0xc0, 0x0b, 0xbf, 0x23,
	0x3e, 0x80, 0x7e, 0x22, 0xb3, 0x66, 0x21, 0x60, 0xcc, 0xdd, 0x3c, 0x91, 0x5b, 0x87, 0x5e, 0xc1,
	0x30, 0xc4, 0x8f, 0x1a, 0xb5, 0xa1, 0x23, 0x56, 0xea, 0x8a, 0x75, 0x0b, 0x4c, 0x76, 0xae, 0x99,
	0x47, 0x6f, 0xe0, 0xa8, 0x03, 0x23, 0xa3, 0x30, 0x2e, 0xff, 0xc7, 0x7d, 0x72, 0x4d, 0xf8, 0x1c,
	0x4e, 0x0b, 0xc9, 0x72, 0x55, 0xa5, 0x2c, 0x95, 0xab, 0x35, 0x2a, 0xd3, 0xfd, 0xca, 0x8f, 0x1f,
	0x76, 0xee, 0x65, 0xdb, 0x6e, 0x49, 0x92, 0x81, 0xad, 0x79, 0xfb, 0x15, 0x00, 0x00, 0xff, 0xff,
	0x6b, 0x2b, 0x19, 0x3e, 0xc4, 0x01, 0x00, 0x00,
}