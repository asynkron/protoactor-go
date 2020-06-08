package actor

import (
	fmt "fmt"
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

func (m *PID) Reset()      { *m = PID{} }
func (*PID) ProtoMessage() {}
func (*PID) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{0}
}
func (m *PID) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *PID) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_PID.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *PID) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PID.Merge(m, src)
}
func (m *PID) XXX_Size() int {
	return m.Size()
}
func (m *PID) XXX_DiscardUnknown() {
	xxx_messageInfo_PID.DiscardUnknown(m)
}

var xxx_messageInfo_PID proto.InternalMessageInfo

func (m *PID) GetAddress() string {
	if m != nil {
		return m.Address
	}
	return ""
}

func (m *PID) GetId() string {
	if m != nil {
		return m.Id
	}
	return ""
}

// user messages
type PoisonPill struct {
}

func (m *PoisonPill) Reset()      { *m = PoisonPill{} }
func (*PoisonPill) ProtoMessage() {}
func (*PoisonPill) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{1}
}
func (m *PoisonPill) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *PoisonPill) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_PoisonPill.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *PoisonPill) XXX_Merge(src proto.Message) {
	xxx_messageInfo_PoisonPill.Merge(m, src)
}
func (m *PoisonPill) XXX_Size() int {
	return m.Size()
}
func (m *PoisonPill) XXX_DiscardUnknown() {
	xxx_messageInfo_PoisonPill.DiscardUnknown(m)
}

var xxx_messageInfo_PoisonPill proto.InternalMessageInfo

// system messages
type Watch struct {
	Watcher *PID `protobuf:"bytes,1,opt,name=watcher,proto3" json:"watcher,omitempty"`
}

func (m *Watch) Reset()      { *m = Watch{} }
func (*Watch) ProtoMessage() {}
func (*Watch) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{2}
}
func (m *Watch) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Watch) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Watch.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Watch) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Watch.Merge(m, src)
}
func (m *Watch) XXX_Size() int {
	return m.Size()
}
func (m *Watch) XXX_DiscardUnknown() {
	xxx_messageInfo_Watch.DiscardUnknown(m)
}

var xxx_messageInfo_Watch proto.InternalMessageInfo

func (m *Watch) GetWatcher() *PID {
	if m != nil {
		return m.Watcher
	}
	return nil
}

type Unwatch struct {
	Watcher *PID `protobuf:"bytes,1,opt,name=watcher,proto3" json:"watcher,omitempty"`
}

func (m *Unwatch) Reset()      { *m = Unwatch{} }
func (*Unwatch) ProtoMessage() {}
func (*Unwatch) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{3}
}
func (m *Unwatch) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Unwatch) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Unwatch.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Unwatch) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Unwatch.Merge(m, src)
}
func (m *Unwatch) XXX_Size() int {
	return m.Size()
}
func (m *Unwatch) XXX_DiscardUnknown() {
	xxx_messageInfo_Unwatch.DiscardUnknown(m)
}

var xxx_messageInfo_Unwatch proto.InternalMessageInfo

func (m *Unwatch) GetWatcher() *PID {
	if m != nil {
		return m.Watcher
	}
	return nil
}

type Terminated struct {
	Who               *PID `protobuf:"bytes,1,opt,name=who,proto3" json:"who,omitempty"`
	AddressTerminated bool `protobuf:"varint,2,opt,name=address_terminated,json=addressTerminated,proto3" json:"address_terminated,omitempty"`
}

func (m *Terminated) Reset()      { *m = Terminated{} }
func (*Terminated) ProtoMessage() {}
func (*Terminated) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{4}
}
func (m *Terminated) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Terminated) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Terminated.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Terminated) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Terminated.Merge(m, src)
}
func (m *Terminated) XXX_Size() int {
	return m.Size()
}
func (m *Terminated) XXX_DiscardUnknown() {
	xxx_messageInfo_Terminated.DiscardUnknown(m)
}

var xxx_messageInfo_Terminated proto.InternalMessageInfo

func (m *Terminated) GetWho() *PID {
	if m != nil {
		return m.Who
	}
	return nil
}

func (m *Terminated) GetAddressTerminated() bool {
	if m != nil {
		return m.AddressTerminated
	}
	return false
}

type Stop struct {
}

func (m *Stop) Reset()      { *m = Stop{} }
func (*Stop) ProtoMessage() {}
func (*Stop) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{5}
}
func (m *Stop) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Stop) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Stop.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Stop) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Stop.Merge(m, src)
}
func (m *Stop) XXX_Size() int {
	return m.Size()
}
func (m *Stop) XXX_DiscardUnknown() {
	xxx_messageInfo_Stop.DiscardUnknown(m)
}

