package remote

import (
	bytes "bytes"
	context "context"
	fmt "fmt"
	actor "github.com/AsynkronIT/protoactor-go/actor"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
	github_com_gogo_protobuf_sortkeys "github.com/gogo/protobuf/sortkeys"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	io "io"
	math "math"
	math_bits "math/bits"
	reflect "reflect"
	strings "strings"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

type MessageBatch struct {
	TypeNames   []string           `protobuf:"bytes,1,rep,name=type_names,json=typeNames,proto3" json:"type_names,omitempty"`
	TargetNames []string           `protobuf:"bytes,2,rep,name=target_names,json=targetNames,proto3" json:"target_names,omitempty"`
	Envelopes   []*MessageEnvelope `protobuf:"bytes,3,rep,name=envelopes,proto3" json:"envelopes,omitempty"`
}

func (m *MessageBatch) Reset()      { *m = MessageBatch{} }
func (*MessageBatch) ProtoMessage() {}
func (*MessageBatch) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{0}
}
func (m *MessageBatch) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *MessageBatch) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_MessageBatch.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *MessageBatch) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MessageBatch.Merge(m, src)
}
func (m *MessageBatch) XXX_Size() int {
	return m.Size()
}
func (m *MessageBatch) XXX_DiscardUnknown() {
	xxx_messageInfo_MessageBatch.DiscardUnknown(m)
}

var xxx_messageInfo_MessageBatch proto.InternalMessageInfo

func (m *MessageBatch) GetTypeNames() []string {
	if m != nil {
		return m.TypeNames
	}
	return nil
}

func (m *MessageBatch) GetTargetNames() []string {
	if m != nil {
		return m.TargetNames
	}
	return nil
}

func (m *MessageBatch) GetEnvelopes() []*MessageEnvelope {
	if m != nil {
		return m.Envelopes
	}
	return nil
}

type MessageEnvelope struct {
	TypeId        int32          `protobuf:"varint,1,opt,name=type_id,json=typeId,proto3" json:"type_id,omitempty"`
	MessageData   []byte         `protobuf:"bytes,2,opt,name=message_data,json=messageData,proto3" json:"message_data,omitempty"`
	Target        int32          `protobuf:"varint,3,opt,name=target,proto3" json:"target,omitempty"`
	Sender        *actor.PID     `protobuf:"bytes,4,opt,name=sender,proto3" json:"sender,omitempty"`
	SerializerId  int32          `protobuf:"varint,5,opt,name=serializer_id,json=serializerId,proto3" json:"serializer_id,omitempty"`
	MessageHeader *MessageHeader `protobuf:"bytes,6,opt,name=message_header,json=messageHeader,proto3" json:"message_header,omitempty"`
}

func (m *MessageEnvelope) Reset()      { *m = MessageEnvelope{} }
func (*MessageEnvelope) ProtoMessage() {}
func (*MessageEnvelope) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{1}
}
func (m *MessageEnvelope) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *MessageEnvelope) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_MessageEnvelope.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *MessageEnvelope) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MessageEnvelope.Merge(m, src)
}
func (m *MessageEnvelope) XXX_Size() int {
	return m.Size()
}
func (m *MessageEnvelope) XXX_DiscardUnknown() {
	xxx_messageInfo_MessageEnvelope.DiscardUnknown(m)
}

var xxx_messageInfo_MessageEnvelope proto.InternalMessageInfo

func (m *MessageEnvelope) GetTypeId() int32 {
	if m != nil {
		return m.TypeId
	}
	return 0
}

func (m *MessageEnvelope) GetMessageData() []byte {
	if m != nil {
		return m.MessageData
	}
	return nil
}

func (m *MessageEnvelope) GetTarget() int32 {
	if m != nil {
		return m.Target
	}
	return 0
}

func (m *MessageEnvelope) GetSender() *actor.PID {
	if m != nil {
		return m.Sender
	}
	return nil
}

func (m *MessageEnvelope) GetSerializerId() int32 {
	if m != nil {
		return m.SerializerId
	}
	return 0
}

func (m *MessageEnvelope) GetMessageHeader() *MessageHeader {
	if m != nil {
		return m.MessageHeader
	}
	return nil
}

