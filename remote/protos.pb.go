package remote

import proto "github.com/gogo/protobuf/proto"
import fmt "fmt"
import math "math"
import _ "github.com/gogo/protobuf/gogoproto"
import actor "github.com/AsynkronIT/protoactor-go/actor"

import bytes "bytes"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

import strings "strings"
import reflect "reflect"

import io "io"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion2 // please upgrade the proto package

type MessageEnvelope struct {
	TypeName    string     `protobuf:"bytes,1,opt,name=type_name,json=typeName,proto3" json:"type_name,omitempty"`
	MessageData []byte     `protobuf:"bytes,2,opt,name=message_data,json=messageData,proto3" json:"message_data,omitempty"`
	Target      *actor.PID `protobuf:"bytes,3,opt,name=target" json:"target,omitempty"`
	Sender      *actor.PID `protobuf:"bytes,4,opt,name=sender" json:"sender,omitempty"`
}

func (m *MessageEnvelope) Reset()                    { *m = MessageEnvelope{} }
func (*MessageEnvelope) ProtoMessage()               {}
func (*MessageEnvelope) Descriptor() ([]byte, []int) { return fileDescriptorProtos, []int{0} }

func (m *MessageEnvelope) GetTarget() *actor.PID {
	if m != nil {
		return m.Target
	}
	return nil
}

func (m *MessageEnvelope) GetSender() *actor.PID {
	if m != nil {
		return m.Sender
	}
	return nil
}

type ActorPidRequest struct {
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Kind string `protobuf:"bytes,2,opt,name=kind,proto3" json:"kind,omitempty"`
}

func (m *ActorPidRequest) Reset()                    { *m = ActorPidRequest{} }
func (*ActorPidRequest) ProtoMessage()               {}
func (*ActorPidRequest) Descriptor() ([]byte, []int) { return fileDescriptorProtos, []int{1} }

type ActorPidResponse struct {
	Pid *actor.PID `protobuf:"bytes,1,opt,name=pid" json:"pid,omitempty"`
}

func (m *ActorPidResponse) Reset()                    { *m = ActorPidResponse{} }
func (*ActorPidResponse) ProtoMessage()               {}
func (*ActorPidResponse) Descriptor() ([]byte, []int) { return fileDescriptorProtos, []int{2} }

func (m *ActorPidResponse) GetPid() *actor.PID {
	if m != nil {
		return m.Pid
	}
	return nil
}

type MessageBatch struct {
	Envelopes []*MessageEnvelope `protobuf:"bytes,1,rep,name=envelopes" json:"envelopes,omitempty"`
}

func (m *MessageBatch) Reset()                    { *m = MessageBatch{} }
func (*MessageBatch) ProtoMessage()               {}
func (*MessageBatch) Descriptor() ([]byte, []int) { return fileDescriptorProtos, []int{3} }

func (m *MessageBatch) GetEnvelopes() []*MessageEnvelope {
	if m != nil {
		return m.Envelopes
	}
	return nil
}

type Unit struct {
}

func (m *Unit) Reset()                    { *m = Unit{} }
func (*Unit) ProtoMessage()               {}
func (*Unit) Descriptor() ([]byte, []int) { return fileDescriptorProtos, []int{4} }