var xxx_messageInfo_Stop proto.InternalMessageInfo

func init() {
	proto.RegisterType((*PID)(nil), "actor.PID")
	proto.RegisterType((*PoisonPill)(nil), "actor.PoisonPill")
	proto.RegisterType((*Watch)(nil), "actor.Watch")
	proto.RegisterType((*Unwatch)(nil), "actor.Unwatch")
	proto.RegisterType((*Terminated)(nil), "actor.Terminated")
	proto.RegisterType((*Stop)(nil), "actor.Stop")
}

func init() { proto.RegisterFile("protos.proto", fileDescriptor_5da3cbeb884d181c) }

var fileDescriptor_5da3cbeb884d181c = []byte{
	// 292 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x29, 0x28, 0xca, 0x2f,
	0xc9, 0x2f, 0xd6, 0x03, 0x53, 0x42, 0xac, 0x89, 0xc9, 0x25, 0xf9, 0x45, 0x52, 0xba, 0xe9, 0x99,
	0x25, 0x19, 0xa5, 0x49, 0x7a, 0xc9, 0xf9, 0xb9, 0xfa, 0xe9, 0xf9, 0xe9, 0xf9, 0xfa, 0x60, 0xd9,
	0xa4, 0xd2, 0x34, 0x30, 0x0f, 0xcc, 0x01, 0xb3, 0x20, 0xba, 0x94, 0x2c, 0xb9, 0x98, 0x03, 0x3c,
	0x5d, 0x84, 0x24, 0xb8, 0xd8, 0x13, 0x53, 0x52, 0x8a, 0x52, 0x8b, 0x8b, 0x25, 0x18, 0x15, 0x18,
	0x35, 0x38, 0x83, 0x60, 0x5c, 0x21, 0x3e, 0x2e, 0xa6, 0xcc, 0x14, 0x09, 0x26, 0xb0, 0x20, 0x53,
	0x66, 0x8a, 0x15, 0xc7, 0x87, 0x85, 0xf2, 0x0c, 0x0d, 0x77, 0x14, 0x18, 0x94, 0x78, 0xb8, 0xb8,
	0x02, 0xf2, 0x33, 0x8b, 0xf3, 0xf3, 0x02, 0x32, 0x73, 0x72, 0x94, 0x74, 0xb9, 0x58, 0xc3, 0x13,
	0x4b, 0x92, 0x33, 0x84, 0x54, 0xb8, 0xd8, 0xcb, 0x41, 0x8c, 0xd4, 0x22, 0xb0, 0x51, 0xdc, 0x46,
	0x5c, 0x7a, 0x60, 0x97, 0xe9, 0x05, 0x78, 0xba, 0x04, 0xc1, 0xa4, 0x94, 0xf4, 0xb9, 0xd8, 0x43,
	0xf3, 0xca, 0x49, 0xd0, 0x10, 0xc9, 0xc5, 0x15, 0x92, 0x5a, 0x94, 0x9b, 0x99, 0x97, 0x58, 0x92,
	0x9a, 0x22, 0x24, 0xc3, 0xc5, 0x5c, 0x9e, 0x91, 0x8f, 0x45, 0x3d, 0x48, 0x58, 0x48, 0x97, 0x4b,
	0x08, 0xea, 0xfc, 0xf8, 0x12, 0xb8, 0x1e, 0xb0, 0x1f, 0x38, 0x82, 0x04, 0xa1, 0x32, 0x08, 0xc3,
	0x94, 0xd8, 0xb8, 0x58, 0x82, 0x4b, 0xf2, 0x0b, 0x9c, 0x4c, 0x2e, 0x3c, 0x94, 0x63, 0xb8, 0xf1,
	0x50, 0x8e, 0xe1, 0xc3, 0x43, 0x39, 0x86, 0x86, 0x47, 0x72, 0x8c, 0x2b, 0x1e, 0xc9, 0x31, 0x9e,
	0x78, 0x24, 0xc7, 0x78, 0xe1, 0x91, 0x1c, 0xe3, 0x83, 0x47, 0x72, 0x8c, 0x2f, 0x1e, 0xc9, 0x31,
	0x7c, 0x78, 0x24, 0xc7, 0x38, 0xe1, 0xb1, 0x1c, 0xc3, 0x85, 0xc7, 0x72, 0x0c, 0x37, 0x1e, 0xcb,
	0x31, 0x24, 0xb1, 0x81, 0x03, 0xd2, 0x18, 0x10, 0x00, 0x00, 0xff, 0xff, 0x01, 0xa9, 0x03, 0x1d,
	0x8e, 0x01, 0x00, 0x00,
}

