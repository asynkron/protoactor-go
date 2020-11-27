package cluster

import (
	bytes "bytes"
	fmt "fmt"
	actor "github.com/AsynkronIT/protoactor-go/actor"
	_ "github.com/gogo/protobuf/gogoproto"
	proto "github.com/gogo/protobuf/proto"
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

type TakeOwnership struct {
	Pid  *actor.PID `protobuf:"bytes,1,opt,name=pid,proto3" json:"pid,omitempty"`
	Name string     `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
}

func (m *TakeOwnership) Reset()      { *m = TakeOwnership{} }
func (*TakeOwnership) ProtoMessage() {}
func (*TakeOwnership) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{0}
}
func (m *TakeOwnership) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *TakeOwnership) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_TakeOwnership.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *TakeOwnership) XXX_Merge(src proto.Message) {
	xxx_messageInfo_TakeOwnership.Merge(m, src)
}
func (m *TakeOwnership) XXX_Size() int {
	return m.Size()
}
func (m *TakeOwnership) XXX_DiscardUnknown() {
	xxx_messageInfo_TakeOwnership.DiscardUnknown(m)
}

var xxx_messageInfo_TakeOwnership proto.InternalMessageInfo

func (m *TakeOwnership) GetPid() *actor.PID {
	if m != nil {
		return m.Pid
	}
	return nil
}

func (m *TakeOwnership) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

type GrainRequest struct {
	MethodIndex int32  `protobuf:"varint,1,opt,name=method_index,json=methodIndex,proto3" json:"method_index,omitempty"`
	MessageData []byte `protobuf:"bytes,2,opt,name=message_data,json=messageData,proto3" json:"message_data,omitempty"`
}

func (m *GrainRequest) Reset()      { *m = GrainRequest{} }
func (*GrainRequest) ProtoMessage() {}
func (*GrainRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{1}
}
func (m *GrainRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GrainRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GrainRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GrainRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GrainRequest.Merge(m, src)
}
func (m *GrainRequest) XXX_Size() int {
	return m.Size()
}
func (m *GrainRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_GrainRequest.DiscardUnknown(m)
}

var xxx_messageInfo_GrainRequest proto.InternalMessageInfo

func (m *GrainRequest) GetMethodIndex() int32 {
	if m != nil {
		return m.MethodIndex
	}
	return 0
}

func (m *GrainRequest) GetMessageData() []byte {
	if m != nil {
		return m.MessageData
	}
	return nil
}

type GrainResponse struct {
	MessageData []byte `protobuf:"bytes,1,opt,name=message_data,json=messageData,proto3" json:"message_data,omitempty"`
}

func (m *GrainResponse) Reset()      { *m = GrainResponse{} }
func (*GrainResponse) ProtoMessage() {}
func (*GrainResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{2}
}
func (m *GrainResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GrainResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GrainResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GrainResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GrainResponse.Merge(m, src)
}
func (m *GrainResponse) XXX_Size() int {
	return m.Size()
}
func (m *GrainResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_GrainResponse.DiscardUnknown(m)
}

var xxx_messageInfo_GrainResponse proto.InternalMessageInfo

func (m *GrainResponse) GetMessageData() []byte {
	if m != nil {
		return m.MessageData
	}
	return nil
}

type GrainErrorResponse struct {
	Err string `protobuf:"bytes,1,opt,name=err,proto3" json:"err,omitempty"`
}

func (m *GrainErrorResponse) Reset()      { *m = GrainErrorResponse{} }
func (*GrainErrorResponse) ProtoMessage() {}
func (*GrainErrorResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{3}
}
func (m *GrainErrorResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GrainErrorResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GrainErrorResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GrainErrorResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GrainErrorResponse.Merge(m, src)
}
func (m *GrainErrorResponse) XXX_Size() int {
	return m.Size()
}
func (m *GrainErrorResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_GrainErrorResponse.DiscardUnknown(m)
}

var xxx_messageInfo_GrainErrorResponse proto.InternalMessageInfo

func (m *GrainErrorResponse) GetErr() string {
	if m != nil {
		return m.Err
	}
	return ""
}

type ClusterIdentity struct {
	Identity string `protobuf:"bytes,1,opt,name=identity,proto3" json:"identity,omitempty"`
	Kind     string `protobuf:"bytes,2,opt,name=kind,proto3" json:"kind,omitempty"`
}

func (m *ClusterIdentity) Reset()      { *m = ClusterIdentity{} }
func (*ClusterIdentity) ProtoMessage() {}
func (*ClusterIdentity) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{4}
}
func (m *ClusterIdentity) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ClusterIdentity) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ClusterIdentity.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ClusterIdentity) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ClusterIdentity.Merge(m, src)
}
func (m *ClusterIdentity) XXX_Size() int {
	return m.Size()
}
func (m *ClusterIdentity) XXX_DiscardUnknown() {
	xxx_messageInfo_ClusterIdentity.DiscardUnknown(m)
}

var xxx_messageInfo_ClusterIdentity proto.InternalMessageInfo

func (m *ClusterIdentity) GetIdentity() string {
	if m != nil {
		return m.Identity
	}
	return ""
}

func (m *ClusterIdentity) GetKind() string {
	if m != nil {
		return m.Kind
	}
	return ""
}

type Activation struct {
	Pid             *actor.PID       `protobuf:"bytes,1,opt,name=pid,proto3" json:"pid,omitempty"`
	ClusterIdentity *ClusterIdentity `protobuf:"bytes,2,opt,name=cluster_identity,json=clusterIdentity,proto3" json:"cluster_identity,omitempty"`
	EventId         uint64           `protobuf:"varint,3,opt,name=eventId,proto3" json:"eventId,omitempty"`
}

func (m *Activation) Reset()      { *m = Activation{} }
func (*Activation) ProtoMessage() {}
func (*Activation) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{5}
}
func (m *Activation) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Activation) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Activation.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Activation) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Activation.Merge(m, src)
}
func (m *Activation) XXX_Size() int {
	return m.Size()
}
func (m *Activation) XXX_DiscardUnknown() {
	xxx_messageInfo_Activation.DiscardUnknown(m)
}

var xxx_messageInfo_Activation proto.InternalMessageInfo

func (m *Activation) GetPid() *actor.PID {
	if m != nil {
		return m.Pid
	}
	return nil
}

func (m *Activation) GetClusterIdentity() *ClusterIdentity {
	if m != nil {
		return m.ClusterIdentity
	}
	return nil
}

func (m *Activation) GetEventId() uint64 {
	if m != nil {
		return m.EventId
	}
	return 0
}

type ActivationTerminated struct {
	Pid             *actor.PID       `protobuf:"bytes,1,opt,name=pid,proto3" json:"pid,omitempty"`
	ClusterIdentity *ClusterIdentity `protobuf:"bytes,2,opt,name=cluster_identity,json=clusterIdentity,proto3" json:"cluster_identity,omitempty"`
	EventId         uint64           `protobuf:"varint,3,opt,name=eventId,proto3" json:"eventId,omitempty"`
}

func (m *ActivationTerminated) Reset()      { *m = ActivationTerminated{} }
func (*ActivationTerminated) ProtoMessage() {}
func (*ActivationTerminated) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{6}
}
func (m *ActivationTerminated) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ActivationTerminated) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ActivationTerminated.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ActivationTerminated) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ActivationTerminated.Merge(m, src)
}
func (m *ActivationTerminated) XXX_Size() int {
	return m.Size()
}
func (m *ActivationTerminated) XXX_DiscardUnknown() {
	xxx_messageInfo_ActivationTerminated.DiscardUnknown(m)
}

var xxx_messageInfo_ActivationTerminated proto.InternalMessageInfo

func (m *ActivationTerminated) GetPid() *actor.PID {
	if m != nil {
		return m.Pid
	}
	return nil
}

func (m *ActivationTerminated) GetClusterIdentity() *ClusterIdentity {
	if m != nil {
		return m.ClusterIdentity
	}
	return nil
}

func (m *ActivationTerminated) GetEventId() uint64 {
	if m != nil {
		return m.EventId
	}
	return 0
}

type ActivationRequest struct {
	ClusterIdentity *ClusterIdentity `protobuf:"bytes,1,opt,name=cluster_identity,json=clusterIdentity,proto3" json:"cluster_identity,omitempty"`
	RequestId       string           `protobuf:"bytes,2,opt,name=request_id,json=requestId,proto3" json:"request_id,omitempty"`
}

func (m *ActivationRequest) Reset()      { *m = ActivationRequest{} }
func (*ActivationRequest) ProtoMessage() {}
func (*ActivationRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{7}
}
func (m *ActivationRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *ActivationRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_ActivationRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *ActivationRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ActivationRequest.Merge(m, src)
}
func (m *ActivationRequest) XXX_Size() int {
	return m.Size()
}
func (m *ActivationRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ActivationRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ActivationRequest proto.InternalMessageInfo

func (m *ActivationRequest) GetClusterIdentity() *ClusterIdentity {
	if m != nil {
		return m.ClusterIdentity
	}
	return nil
}

func (m *ActivationRequest) GetRequestId() string {
	if m != nil {
		return m.RequestId
	}
	return ""
}

func init() {
	proto.RegisterType((*TakeOwnership)(nil), "cluster.TakeOwnership")
	proto.RegisterType((*GrainRequest)(nil), "cluster.GrainRequest")
	proto.RegisterType((*GrainResponse)(nil), "cluster.GrainResponse")
	proto.RegisterType((*GrainErrorResponse)(nil), "cluster.GrainErrorResponse")
	proto.RegisterType((*ClusterIdentity)(nil), "cluster.ClusterIdentity")
	proto.RegisterType((*Activation)(nil), "cluster.Activation")
	proto.RegisterType((*ActivationTerminated)(nil), "cluster.ActivationTerminated")
	proto.RegisterType((*ActivationRequest)(nil), "cluster.ActivationRequest")
}

func init() { proto.RegisterFile("protos.proto", fileDescriptor_5da3cbeb884d181c) }

var fileDescriptor_5da3cbeb884d181c = []byte{
	// 454 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xc4, 0x53, 0x31, 0x6f, 0xd3, 0x40,
	0x14, 0xf6, 0x35, 0x85, 0x92, 0xd7, 0x54, 0x2d, 0x16, 0x83, 0x15, 0xc1, 0x29, 0x78, 0x40, 0x59,
	0xea, 0x48, 0x05, 0xb1, 0x87, 0x16, 0x21, 0x4f, 0x20, 0x2b, 0x7b, 0x74, 0xf1, 0x3d, 0x9c, 0x53,
	0xc8, 0x5d, 0xb8, 0x3b, 0xb7, 0x74, 0x63, 0x65, 0x43, 0xe2, 0x4f, 0xf0, 0x53, 0x18, 0x33, 0x76,
	0x24, 0xce, 0xc2, 0xd8, 0x9f, 0x80, 0x7c, 0xb6, 0xd3, 0xaa, 0x41, 0x42, 0x4c, 0x4c, 0xfe, 0xde,
	0x77, 0xdf, 0xf7, 0xde, 0x77, 0x77, 0x3e, 0xe8, 0x2c, 0xb4, 0xb2, 0xca, 0x44, 0xee, 0xe3, 0xef,
	0xa5, 0x1f, 0x72, 0x63, 0x51, 0x77, 0x8f, 0x33, 0x61, 0xa7, 0xf9, 0x24, 0x4a, 0xd5, 0x7c, 0x90,
	0xa9, 0x4c, 0x0d, 0xdc, 0xfa, 0x24, 0x7f, 0xef, 0x2a, 0x57, 0x38, 0x54, 0xf9, 0xba, 0x2f, 0x6f,
	0xc9, 0x87, 0xe6, 0x52, 0xce, 0xb4, 0x92, 0xf1, 0xa8, 0x32, 0xb1, 0xd4, 0x2a, 0x7d, 0x9c, 0xa9,
	0x81, 0x03, 0x83, 0xdb, 0xf3, 0xc2, 0x21, 0x1c, 0x8c, 0xd8, 0x0c, 0xdf, 0x5e, 0x48, 0xd4, 0x66,
	0x2a, 0x16, 0xfe, 0x63, 0x68, 0x2d, 0x04, 0x0f, 0x48, 0x8f, 0xf4, 0xf7, 0x4f, 0x20, 0x72, 0x96,
	0xe8, 0x5d, 0x7c, 0x96, 0x94, 0xb4, 0xef, 0xc3, 0xae, 0x64, 0x73, 0x0c, 0x76, 0x7a, 0xa4, 0xdf,
	0x4e, 0x1c, 0x0e, 0x47, 0xd0, 0x79, 0xa3, 0x99, 0x90, 0x09, 0x7e, 0xcc, 0xd1, 0x58, 0xff, 0x29,
	0x74, 0xe6, 0x68, 0xa7, 0x8a, 0x8f, 0x85, 0xe4, 0xf8, 0xc9, 0xb5, 0xba, 0x97, 0xec, 0x57, 0x5c,
	0x5c, 0x52, 0x95, 0xc4, 0x18, 0x96, 0xe1, 0x98, 0x33, 0xcb, 0x5c, 0xbb, 0x4e, 0x29, 0x71, 0xdc,
	0x19, 0xb3, 0x2c, 0x3c, 0x81, 0x83, 0xba, 0xab, 0x59, 0x28, 0x69, 0x70, 0xcb, 0x43, 0xb6, 0x3d,
	0xcf, 0xc0, 0x77, 0x9e, 0xd7, 0x5a, 0x2b, 0xbd, 0x31, 0x1e, 0x41, 0x0b, 0xb5, 0x76, 0xfa, 0x76,
	0x52, 0xc2, 0x70, 0x08, 0x87, 0xa7, 0xd5, 0x31, 0xc7, 0x1c, 0xa5, 0x15, 0xf6, 0xd2, 0xef, 0xc2,
	0x03, 0x51, 0xe3, 0x5a, 0xb9, 0xa9, 0xcb, 0x4d, 0xcf, 0x84, 0xe4, 0xcd, 0xa6, 0x4b, 0x1c, 0x7e,
	0x21, 0x00, 0xc3, 0xd4, 0x8a, 0x73, 0x66, 0x85, 0x92, 0x7f, 0x39, 0xb5, 0x53, 0x38, 0xaa, 0xaf,
	0x75, 0xbc, 0x19, 0xb2, 0xe3, 0xa4, 0x41, 0x54, 0x2f, 0x44, 0x77, 0x02, 0x25, 0x87, 0xe9, 0x9d,
	0x84, 0x01, 0xec, 0xe1, 0x39, 0x4a, 0x1b, 0xf3, 0xa0, 0xd5, 0x23, 0xfd, 0xdd, 0xa4, 0x29, 0xc3,
	0x6f, 0x04, 0x1e, 0xdd, 0x64, 0x19, 0xa1, 0x9e, 0x0b, 0xc9, 0x2c, 0xf2, 0xff, 0x9b, 0xea, 0x02,
	0x1e, 0xde, 0x84, 0x6a, 0xfe, 0x8d, 0x3f, 0xcd, 0x24, 0xff, 0x3a, 0xf3, 0x09, 0x80, 0xae, 0xfa,
	0x8d, 0x45, 0x73, 0x2b, 0xed, 0x9a, 0x89, 0xf9, 0xab, 0x17, 0xcb, 0x15, 0xf5, 0xae, 0x56, 0xd4,
	0xbb, 0x5e, 0x51, 0xef, 0x73, 0x41, 0xc9, 0xf7, 0x82, 0x92, 0x1f, 0x05, 0x25, 0xcb, 0x82, 0x92,
	0x9f, 0x05, 0x25, 0xbf, 0x0a, 0xea, 0x5d, 0x17, 0x94, 0x7c, 0x5d, 0x53, 0x6f, 0xb9, 0xa6, 0xde,
	0xd5, 0x9a, 0x7a, 0x93, 0xfb, 0xee, 0x3d, 0x3c, 0xff, 0x1d, 0x00, 0x00, 0xff, 0xff, 0xc0, 0xa5,
	0x11, 0x22, 0x8f, 0x03, 0x00, 0x00,
}

func (this *TakeOwnership) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*TakeOwnership)
	if !ok {
		that2, ok := that.(TakeOwnership)
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
	if this.Name != that1.Name {
		return false
	}
	return true
}
func (this *GrainRequest) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*GrainRequest)
	if !ok {
		that2, ok := that.(GrainRequest)
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
	if this.MethodIndex != that1.MethodIndex {
		return false
	}
	if !bytes.Equal(this.MessageData, that1.MessageData) {
		return false
	}
	return true
}
func (this *GrainResponse) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*GrainResponse)
	if !ok {
		that2, ok := that.(GrainResponse)
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
	if !bytes.Equal(this.MessageData, that1.MessageData) {
		return false
	}
	return true
}
func (this *GrainErrorResponse) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*GrainErrorResponse)
	if !ok {
		that2, ok := that.(GrainErrorResponse)
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
	if this.Err != that1.Err {
		return false
	}
	return true
}
func (this *ClusterIdentity) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*ClusterIdentity)
	if !ok {
		that2, ok := that.(ClusterIdentity)
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
	if this.Identity != that1.Identity {
		return false
	}
	if this.Kind != that1.Kind {
		return false
	}
	return true
}
func (this *Activation) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Activation)
	if !ok {
		that2, ok := that.(Activation)
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
	if !this.ClusterIdentity.Equal(that1.ClusterIdentity) {
		return false
	}
	if this.EventId != that1.EventId {
		return false
	}
	return true
}
func (this *ActivationTerminated) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*ActivationTerminated)
	if !ok {
		that2, ok := that.(ActivationTerminated)
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
	if !this.ClusterIdentity.Equal(that1.ClusterIdentity) {
		return false
	}
	if this.EventId != that1.EventId {
		return false
	}
	return true
}
func (this *ActivationRequest) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*ActivationRequest)
	if !ok {
		that2, ok := that.(ActivationRequest)
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
	if !this.ClusterIdentity.Equal(that1.ClusterIdentity) {
		return false
	}
	if this.RequestId != that1.RequestId {
		return false
	}
	return true
}
func (m *TakeOwnership) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *TakeOwnership) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *TakeOwnership) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Name) > 0 {
		i -= len(m.Name)
		copy(dAtA[i:], m.Name)
		i = encodeVarintProtos(dAtA, i, uint64(len(m.Name)))
		i--
		dAtA[i] = 0x12
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

func (m *GrainRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GrainRequest) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *GrainRequest) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.MessageData) > 0 {
		i -= len(m.MessageData)
		copy(dAtA[i:], m.MessageData)
		i = encodeVarintProtos(dAtA, i, uint64(len(m.MessageData)))
		i--
		dAtA[i] = 0x12
	}
	if m.MethodIndex != 0 {
		i = encodeVarintProtos(dAtA, i, uint64(m.MethodIndex))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *GrainResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GrainResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *GrainResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.MessageData) > 0 {
		i -= len(m.MessageData)
		copy(dAtA[i:], m.MessageData)
		i = encodeVarintProtos(dAtA, i, uint64(len(m.MessageData)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *GrainErrorResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GrainErrorResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *GrainErrorResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Err) > 0 {
		i -= len(m.Err)
		copy(dAtA[i:], m.Err)
		i = encodeVarintProtos(dAtA, i, uint64(len(m.Err)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *ClusterIdentity) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ClusterIdentity) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ClusterIdentity) MarshalToSizedBuffer(dAtA []byte) (int, error) {
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
	if len(m.Identity) > 0 {
		i -= len(m.Identity)
		copy(dAtA[i:], m.Identity)
		i = encodeVarintProtos(dAtA, i, uint64(len(m.Identity)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *Activation) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Activation) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Activation) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.EventId != 0 {
		i = encodeVarintProtos(dAtA, i, uint64(m.EventId))
		i--
		dAtA[i] = 0x18
	}
	if m.ClusterIdentity != nil {
		{
			size, err := m.ClusterIdentity.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintProtos(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x12
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

func (m *ActivationTerminated) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ActivationTerminated) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ActivationTerminated) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.EventId != 0 {
		i = encodeVarintProtos(dAtA, i, uint64(m.EventId))
		i--
		dAtA[i] = 0x18
	}
	if m.ClusterIdentity != nil {
		{
			size, err := m.ClusterIdentity.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintProtos(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x12
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

func (m *ActivationRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ActivationRequest) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *ActivationRequest) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.RequestId) > 0 {
		i -= len(m.RequestId)
		copy(dAtA[i:], m.RequestId)
		i = encodeVarintProtos(dAtA, i, uint64(len(m.RequestId)))
		i--
		dAtA[i] = 0x12
	}
	if m.ClusterIdentity != nil {
		{
			size, err := m.ClusterIdentity.MarshalToSizedBuffer(dAtA[:i])
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
func (m *TakeOwnership) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Pid != nil {
		l = m.Pid.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	l = len(m.Name)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *GrainRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.MethodIndex != 0 {
		n += 1 + sovProtos(uint64(m.MethodIndex))
	}
	l = len(m.MessageData)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *GrainResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.MessageData)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *GrainErrorResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Err)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *ClusterIdentity) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Identity)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	l = len(m.Kind)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *Activation) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Pid != nil {
		l = m.Pid.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	if m.ClusterIdentity != nil {
		l = m.ClusterIdentity.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	if m.EventId != 0 {
		n += 1 + sovProtos(uint64(m.EventId))
	}
	return n
}

func (m *ActivationTerminated) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Pid != nil {
		l = m.Pid.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	if m.ClusterIdentity != nil {
		l = m.ClusterIdentity.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	if m.EventId != 0 {
		n += 1 + sovProtos(uint64(m.EventId))
	}
	return n
}

func (m *ActivationRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.ClusterIdentity != nil {
		l = m.ClusterIdentity.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	l = len(m.RequestId)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func sovProtos(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozProtos(x uint64) (n int) {
	return sovProtos(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *TakeOwnership) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&TakeOwnership{`,
		`Pid:` + strings.Replace(fmt.Sprintf("%v", this.Pid), "PID", "actor.PID", 1) + `,`,
		`Name:` + fmt.Sprintf("%v", this.Name) + `,`,
		`}`,
	}, "")
	return s
}
func (this *GrainRequest) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&GrainRequest{`,
		`MethodIndex:` + fmt.Sprintf("%v", this.MethodIndex) + `,`,
		`MessageData:` + fmt.Sprintf("%v", this.MessageData) + `,`,
		`}`,
	}, "")
	return s
}
func (this *GrainResponse) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&GrainResponse{`,
		`MessageData:` + fmt.Sprintf("%v", this.MessageData) + `,`,
		`}`,
	}, "")
	return s
}
func (this *GrainErrorResponse) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&GrainErrorResponse{`,
		`Err:` + fmt.Sprintf("%v", this.Err) + `,`,
		`}`,
	}, "")
	return s
}
func (this *ClusterIdentity) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&ClusterIdentity{`,
		`Identity:` + fmt.Sprintf("%v", this.Identity) + `,`,
		`Kind:` + fmt.Sprintf("%v", this.Kind) + `,`,
		`}`,
	}, "")
	return s
}
func (this *Activation) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Activation{`,
		`Pid:` + strings.Replace(fmt.Sprintf("%v", this.Pid), "PID", "actor.PID", 1) + `,`,
		`ClusterIdentity:` + strings.Replace(this.ClusterIdentity.String(), "ClusterIdentity", "ClusterIdentity", 1) + `,`,
		`EventId:` + fmt.Sprintf("%v", this.EventId) + `,`,
		`}`,
	}, "")
	return s
}
func (this *ActivationTerminated) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&ActivationTerminated{`,
		`Pid:` + strings.Replace(fmt.Sprintf("%v", this.Pid), "PID", "actor.PID", 1) + `,`,
		`ClusterIdentity:` + strings.Replace(this.ClusterIdentity.String(), "ClusterIdentity", "ClusterIdentity", 1) + `,`,
		`EventId:` + fmt.Sprintf("%v", this.EventId) + `,`,
		`}`,
	}, "")
	return s
}
func (this *ActivationRequest) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&ActivationRequest{`,
		`ClusterIdentity:` + strings.Replace(this.ClusterIdentity.String(), "ClusterIdentity", "ClusterIdentity", 1) + `,`,
		`RequestId:` + fmt.Sprintf("%v", this.RequestId) + `,`,
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
func (m *TakeOwnership) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: TakeOwnership: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: TakeOwnership: illegal tag %d (wire type %d)", fieldNum, wire)
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
func (m *GrainRequest) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: GrainRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GrainRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field MethodIndex", wireType)
			}
			m.MethodIndex = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.MethodIndex |= int32(b&0x7F) << shift
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
func (m *GrainResponse) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: GrainResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GrainResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
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
func (m *GrainErrorResponse) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: GrainErrorResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GrainErrorResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Err", wireType)
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
			m.Err = string(dAtA[iNdEx:postIndex])
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
func (m *ClusterIdentity) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: ClusterIdentity: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ClusterIdentity: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Identity", wireType)
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
			m.Identity = string(dAtA[iNdEx:postIndex])
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
func (m *Activation) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: Activation: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Activation: illegal tag %d (wire type %d)", fieldNum, wire)
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
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ClusterIdentity", wireType)
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
			if m.ClusterIdentity == nil {
				m.ClusterIdentity = &ClusterIdentity{}
			}
			if err := m.ClusterIdentity.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field EventId", wireType)
			}
			m.EventId = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.EventId |= uint64(b&0x7F) << shift
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
func (m *ActivationTerminated) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: ActivationTerminated: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ActivationTerminated: illegal tag %d (wire type %d)", fieldNum, wire)
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
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ClusterIdentity", wireType)
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
			if m.ClusterIdentity == nil {
				m.ClusterIdentity = &ClusterIdentity{}
			}
			if err := m.ClusterIdentity.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 3:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field EventId", wireType)
			}
			m.EventId = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.EventId |= uint64(b&0x7F) << shift
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
func (m *ActivationRequest) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: ActivationRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: ActivationRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field ClusterIdentity", wireType)
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
			if m.ClusterIdentity == nil {
				m.ClusterIdentity = &ClusterIdentity{}
			}
			if err := m.ClusterIdentity.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field RequestId", wireType)
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
			m.RequestId = string(dAtA[iNdEx:postIndex])
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