func init() {
	proto.RegisterType((*MessageEnvelope)(nil), "remote.MessageEnvelope")
	proto.RegisterType((*ActorPidRequest)(nil), "remote.ActorPidRequest")
	proto.RegisterType((*ActorPidResponse)(nil), "remote.ActorPidResponse")
	proto.RegisterType((*MessageBatch)(nil), "remote.MessageBatch")
	proto.RegisterType((*Unit)(nil), "remote.Unit")
}
func (this *MessageEnvelope) Equal(that interface{}) bool {
	if that == nil {
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
	} else if this == nil {
		return false
	}
	if this.TypeName != that1.TypeName {
		return false
	}
	if !bytes.Equal(this.MessageData, that1.MessageData) {
		return false
	}
	if !this.Target.Equal(that1.Target) {
		return false
	}
	if !this.Sender.Equal(that1.Sender) {
		return false
	}
	return true
}
func (this *ActorPidRequest) Equal(that interface{}) bool {
	if that == nil {
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
	} else if this == nil {
		return false
	}
	if !this.Pid.Equal(that1.Pid) {
		return false
	}
	return true
}
func (this *MessageBatch) Equal(that interface{}) bool {
	if that == nil {
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
	} else if this == nil {
		return false
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
func (this *Unit) Equal(that interface{}) bool {
	if that == nil {
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
	} else if this == nil {
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

// Client API for Remoting service

type RemotingClient interface {
	Receive(ctx context.Context, opts ...grpc.CallOption) (Remoting_ReceiveClient, error)
}

type remotingClient struct {
	cc *grpc.ClientConn
}

func NewRemotingClient(cc *grpc.ClientConn) RemotingClient {
	return &remotingClient{cc}
}

func (c *remotingClient) Receive(ctx context.Context, opts ...grpc.CallOption) (Remoting_ReceiveClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_Remoting_serviceDesc.Streams[0], c.cc, "/remote.Remoting/Receive", opts...)
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

// Server API for Remoting service

type RemotingServer interface {
	Receive(Remoting_ReceiveServer) error
}

func RegisterRemotingServer(s *grpc.Server, srv RemotingServer) {
	s.RegisterService(&_Remoting_serviceDesc, srv)
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
	Methods:     []grpc.MethodDesc{},
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

func (m *MessageEnvelope) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MessageEnvelope) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.TypeName) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintProtos(dAtA, i, uint64(len(m.TypeName)))
		i += copy(dAtA[i:], m.TypeName)
	}
	if len(m.MessageData) > 0 {
		dAtA[i] = 0x12
		i++
		i = encodeVarintProtos(dAtA, i, uint64(len(m.MessageData)))
		i += copy(dAtA[i:], m.MessageData)
	}
	if m.Target != nil {
		dAtA[i] = 0x1a
		i++
		i = encodeVarintProtos(dAtA, i, uint64(m.Target.Size()))
		n1, err := m.Target.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n1
	}
	if m.Sender != nil {
		dAtA[i] = 0x22
		i++
		i = encodeVarintProtos(dAtA, i, uint64(m.Sender.Size()))
		n2, err := m.Sender.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n2
	}
	return i, nil
}

func (m *ActorPidRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ActorPidRequest) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Name) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintProtos(dAtA, i, uint64(len(m.Name)))
		i += copy(dAtA[i:], m.Name)
	}
	if len(m.Kind) > 0 {
		dAtA[i] = 0x12
		i++
		i = encodeVarintProtos(dAtA, i, uint64(len(m.Kind)))
		i += copy(dAtA[i:], m.Kind)
	}
	return i, nil
}

func (m *ActorPidResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *ActorPidResponse) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Pid != nil {
		dAtA[i] = 0xa
		i++
		i = encodeVarintProtos(dAtA, i, uint64(m.Pid.Size()))
		n3, err := m.Pid.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n3
	}
	return i, nil
}

func (m *MessageBatch) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MessageBatch) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Envelopes) > 0 {
		for _, msg := range m.Envelopes {
			dAtA[i] = 0xa
			i++
			i = encodeVarintProtos(dAtA, i, uint64(msg.Size()))
			n, err := msg.MarshalTo(dAtA[i:])
			if err != nil {
				return 0, err
			}
			i += n
		}
	}
	return i, nil
}

func (m *Unit) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Unit) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	return i, nil
}