func (this *PID) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*PID)
	if !ok {
		that2, ok := that.(PID)
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
	if this.Address != that1.Address {
		return false
	}
	if this.Id != that1.Id {
		return false
	}
	return true
}
func (this *PoisonPill) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*PoisonPill)
	if !ok {
		that2, ok := that.(PoisonPill)
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
func (this *Watch) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Watch)
	if !ok {
		that2, ok := that.(Watch)
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
	if !this.Watcher.Equal(that1.Watcher) {
		return false
	}
	return true
}
func (this *Unwatch) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Unwatch)
	if !ok {
		that2, ok := that.(Unwatch)
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
	if !this.Watcher.Equal(that1.Watcher) {
		return false
	}
	return true
}
func (this *Terminated) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Terminated)
	if !ok {
		that2, ok := that.(Terminated)
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
	if !this.Who.Equal(that1.Who) {
		return false
	}
	if this.AddressTerminated != that1.AddressTerminated {
		return false
	}
	return true
}
func (this *Stop) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Stop)
	if !ok {
		that2, ok := that.(Stop)
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
func (m *PID) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *PID) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PID) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Id) > 0 {
		i -= len(m.Id)
		copy(dAtA[i:], m.Id)
		i = encodeVarintProtos(dAtA, i, uint64(len(m.Id)))
		i--
		dAtA[i] = 0x12
	}
	if len(m.Address) > 0 {
		i -= len(m.Address)
		copy(dAtA[i:], m.Address)
		i = encodeVarintProtos(dAtA, i, uint64(len(m.Address)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *PoisonPill) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *PoisonPill) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *PoisonPill) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	return len(dAtA) - i, nil
}

func (m *Watch) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Watch) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Watch) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Watcher != nil {
		{
			size, err := m.Watcher.MarshalToSizedBuffer(dAtA[:i])
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

func (m *Unwatch) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Unwatch) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Unwatch) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Watcher != nil {
		{
			size, err := m.Watcher.MarshalToSizedBuffer(dAtA[:i])
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

func (m *Terminated) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Terminated) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Terminated) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.AddressTerminated {
		i--
		if m.AddressTerminated {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x10
	}
	if m.Who != nil {
		{
			size, err := m.Who.MarshalToSizedBuffer(dAtA[:i])
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

func (m *Stop) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Stop) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Stop) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
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
func (m *PID) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Address)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	l = len(m.Id)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *PoisonPill) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	return n
}

func (m *Watch) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Watcher != nil {
		l = m.Watcher.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *Unwatch) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Watcher != nil {
		l = m.Watcher.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *Terminated) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Who != nil {
		l = m.Who.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	if m.AddressTerminated {
		n += 2
	}
	return n
}

func (m *Stop) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	return n
}

func sovProtos(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozProtos(x uint64) (n int) {
	return sovProtos(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *PoisonPill) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&PoisonPill{`,
		`}`,
	}, "")
	return s
}
func (this *Watch) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Watch{`,
		`Watcher:` + strings.Replace(fmt.Sprintf("%v", this.Watcher), "PID", "PID", 1) + `,`,
		`}`,
	}, "")
	return s
}
func (this *Unwatch) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Unwatch{`,
		`Watcher:` + strings.Replace(fmt.Sprintf("%v", this.Watcher), "PID", "PID", 1) + `,`,
		`}`,
	}, "")
	return s
}
func (this *Terminated) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Terminated{`,
		`Who:` + strings.Replace(fmt.Sprintf("%v", this.Who), "PID", "PID", 1) + `,`,
		`AddressTerminated:` + fmt.Sprintf("%v", this.AddressTerminated) + `,`,
		`}`,
	}, "")
	return s
}
func (this *Stop) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Stop{`,
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
func (m *PID) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: PID: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: PID: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Address", wireType)
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
			m.Address = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Id", wireType)
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
			m.Id = string(dAtA[iNdEx:postIndex])
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
func (m *PoisonPill) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: PoisonPill: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: PoisonPill: illegal tag %d (wire type %d)", fieldNum, wire)
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
func (m *Watch) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: Watch: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Watch: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Watcher", wireType)
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
			if m.Watcher == nil {
				m.Watcher = &PID{}
			}
			if err := m.Watcher.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
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
func (m *Unwatch) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: Unwatch: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Unwatch: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Watcher", wireType)
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
			if m.Watcher == nil {
				m.Watcher = &PID{}
			}
			if err := m.Watcher.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
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
func (m *Terminated) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: Terminated: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Terminated: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Who", wireType)
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
			if m.Who == nil {
				m.Who = &PID{}
			}
			if err := m.Who.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		case 2:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field AddressTerminated", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.AddressTerminated = bool(v != 0)
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
func (m *Stop) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: Stop: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Stop: illegal tag %d (wire type %d)", fieldNum, wire)
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