type MessageHeader struct {
	HeaderData map[string]string `protobuf:"bytes,1,rep,name=header_data,json=headerData,proto3" json:"header_data,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (m *MessageHeader) Reset()      { *m = MessageHeader{} }
func (*MessageHeader) ProtoMessage() {}
func (*MessageHeader) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{2}
}
func (m *MessageHeader) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *MessageHeader) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_MessageHeader.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *MessageHeader) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MessageHeader.Merge(m, src)
}
func (m *MessageHeader) XXX_Size() int {
	return m.Size()
}
func (m *MessageHeader) XXX_DiscardUnknown() {
	xxx_messageInfo_MessageHeader.DiscardUnknown(m)
}

var xxx_messageInfo_MessageHeader proto.InternalMessageInfo

func (m *MessageHeader) GetHeaderData() map[string]string {
	if m != nil {
		return m.HeaderData
	}
	return nil
}

type ActorPidRequest struct {
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Kind string `protobuf:"bytes,2,opt,name=kind,proto3" json:"kind,omitempty"`
}

func (m *ActorPidRequest) Reset()      { *m = ActorPidRequest{} }
func (*ActorPidRequest) ProtoMessage() {}
func (*ActorPidRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{3}
}
func (m *ActorPidRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ActorPidRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ActorPidRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ActorPidRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ActorPidRequest.Merge(m, src)
}
func (m *ActorPidRequest) XXX_Size() int {
	return m.Size()
}
func (m *ActorPidRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ActorPidRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ActorPidRequest proto.InternalMessageInfo

func (m *ActorPidRequest) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *ActorPidRequest) GetKind() string {
	if m != nil {
		return m.Kind
	}
	return ""
}

type ActorPidResponse struct {
	Pid        *actor.PID `protobuf:"bytes,1,opt,name=pid,proto3" json:"pid,omitempty"`
	StatusCode int32      `protobuf:"varint,2,opt,name=status_code,json=statusCode,proto3" json:"status_code,omitempty"`
}

func (m *ActorPidResponse) Reset()      { *m = ActorPidResponse{} }
func (*ActorPidResponse) ProtoMessage() {}
func (*ActorPidResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{4}
}
func (m *ActorPidResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ActorPidResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ActorPidResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ActorPidResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ActorPidResponse.Merge(m, src)
}
func (m *ActorPidResponse) XXX_Size() int {
	return m.Size()
}
func (m *ActorPidResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ActorPidResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ActorPidResponse proto.InternalMessageInfo

func (m *ActorPidResponse) GetPid() *actor.PID {
	if m != nil {
		return m.Pid
	}
	return nil
}

func (m *ActorPidResponse) GetStatusCode() int32 {
	if m != nil {
		return m.StatusCode
	}
	return 0
}

type Unit struct {
}

func (m *Unit) Reset()      { *m = Unit{} }
func (*Unit) ProtoMessage() {}
func (*Unit) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{5}
}
func (m *Unit) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Unit) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Unit.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Unit) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Unit.Merge(m, src)
}
func (m *Unit) XXX_Size() int {
	return m.Size()
}
func (m *Unit) XXX_DiscardUnknown() {
	xxx_messageInfo_Unit.DiscardUnknown(m)
}

var xxx_messageInfo_Unit proto.InternalMessageInfo

type ConnectRequest struct {
}

func (m *ConnectRequest) Reset()      { *m = ConnectRequest{} }
func (*ConnectRequest) ProtoMessage() {}
func (*ConnectRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{6}
}
func (m *ConnectRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ConnectRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ConnectRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ConnectRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ConnectRequest.Merge(m, src)
}
func (m *ConnectRequest) XXX_Size() int {
	return m.Size()
}
func (m *ConnectRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ConnectRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ConnectRequest proto.InternalMessageInfo

type ConnectResponse struct {
	DefaultSerializerId int32 `protobuf:"varint,1,opt,name=default_serializer_id,json=defaultSerializerId,proto3" json:"default_serializer_id,omitempty"`
}

func (m *ConnectResponse) Reset()      { *m = ConnectResponse{} }
func (*ConnectResponse) ProtoMessage() {}
func (*ConnectResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{7}
}
func (m *ConnectResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ConnectResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ConnectResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ConnectResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ConnectResponse.Merge(m, src)
}
func (m *ConnectResponse) XXX_Size() int {
	return m.Size()
}
func (m *ConnectResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ConnectResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ConnectResponse proto.InternalMessageInfo

func (m *ConnectResponse) GetDefaultSerializerId() int32 {
	if m != nil {
		return m.DefaultSerializerId
	}
	return 0
}

func init() {
	proto.RegisterType((*MessageBatch)(nil), "remote.MessageBatch")
	proto.RegisterType((*MessageEnvelope)(nil), "remote.MessageEnvelope")
	proto.RegisterType((*MessageHeader)(nil), "remote.MessageHeader")
	proto.RegisterMapType((map[string]string)(nil), "remote.MessageHeader.HeaderDataEntry")
	proto.RegisterType((*ActorPidRequest)(nil), "remote.ActorPidRequest")
	proto.RegisterType((*ActorPidResponse)(nil), "remote.ActorPidResponse")
	proto.RegisterType((*Unit)(nil), "remote.Unit")
	proto.RegisterType((*ConnectRequest)(nil), "remote.ConnectRequest")
	proto.RegisterType((*ConnectResponse)(nil), "remote.ConnectResponse")
}

func init() { proto.RegisterFile("protos.proto", fileDescriptor_5da3cbeb884d181c) }

var fileDescriptor_5da3cbeb884d181c = []byte{
	// 625 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x53, 0x3d, 0x6f, 0x13, 0x4d,
	0x10, 0xbe, 0x8d, 0x63, 0xe7, 0xf5, 0xd8, 0x89, 0xa3, 0x7d, 0xf3, 0x61, 0x59, 0x70, 0x98, 0x43,
	0x48, 0x6e, 0x72, 0x46, 0x0e, 0x20, 0x40, 0xa1, 0xc8, 0x17, 0xc2, 0x05, 0x28, 0x1c, 0x50, 0x5b,
	0x1b, 0xdf, 0xe4, 0x7c, 0x8a, 0x7d, 0x6b, 0x6e, 0xd7, 0x91, 0x8c, 0x84, 0x44, 0x47, 0x4b, 0xc5,
	0x6f, 0xe0, 0xa7, 0x50, 0xa6, 0x4c, 0x49, 0x2e, 0x0d, 0x05, 0x45, 0x7e, 0x02, 0xda, 0x8f, 0x24,
	0xb6, 0xa1, 0xba, 0x99, 0x67, 0x9e, 0xf9, 0x7a, 0xe6, 0x16, 0xca, 0xc3, 0x94, 0x4b, 0x2e, 0x7c,
	0xfd, 0xa1, 0x85, 0x14, 0x07, 0x5c, 0x62, 0x6d, 0x23, 0x8a, 0x65, 0x6f, 0x74, 0xe8, 0x77, 0xf9,
	0xa0, 0x19, 0xf1, 0x88, 0x37, 0x75, 0xf8, 0x70, 0x74, 0xa4, 0x3d, 0xed, 0x68, 0xcb, 0xa4, 0xd5,
	0x1e, 0x4f, 0xd0, 0xb7, 0xc5, 0x38, 0x39, 0x4e, 0x79, 0xd2, 0x7e, 0x67, 0x92, 0x58, 0x57, 0xf2,
	0x74, 0x23, 0xe2, 0x4d, 0x6d, 0x34, 0x27, 0xdb, 0x79, 0x5f, 0x08, 0x94, 0x5f, 0xa1, 0x10, 0x2c,
	0xc2, 0x1d, 0x26, 0xbb, 0x3d, 0x7a, 0x1b, 0x40, 0x8e, 0x87, 0xd8, 0x49, 0xd8, 0x00, 0x45, 0x95,
	0xd4, 0x73, 0x8d, 0x62, 0x50, 0x54, 0xc8, 0x6b, 0x05, 0xd0, 0xbb, 0x50, 0x96, 0x2c, 0x8d, 0x50,
	0x5a, 0xc2, 0x9c, 0x26, 0x94, 0x0c, 0x66, 0x28, 0x8f, 0xa0, 0x88, 0xc9, 0x09, 0xf6, 0xf9, 0x10,
	0x45, 0x35, 0x57, 0xcf, 0x35, 0x4a, 0xad, 0x75, 0xdf, 0x6c, 0xe5, 0xdb, 0x56, 0xfb, 0x36, 0x1e,
	0xdc, 0x30, 0xbd, 0xdf, 0x04, 0x2a, 0x33, 0x61, 0xba, 0x0e, 0x0b, 0x7a, 0x98, 0x38, 0xac, 0x92,
	0x3a, 0x69, 0xe4, 0x83, 0x82, 0x72, 0xdb, 0xa1, 0x1a, 0x63, 0x60, 0xb8, 0x9d, 0x90, 0x49, 0x56,
	0x9d, 0xab, 0x93, 0x46, 0x39, 0x28, 0x59, 0x6c, 0x8f, 0x49, 0x46, 0xd7, 0xa0, 0x60, 0xa6, 0xaa,
	0xe6, 0x6c, 0xaa, 0xf6, 0xa8, 0x07, 0x05, 0x81, 0x49, 0x88, 0x69, 0x75, 0xbe, 0x4e, 0x1a, 0xa5,
	0x16, 0xf8, 0x5a, 0x16, 0xff, 0xa0, 0xbd, 0x17, 0xd8, 0x08, 0xbd, 0x07, 0x8b, 0x02, 0xd3, 0x98,
	0xf5, 0xe3, 0x8f, 0x98, 0xaa, 0xee, 0x79, 0x5d, 0xa2, 0x7c, 0x03, 0xb6, 0x43, 0xba, 0x05, 0x4b,
	0x57, 0x33, 0xf4, 0x90, 0xa9, 0x82, 0x05, 0x5d, 0x70, 0x75, 0x66, 0xd9, 0x97, 0x3a, 0x18, 0x2c,
	0x0e, 0x26, 0x5d, 0xef, 0x1b, 0x81, 0xc5, 0x29, 0x02, 0x7d, 0x01, 0x25, 0x53, 0xc7, 0xac, 0x44,
	0xb4, 0x72, 0xf7, 0xff, 0x59, 0xcc, 0x37, 0x1f, 0xb5, 0xe7, 0x7e, 0x22, 0xd3, 0x71, 0x00, 0xbd,
	0x6b, 0xa0, 0xf6, 0x1c, 0x2a, 0x33, 0x61, 0xba, 0x0c, 0xb9, 0x63, 0x1c, 0x6b, 0x0d, 0x8b, 0x81,
	0x32, 0xe9, 0x0a, 0xe4, 0x4f, 0x58, 0x7f, 0x84, 0x5a, 0xb9, 0x62, 0x60, 0x9c, 0x67, 0x73, 0x4f,
	0x88, 0xf7, 0x14, 0x2a, 0xdb, 0x4a, 0x90, 0x83, 0x38, 0x0c, 0xf0, 0xc3, 0x08, 0x85, 0xa4, 0x14,
	0xe6, 0xd5, 0xb5, 0x6d, 0xbe, 0xb6, 0x15, 0x76, 0x1c, 0x27, 0xa1, 0xcd, 0xd7, 0xb6, 0xf7, 0x06,
	0x96, 0x6f, 0x52, 0xc5, 0x90, 0x27, 0x02, 0xe9, 0x2d, 0xc8, 0x0d, 0xed, 0xf9, 0xa6, 0xb5, 0x56,
	0x30, 0xbd, 0x03, 0x25, 0x21, 0x99, 0x1c, 0x89, 0x4e, 0x97, 0x87, 0x66, 0x98, 0x7c, 0x00, 0x06,
	0xda, 0xe5, 0x21, 0x7a, 0x05, 0x98, 0x7f, 0x9f, 0xc4, 0xd2, 0x5b, 0x86, 0xa5, 0x5d, 0x9e, 0x24,
	0xd8, 0x95, 0x76, 0x28, 0x6f, 0x1f, 0x2a, 0xd7, 0x88, 0xed, 0xd5, 0x82, 0xd5, 0x10, 0x8f, 0xd8,
	0xa8, 0x2f, 0x3b, 0xd3, 0xe7, 0x33, 0x3f, 0xcf, 0xff, 0x36, 0xf8, 0x76, 0xe2, 0x8a, 0xad, 0x4f,
	0xf0, 0x5f, 0xa0, 0x14, 0x8e, 0x93, 0x88, 0x6e, 0xc1, 0x82, 0x2d, 0x49, 0xd7, 0xae, 0x74, 0x9f,
	0xee, 0x5a, 0x5b, 0xff, 0x0b, 0x37, 0xbd, 0x3d, 0x87, 0x6e, 0xc2, 0x42, 0x80, 0x5d, 0x8c, 0x4f,
	0x90, 0xae, 0xcc, 0x5c, 0x4d, 0x3f, 0xad, 0x5a, 0xf9, 0x0a, 0xd5, 0x1b, 0x39, 0x0d, 0xf2, 0x80,
	0xec, 0x3c, 0x3c, 0x3d, 0x77, 0x9d, 0xb3, 0x73, 0xd7, 0xb9, 0x3c, 0x77, 0x9d, 0xcf, 0x99, 0x4b,
	0xbe, 0x67, 0x2e, 0xf9, 0x91, 0xb9, 0xe4, 0x34, 0x73, 0xc9, 0xcf, 0xcc, 0x25, 0xbf, 0x32, 0xd7,
	0xb9, 0xcc, 0x5c, 0xf2, 0xf5, 0xc2, 0x75, 0x4e, 0x2f, 0x5c, 0xe7, 0xec, 0xc2, 0x75, 0x0e, 0x0b,
	0xfa, 0xf1, 0x6e, 0xfe, 0x09, 0x00, 0x00, 0xff, 0xff, 0xa0, 0x63, 0xc7, 0x3d, 0x3b, 0x04, 0x00,
	0x00,
}

func (this *MessageBatch) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*MessageBatch)
	if !ok {
		that2, ok := that.(MessageBatch)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if len(this.TypeNames) != len(that1.TypeNames) {
		return false
	}
	for i := range this.TypeNames {
		if this.TypeNames[i] != that1.TypeNames[i] {
			return false
		}
	}
	if len(this.TargetNames) != len(that1.TargetNames) {
		return false
	}
	for i := range this.TargetNames {
		if this.TargetNames[i] != that1.TargetNames[i] {
			return false
		}
	}
	if len(this.Envelopes) != len(that1.Envelopes) {
		return false
	}
	for i := range this.Envelopes {
		if !this.Envelopes[i].Equal(that1.Envelopes[i]) {
			return false
		}
	}
	return true
}
func (this *MessageEnvelope) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*MessageEnvelope)
	if !ok {
		that2, ok := that.(MessageEnvelope)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.TypeId != that1.TypeId {
		return false
	}
	if !bytes.Equal(this.MessageData, that1.MessageData) {
		return false
	}
	if this.Target != that1.Target {
		return false
	}
	if !this.Sender.Equal(that1.Sender) {
		return false
	}
	if this.SerializerId != that1.SerializerId {
		return false
	}
	if !this.MessageHeader.Equal(that1.MessageHeader) {
		return false
	}
	return true
}
func (this *MessageHeader) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*MessageHeader)
	if !ok {
		that2, ok := that.(MessageHeader)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if len(this.HeaderData) != len(that1.HeaderData) {
		return false
	}
	for i := range this.HeaderData {
		if this.HeaderData[i] != that1.HeaderData[i] {
			return false
		}
	}
	return true
}
func (this *ActorPidRequest) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*ActorPidRequest)
	if !ok {
		that2, ok := that.(ActorPidRequest)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.Name != that1.Name {
		return false
	}
	if this.Kind != that1.Kind {
		return false
	}
	return true
}
func (this *ActorPidResponse) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*ActorPidResponse)
	if !ok {
		that2, ok := that.(ActorPidResponse)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if !this.Pid.Equal(that1.Pid) {
		return false
	}
	if this.StatusCode != that1.StatusCode {
		return false
	}
	return true
}
func (this *Unit) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Unit)
	if !ok {
		that2, ok := that.(Unit)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	return true
}
func (this *ConnectRequest) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*ConnectRequest)
	if !ok {
		that2, ok := that.(ConnectRequest)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	return true
}
func (this *ConnectResponse) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*ConnectResponse)
	if !ok {
		that2, ok := that.(ConnectResponse)
		if ok {
			that1 = &that2
		} else {
			return false
		}
	}
	if that1 == nil {
		return this == nil
	} else if this == nil {
		return false
	}
	if this.DefaultSerializerId != that1.DefaultSerializerId {
		return false
	}
	return true
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// RemotingClient is the client API for Remoting service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type RemotingClient interface {
	Connect(ctx context.Context, in *ConnectRequest, opts ...grpc.CallOption) (*ConnectResponse, error)
	Receive(ctx context.Context, opts ...grpc.CallOption) (Remoting_ReceiveClient, error)
}

type remotingClient struct {
	cc *grpc.ClientConn
}

func NewRemotingClient(cc *grpc.ClientConn) RemotingClient {
	return &remotingClient{cc}
}

func (c *remotingClient) Connect(ctx context.Context, in *ConnectRequest, opts ...grpc.CallOption) (*ConnectResponse, error) {
	out := new(ConnectResponse)
	err := c.cc.Invoke(ctx, "/remote.Remoting/Connect", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *remotingClient) Receive(ctx context.Context, opts ...grpc.CallOption) (Remoting_ReceiveClient, error) {
	stream, err := c.cc.NewStream(ctx, &_Remoting_serviceDesc.Streams[0], "/remote.Remoting/Receive", opts...)
	if err != nil {
		return nil, err
	}
	x := &remotingReceiveClient{stream}
	return x, nil
}

type Remoting_ReceiveClient interface {
	Send(*MessageBatch) error
	Recv() (*Unit, error)
	grpc.ClientStream
}

type remotingReceiveClient struct {
	grpc.ClientStream
}

func (x *remotingReceiveClient) Send(m *MessageBatch) error {
	return x.ClientStream.SendMsg(m)
}

func (x *remotingReceiveClient) Recv() (*Unit, error) {
	m := new(Unit)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// RemotingServer is the server API for Remoting service.
type RemotingServer interface {
	Connect(context.Context, *ConnectRequest) (*ConnectResponse, error)
	Receive(Remoting_ReceiveServer) error
}

// UnimplementedRemotingServer can be embedded to have forward compatible implementations.
type UnimplementedRemotingServer struct {
}

func (*UnimplementedRemotingServer) Connect(ctx context.Context, req *ConnectRequest) (*ConnectResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Connect not implemented")
}
func (*UnimplementedRemotingServer) Receive(srv Remoting_ReceiveServer) error {
	return status.Errorf(codes.Unimplemented, "method Receive not implemented")
}

func RegisterRemotingServer(s *grpc.Server, srv RemotingServer) {
	s.RegisterService(&_Remoting_serviceDesc, srv)
}

func _Remoting_Connect_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConnectRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RemotingServer).Connect(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/remote.Remoting/Connect",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RemotingServer).Connect(ctx, req.(*ConnectRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Remoting_Receive_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(RemotingServer).Receive(&remotingReceiveServer{stream})
}

type Remoting_ReceiveServer interface {
	Send(*Unit) error
	Recv() (*MessageBatch, error)
	grpc.ServerStream
}

type remotingReceiveServer struct {
	grpc.ServerStream
}

func (x *remotingReceiveServer) Send(m *Unit) error {
	return x.ServerStream.SendMsg(m)
}

func (x *remotingReceiveServer) Recv() (*MessageBatch, error) {
	m := new(MessageBatch)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _Remoting_serviceDesc = grpc.ServiceDesc{
	ServiceName: "remote.Remoting",
	HandlerType: (*RemotingServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Connect",
			Handler:    _Remoting_Connect_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Receive",
			Handler:       _Remoting_Receive_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "protos.proto",
}

func (m *MessageBatch) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MessageBatch) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *MessageBatch) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Envelopes) > 0 {
		for iNdEx := len(m.Envelopes) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.Envelopes[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintProtos(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0x1a
		}
	}
	if len(m.TargetNames) > 0 {
		for iNdEx := len(m.TargetNames) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.TargetNames[iNdEx])
			copy(dAtA[i:], m.TargetNames[iNdEx])
			i = encodeVarintProtos(dAtA, i, uint64(len(m.TargetNames[iNdEx])))
			i--
			dAtA[i] = 0x12
		}
	}
	if len(m.TypeNames) > 0 {
		for iNdEx := len(m.TypeNames) - 1; iNdEx >= 0; iNdEx-- {
			i -= len(m.TypeNames[iNdEx])
			copy(dAtA[i:], m.TypeNames[iNdEx])
			i = encodeVarintProtos(dAtA, i, uint64(len(m.TypeNames[iNdEx])))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *MessageEnvelope) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MessageEnvelope) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *MessageEnvelope) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.MessageHeader != nil {
		{
			size, err := m.MessageHeader.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintProtos(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x32
	}
	if m.SerializerId != 0 {
		i = encodeVarintProtos(dAtA, i, uint64(m.SerializerId))
		i--
		dAtA[i] = 0x28
	}
	if m.Sender != nil {
		{
			size, err := m.Sender.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintProtos(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x22
	}
	if m.Target != 0 {
		i = encodeVarintProtos(dAtA, i, uint64(m.Target))
		i--
		dAtA[i] = 0x18
	}
	if len(m.MessageData) > 0 {
		i -= len(m.MessageData)
		copy(dAtA[i:], m.MessageData)
		i = encodeVarintProtos(dAtA, i, uint64(len(m.MessageData)))
		i--
		dAtA[i] = 0x12
	}
	if m.TypeId != 0 {
		i = encodeVarintProtos(dAtA, i, uint64(m.TypeId))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *MessageHeader) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MessageHeader) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *MessageHeader) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.HeaderData) > 0 {
		for k := range m.HeaderData {
			v := m.HeaderData[k]
			baseI := i
			i -= len(v)
			copy(dAtA[i:], v)
			i = encodeVarintProtos(dAtA, i, uint64(len(v)))
			i--
			dAtA[i] = 0x12
			i -= len(k)
			copy(dAtA[i:], k)
			i = encodeVarintProtos(dAtA, i, uint64(len(k)))
			i--
			dAtA[i] = 0xa
			i = encodeVarintProtos(dAtA, i, uint64(baseI-i))
			i--
			dAtA[i] = 0xa
		}
	}
	return len(dAtA) - i, nil
}

func (m *ActorPidRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ActorPidRequest) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ActorPidRequest) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Kind) > 0 {
		i -= len(m.Kind)
		copy(dAtA[i:], m.Kind)
		i = encodeVarintProtos(dAtA, i, uint64(len(m.Kind)))
		i--
		dAtA[i] = 0x12
	}
	if len(m.Name) > 0 {
		i -= len(m.Name)
		copy(dAtA[i:], m.Name)
		i = encodeVarintProtos(dAtA, i, uint64(len(m.Name)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *ActorPidResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ActorPidResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ActorPidResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.StatusCode != 0 {
		i = encodeVarintProtos(dAtA, i, uint64(m.StatusCode))
		i--
		dAtA[i] = 0x10
	}
	if m.Pid != nil {
		{
			size, err := m.Pid.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintProtos(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *Unit) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Unit) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Unit) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	return len(dAtA) - i, nil
}

func (m *ConnectRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ConnectRequest) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ConnectRequest) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	return len(dAtA) - i, nil
}

func (m *ConnectResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ConnectResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ConnectResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.DefaultSerializerId != 0 {
		i = encodeVarintProtos(dAtA, i, uint64(m.DefaultSerializerId))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintProtos(dAtA []byte, offset int, v uint64) int {
	offset -= sovProtos(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *MessageBatch) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.TypeNames) > 0 {
		for _, s := range m.TypeNames {
			l = len(s)
			n += 1 + l + sovProtos(uint64(l))
		}
	}
	if len(m.TargetNames) > 0 {
		for _, s := range m.TargetNames {
			l = len(s)
			n += 1 + l + sovProtos(uint64(l))
		}
	}
	if len(m.Envelopes) > 0 {
		for _, e := range m.Envelopes {
			l = e.Size()
			n += 1 + l + sovProtos(uint64(l))
		}
	}
	return n
}

func (m *MessageEnvelope) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.TypeId != 0 {
		n += 1 + sovProtos(uint64(m.TypeId))
	}
	l = len(m.MessageData)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	if m.Target != 0 {
		n += 1 + sovProtos(uint64(m.Target))
	}
	if m.Sender != nil {
		l = m.Sender.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	if m.SerializerId != 0 {
		n += 1 + sovProtos(uint64(m.SerializerId))
	}
	if m.MessageHeader != nil {
		l = m.MessageHeader.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *MessageHeader) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.HeaderData) > 0 {
		for k, v := range m.HeaderData {
			_ = k
			_ = v
			mapEntrySize := 1 + len(k) + sovProtos(uint64(len(k))) + 1 + len(v) + sovProtos(uint64(len(v)))
			n += mapEntrySize + 1 + sovProtos(uint64(mapEntrySize))
		}
	}
	return n
}

func (m *ActorPidRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Name)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	l = len(m.Kind)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *ActorPidResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Pid != nil {
		l = m.Pid.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	if m.StatusCode != 0 {
		n += 1 + sovProtos(uint64(m.StatusCode))
	}
	return n
}

func (m *Unit) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	return n
}

func (m *ConnectRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	return n
}

func (m *ConnectResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.DefaultSerializerId != 0 {
		n += 1 + sovProtos(uint64(m.DefaultSerializerId))
	}
	return n
}

func sovProtos(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozProtos(x uint64) (n int) {
	return sovProtos(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *MessageBatch) String() string {
	if this == nil {
		return "nil"
	}
	repeatedStringForEnvelopes := "[]*MessageEnvelope{"
	for _, f := range this.Envelopes {
		repeatedStringForEnvelopes += strings.Replace(f.String(), "MessageEnvelope", "MessageEnvelope", 1) + ","
	}
	repeatedStringForEnvelopes += "}"
	s := strings.Join([]string{`&MessageBatch{`,
		`TypeNames:` + fmt.Sprintf("%v", this.TypeNames) + `,`,
		`TargetNames:` + fmt.Sprintf("%v", this.TargetNames) + `,`,
		`Envelopes:` + repeatedStringForEnvelopes + `,`,
		`}`,
	}, "")
	return s
}
func (this *MessageEnvelope) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&MessageEnvelope{`,
		`TypeId:` + fmt.Sprintf("%v", this.TypeId) + `,`,
		`MessageData:` + fmt.Sprintf("%v", this.MessageData) + `,`,
		`Target:` + fmt.Sprintf("%v", this.Target) + `,`,
		`Sender:` + strings.Replace(fmt.Sprintf("%v", this.Sender), "PID", "actor.PID", 1) + `,`,
		`SerializerId:` + fmt.Sprintf("%v", this.SerializerId) + `,`,
		`MessageHeader:` + strings.Replace(this.MessageHeader.String(), "MessageHeader", "MessageHeader", 1) + `,`,
		`}`,
	}, "")
	return s
}
func (this *MessageHeader) String() string {
	if this == nil {
		return "nil"
	}
	keysForHeaderData := make([]string, 0, len(this.HeaderData))
	for k, _ := range this.HeaderData {
		keysForHeaderData = append(keysForHeaderData, k)
	}
	github_com_gogo_protobuf_sortkeys.Strings(keysForHeaderData)
	mapStringForHeaderData := "map[string]string{"
	for _, k := range keysForHeaderData {
		mapStringForHeaderData += fmt.Sprintf("%v: %v,", k, this.HeaderData[k])
	}
	mapStringForHeaderData += "}"
	s := strings.Join([]string{`&MessageHeader{`,
		`HeaderData:` + mapStringForHeaderData + `,`,
		`}`,
	}, "")
	return s
}
func (this *ActorPidRequest) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&ActorPidRequest{`,
		`Name:` + fmt.Sprintf("%v", this.Name) + `,`,
		`Kind:` + fmt.Sprintf("%v", this.Kind) + `,`,
		`}`,
	}, "")
	return s
}
func (this *ActorPidResponse) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&ActorPidResponse{`,
		`Pid:` + strings.Replace(fmt.Sprintf("%v", this.Pid), "PID", "actor.PID", 1) + `,`,
		`StatusCode:` + fmt.Sprintf("%v", this.StatusCode) + `,`,
		`}`,
	}, "")
	return s
}
func (this *Unit) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Unit{`,
		`}`,
	}, "")
	return s
}
func (this *ConnectRequest) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&ConnectRequest{`,
		`}`,
	}, "")
	return s
}
func (this *ConnectResponse) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&ConnectResponse{`,
		`DefaultSerializerId:` + fmt.Sprintf("%v", this.DefaultSerializerId) + `,`,
		`}`,
	}, "")
	return s
}
func valueToStringProtos(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
}
func (m *MessageBatch) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowProtos
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: MessageBatch: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MessageBatch: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field TypeNames", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthProtos
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.TypeNames = append(m.TypeNames, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field TargetNames", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthProtos
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.TargetNames = append(m.TargetNames, string(dAtA[iNdEx:postIndex]))
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Envelopes", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthProtos
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Envelopes = append(m.Envelopes, &MessageEnvelope{})
			if err := m.Envelopes[len(m.Envelopes)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipProtos(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *MessageEnvelope) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowProtos
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: MessageEnvelope: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MessageEnvelope: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field TypeId", wireType)
			}
			m.TypeId = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.TypeId |= int32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field MessageData", wireType)
			}
			var byteLen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				byteLen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + byteLen
			if postIndex < 0 {
				return ErrInvalidLengthProtos
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.MessageData = append(m.MessageData[:0], dAtA[iNdEx:postIndex]...)
			if m.MessageData == nil {
				m.MessageData = []byte{}
			}
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Target", wireType)
			}
			m.Target = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Target |= int32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 4:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Sender", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthProtos
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Sender == nil {
				m.Sender = &actor.PID{}
			}
			if err := m.Sender.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 5:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field SerializerId", wireType)
			}
			m.SerializerId = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.SerializerId |= int32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 6:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field MessageHeader", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthProtos
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.MessageHeader == nil {
				m.MessageHeader = &MessageHeader{}
			}
			if err := m.MessageHeader.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipProtos(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *MessageHeader) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowProtos
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: MessageHeader: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MessageHeader: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field HeaderData", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthProtos
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.HeaderData == nil {
				m.HeaderData = make(map[string]string)
			}
			var mapkey string
			var mapvalue string
			for iNdEx < postIndex {
				entryPreIndex := iNdEx
				var wire uint64
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return ErrIntOverflowProtos
					}
					if iNdEx >= l {
						return io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					wire |= uint64(b&0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				fieldNum := int32(wire >> 3)
				if fieldNum == 1 {
					var stringLenmapkey uint64
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowProtos
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						stringLenmapkey |= uint64(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					intStringLenmapkey := int(stringLenmapkey)
					if intStringLenmapkey < 0 {
						return ErrInvalidLengthProtos
					}
					postStringIndexmapkey := iNdEx + intStringLenmapkey
					if postStringIndexmapkey < 0 {
						return ErrInvalidLengthProtos
					}
					if postStringIndexmapkey > l {
						return io.ErrUnexpectedEOF
					}
					mapkey = string(dAtA[iNdEx:postStringIndexmapkey])
					iNdEx = postStringIndexmapkey
				} else if fieldNum == 2 {
					var stringLenmapvalue uint64
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowProtos
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						stringLenmapvalue |= uint64(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
					intStringLenmapvalue := int(stringLenmapvalue)
					if intStringLenmapvalue < 0 {
						return ErrInvalidLengthProtos
					}
					postStringIndexmapvalue := iNdEx + intStringLenmapvalue
					if postStringIndexmapvalue < 0 {
						return ErrInvalidLengthProtos
					}
					if postStringIndexmapvalue > l {
						return io.ErrUnexpectedEOF
					}
					mapvalue = string(dAtA[iNdEx:postStringIndexmapvalue])
					iNdEx = postStringIndexmapvalue
				} else {
					iNdEx = entryPreIndex
					skippy, err := skipProtos(dAtA[iNdEx:])
					if err != nil {
						return err
					}
					if skippy < 0 {
						return ErrInvalidLengthProtos
					}
					if (iNdEx + skippy) > postIndex {
						return io.ErrUnexpectedEOF
					}
					iNdEx += skippy
				}
			}
			m.HeaderData[mapkey] = mapvalue
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipProtos(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *ActorPidRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowProtos
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: ActorPidRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ActorPidRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Name", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthProtos
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Name = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Kind", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthProtos
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Kind = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipProtos(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *ActorPidResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowProtos
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: ActorPidResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ActorPidResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Pid", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthProtos
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Pid == nil {
				m.Pid = &actor.PID{}
			}
			if err := m.Pid.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field StatusCode", wireType)
			}
			m.StatusCode = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.StatusCode |= int32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipProtos(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *Unit) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowProtos
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: Unit: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Unit: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		default:
			iNdEx = preIndex
			skippy, err := skipProtos(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *ConnectRequest) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowProtos
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: ConnectRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ConnectRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		default:
			iNdEx = preIndex
			skippy, err := skipProtos(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *ConnectResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowProtos
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: ConnectResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ConnectResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field DefaultSerializerId", wireType)
			}
			m.DefaultSerializerId = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.DefaultSerializerId |= int32(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		default:
			iNdEx = preIndex
			skippy, err := skipProtos(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) < 0 {
				return ErrInvalidLengthProtos
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipProtos(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowProtos
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthProtos
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupProtos
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthProtos
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthProtos        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowProtos          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupProtos = fmt.Errorf("proto: unexpected end of group")
)