func encodeFixed64Protos(dAtA []byte, offset int, v uint64) int {
	dAtA[offset] = uint8(v)
	dAtA[offset+1] = uint8(v >> 8)
	dAtA[offset+2] = uint8(v >> 16)
	dAtA[offset+3] = uint8(v >> 24)
	dAtA[offset+4] = uint8(v >> 32)
	dAtA[offset+5] = uint8(v >> 40)
	dAtA[offset+6] = uint8(v >> 48)
	dAtA[offset+7] = uint8(v >> 56)
	return offset + 8
}
func encodeFixed32Protos(dAtA []byte, offset int, v uint32) int {
	dAtA[offset] = uint8(v)
	dAtA[offset+1] = uint8(v >> 8)
	dAtA[offset+2] = uint8(v >> 16)
	dAtA[offset+3] = uint8(v >> 24)
	return offset + 4
}
func encodeVarintProtos(dAtA []byte, offset int, v uint64) int {
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return offset + 1
}
func (m *MessageEnvelope) Size() (n int) {
	var l int
	_ = l
	l = len(m.TypeName)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	l = len(m.MessageData)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	if m.Target != nil {
		l = m.Target.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	if m.Sender != nil {
		l = m.Sender.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *ActorPidRequest) Size() (n int) {
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
	var l int
	_ = l
	if m.Pid != nil {
		l = m.Pid.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *MessageBatch) Size() (n int) {
	var l int
	_ = l
	if len(m.Envelopes) > 0 {
		for _, e := range m.Envelopes {
			l = e.Size()
			n += 1 + l + sovProtos(uint64(l))
		}
	}
	return n
}

func (m *Unit) Size() (n int) {
	var l int
	_ = l
	return n
}

func sovProtos(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}
func sozProtos(x uint64) (n int) {
	return sovProtos(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *MessageEnvelope) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&MessageEnvelope{`,
		`TypeName:` + fmt.Sprintf("%v", this.TypeName) + `,`,
		`MessageData:` + fmt.Sprintf("%v", this.MessageData) + `,`,
		`Target:` + strings.Replace(fmt.Sprintf("%v", this.Target), "PID", "actor.PID", 1) + `,`,
		`Sender:` + strings.Replace(fmt.Sprintf("%v", this.Sender), "PID", "actor.PID", 1) + `,`,
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
		`}`,
	}, "")
	return s
}
func (this *MessageBatch) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&MessageBatch{`,
		`Envelopes:` + strings.Replace(fmt.Sprintf("%v", this.Envelopes), "MessageEnvelope", "MessageEnvelope", 1) + `,`,
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
func valueToStringProtos(v interface{}) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("*%v", pv)
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
			wire |= (uint64(b) & 0x7F) << shift
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
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field TypeName", wireType)
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
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + intStringLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.TypeName = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
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
				byteLen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if byteLen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + byteLen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.MessageData = append(m.MessageData[:0], dAtA[iNdEx:postIndex]...)
			if m.MessageData == nil {
				m.MessageData = []byte{}
			}
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Target", wireType)
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
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + msglen
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Target == nil {
				m.Target = &actor.PID{}
			}
			if err := m.Target.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
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
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + msglen
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
		default:
			iNdEx = preIndex
			skippy, err := skipProtos(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
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
			wire |= (uint64(b) & 0x7F) << shift
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
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + intStringLen
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
				stringLen |= (uint64(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + intStringLen
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
			wire |= (uint64(b) & 0x7F) << shift
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
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + msglen
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
		default:
			iNdEx = preIndex
			skippy, err := skipProtos(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if skippy < 0 {
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
			wire |= (uint64(b) & 0x7F) << shift
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
				msglen |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthProtos
			}
			postIndex := iNdEx + msglen
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
			wire |= (uint64(b) & 0x7F) << shift
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
			return iNdEx, nil
		case 1:
			iNdEx += 8
			return iNdEx, nil
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
			iNdEx += length
			if length < 0 {
				return 0, ErrInvalidLengthProtos
			}
			return iNdEx, nil
		case 3:
			for {
				var innerWire uint64
				var start int = iNdEx
				for shift := uint(0); ; shift += 7 {
					if shift >= 64 {
						return 0, ErrIntOverflowProtos
					}
					if iNdEx >= l {
						return 0, io.ErrUnexpectedEOF
					}
					b := dAtA[iNdEx]
					iNdEx++
					innerWire |= (uint64(b) & 0x7F) << shift
					if b < 0x80 {
						break
					}
				}
				innerWireType := int(innerWire & 0x7)
				if innerWireType == 4 {
					break
				}
				next, err := skipProtos(dAtA[start:])
				if err != nil {
					return 0, err
				}
				iNdEx = start + next
			}
			return iNdEx, nil
		case 4:
			return iNdEx, nil
		case 5:
			iNdEx += 4
			return iNdEx, nil
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
	}
	panic("unreachable")
}

var (
	ErrInvalidLengthProtos = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowProtos   = fmt.Errorf("proto: integer overflow")
)

func init() { proto.RegisterFile("protos.proto", fileDescriptorProtos) }

var fileDescriptorProtos = []byte{
	// 401 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0x64, 0x51, 0xb1, 0x8e, 0xd3, 0x40,
	0x10, 0xf5, 0x92, 0xc8, 0x9c, 0x37, 0x96, 0x0e, 0xad, 0x90, 0xb0, 0x02, 0x5a, 0x19, 0x57, 0x2e,
	0x38, 0xfb, 0x94, 0x13, 0x48, 0x54, 0xe8, 0x4e, 0x49, 0x91, 0x02, 0x14, 0xad, 0xa0, 0x8e, 0x36,
	0xf6, 0xe0, 0x58, 0xc1, 0xbb, 0xc6, 0xbb, 0x89, 0x94, 0x8e, 0x4f, 0xa0, 0xe5, 0x0f, 0xf8, 0x14,
	0xca, 0x94, 0x94, 0xc4, 0x34, 0x94, 0xf9, 0x04, 0xe4, 0xb5, 0x51, 0x42, 0xae, 0xda, 0x37, 0x6f,
	0xde, 0xdb, 0x37, 0x9a, 0xc1, 0x6e, 0x59, 0x49, 0x2d, 0x55, 0x64, 0x1e, 0x62, 0x57, 0x50, 0x48,
	0x0d, 0xc3, 0xab, 0x2c, 0xd7, 0xcb, 0xf5, 0x22, 0x4a, 0x64, 0x11, 0x67, 0x32, 0x93, 0xb1, 0x69,
	0x2f, 0xd6, 0x1f, 0x4d, 0x65, 0x0a, 0x83, 0x5a, 0xdb, 0xf0, 0xd5, 0x89, 0xfc, 0x56, 0x6d, 0xc5,
	0xaa, 0x92, 0x62, 0xfa, 0xbe, 0x35, 0xf1, 0x44, 0xcb, 0xea, 0x2a, 0x93, 0xb1, 0x01, 0xf1, 0x69,
	0x5c, 0xf0, 0x0d, 0xe1, 0xcb, 0xb7, 0xa0, 0x14, 0xcf, 0x60, 0x22, 0x36, 0xf0, 0x49, 0x96, 0x40,
	0x9e, 0x62, 0x47, 0x6f, 0x4b, 0x98, 0x0b, 0x5e, 0x80, 0x87, 0x7c, 0x14, 0x3a, 0xec, 0xa2, 0x21,
	0xde, 0xf1, 0x02, 0xc8, 0x73, 0xec, 0x16, 0xad, 0x7e, 0x9e, 0x72, 0xcd, 0xbd, 0x07, 0x3e, 0x0a,
	0x5d, 0x36, 0xe8, 0xb8, 0x31, 0xd7, 0x9c, 0x04, 0xd8, 0xd6, 0xbc, 0xca, 0x40, 0x7b, 0x3d, 0x1f,
	0x85, 0x83, 0x11, 0x8e, 0x4c, 0x70, 0x34, 0x9b, 0x8e, 0x59, 0xd7, 0x69, 0x34, 0x0a, 0x44, 0x0a,
	0x95, 0xd7, 0xbf, 0xaf, 0x69, 0x3b, 0xc1, 0x6b, 0x7c, 0x79, 0xdb, 0x90, 0xb3, 0x3c, 0x65, 0xf0,
	0x79, 0x0d, 0x4a, 0x13, 0x82, 0xfb, 0x27, 0x53, 0x19, 0xdc, 0x70, 0xab, 0x5c, 0xa4, 0x66, 0x12,
	0x87, 0x19, 0x1c, 0x5c, 0xe3, 0x47, 0x47, 0xab, 0x2a, 0xa5, 0x50, 0x40, 0x9e, 0xe1, 0x5e, 0x99,
	0xa7, 0xc6, 0xfa, 0x7f, 0x5e, 0x43, 0x07, 0x13, 0xec, 0x76, 0x7b, 0xb8, 0xe3, 0x3a, 0x59, 0x92,
	0x97, 0xd8, 0x81, 0x6e, 0x21, 0xca, 0x43, 0x7e, 0x2f, 0x1c, 0x8c, 0x9e, 0x44, 0xed, 0x6d, 0xa2,
	0xb3, 0x85, 0xb1, 0xa3, 0x32, 0xb0, 0x71, 0xff, 0x83, 0xc8, 0xf5, 0xe8, 0x0d, 0xbe, 0x60, 0x8d,
	0x38, 0x17, 0x19, 0xb9, 0xc1, 0x0f, 0x19, 0x24, 0x90, 0x6f, 0x80, 0x3c, 0x3e, 0xfb, 0xc2, 0x64,
	0x0d, 0xdd, 0x7f, 0x6c, 0x63, 0x0d, 0xac, 0x10, 0x5d, 0xa3, 0xbb, 0x17, 0xbb, 0x3d, 0xb5, 0x7e,
	0xee, 0xa9, 0x75, 0xd8, 0x53, 0xeb, 0x4b, 0x4d, 0xd1, 0xf7, 0x9a, 0xa2, 0x1f, 0x35, 0x45, 0xbb,
	0x9a, 0xa2, 0x5f, 0x35, 0x45, 0x7f, 0x6a, 0x6a, 0x1d, 0x6a, 0x8a, 0xbe, 0xfe, 0xa6, 0xd6, 0xc2,
	0x36, 0xd7, 0xbc, 0xf9, 0x1b, 0x00, 0x00, 0xff, 0xff, 0x55, 0xac, 0xdf, 0x02, 0x4c, 0x02, 0x00,
	0x00,
}
