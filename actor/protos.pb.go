package actor

import proto "github.com/gogo/protobuf/proto"
import fmt "fmt"
import math "math"
import _ "github.com/gogo/protobuf/gogoproto"

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

type PID struct {
	Address string `protobuf:"bytes,1,opt,name=Address,proto3" json:"Address,omitempty"`
	Id      string `protobuf:"bytes,2,opt,name=Id,proto3" json:"Id,omitempty"`

	p *Process
}

func (m *PID) Reset()                    { *m = PID{} }
func (*PID) ProtoMessage()               {}
func (*PID) Descriptor() ([]byte, []int) { return fileDescriptorProtos, []int{0} }

// user messages
type PoisonPill struct {
}

func (m *PoisonPill) Reset()                    { *m = PoisonPill{} }
func (*PoisonPill) ProtoMessage()               {}
func (*PoisonPill) Descriptor() ([]byte, []int) { return fileDescriptorProtos, []int{1} }

// system messages
type Watch struct {
	Watcher *PID `protobuf:"bytes,1,opt,name=watcher" json:"watcher,omitempty"`
}

func (m *Watch) Reset()                    { *m = Watch{} }
func (*Watch) ProtoMessage()               {}
func (*Watch) Descriptor() ([]byte, []int) { return fileDescriptorProtos, []int{2} }

func (m *Watch) GetWatcher() *PID {
	if m != nil {
		return m.Watcher
	}
	return nil
}

type Unwatch struct {
	Watcher *PID `protobuf:"bytes,1,opt,name=watcher" json:"watcher,omitempty"`
}

func (m *Unwatch) Reset()                    { *m = Unwatch{} }
func (*Unwatch) ProtoMessage()               {}
func (*Unwatch) Descriptor() ([]byte, []int) { return fileDescriptorProtos, []int{3} }

func (m *Unwatch) GetWatcher() *PID {
	if m != nil {
		return m.Watcher
	}
	return nil
}

type Terminated struct {
	Who               *PID `protobuf:"bytes,1,opt,name=who" json:"who,omitempty"`
	AddressTerminated bool `protobuf:"varint,2,opt,name=AddressTerminated,proto3" json:"AddressTerminated,omitempty"`
}

func (m *Terminated) Reset()                    { *m = Terminated{} }
func (*Terminated) ProtoMessage()               {}
func (*Terminated) Descriptor() ([]byte, []int) { return fileDescriptorProtos, []int{4} }

func (m *Terminated) GetWho() *PID {
	if m != nil {
		return m.Who
	}
	return nil
}

type Stop struct {
}

func (m *Stop) Reset()                    { *m = Stop{} }
func (*Stop) ProtoMessage()               {}
func (*Stop) Descriptor() ([]byte, []int) { return fileDescriptorProtos, []int{5} }

func init() {
	proto.RegisterType((*PID)(nil), "actor.PID")
	proto.RegisterType((*PoisonPill)(nil), "actor.PoisonPill")
	proto.RegisterType((*Watch)(nil), "actor.Watch")
	proto.RegisterType((*Unwatch)(nil), "actor.Unwatch")
	proto.RegisterType((*Terminated)(nil), "actor.Terminated")
	proto.RegisterType((*Stop)(nil), "actor.Stop")
}
func (this *PID) Equal(that interface{}) bool {
	if that == nil {
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
	} else if this == nil {
		return false
	}
	return true
}
func (this *Watch) Equal(that interface{}) bool {
	if that == nil {
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
	} else if this == nil {
		return false
	}
	return true
}
func (m *PID) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *PID) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Address) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintProtos(dAtA, i, uint64(len(m.Address)))
		i += copy(dAtA[i:], m.Address)
	}
	if len(m.Id) > 0 {
		dAtA[i] = 0x12
		i++
		i = encodeVarintProtos(dAtA, i, uint64(len(m.Id)))
		i += copy(dAtA[i:], m.Id)
	}
	return i, nil
}

func (m *PoisonPill) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *PoisonPill) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	return i, nil
}

func (m *Watch) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Watch) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Watcher != nil {
		dAtA[i] = 0xa
		i++
		i = encodeVarintProtos(dAtA, i, uint64(m.Watcher.Size()))
		n1, err := m.Watcher.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n1
	}
	return i, nil
}

func (m *Unwatch) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Unwatch) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Watcher != nil {
		dAtA[i] = 0xa
		i++
		i = encodeVarintProtos(dAtA, i, uint64(m.Watcher.Size()))
		n2, err := m.Watcher.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n2
	}
	return i, nil
}

func (m *Terminated) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Terminated) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Who != nil {
		dAtA[i] = 0xa
		i++
		i = encodeVarintProtos(dAtA, i, uint64(m.Who.Size()))
		n3, err := m.Who.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n3
	}
	if m.AddressTerminated {
		dAtA[i] = 0x10
		i++
		if m.AddressTerminated {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i++
	}
	return i, nil
}

func (m *Stop) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Stop) MarshalTo(dAtA []byte) (int, error) {
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
func (m *PID) Size() (n int) {
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
	var l int
	_ = l
	return n
}

func (m *Watch) Size() (n int) {
	var l int
	_ = l
	if m.Watcher != nil {
		l = m.Watcher.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *Unwatch) Size() (n int) {
	var l int
	_ = l
	if m.Watcher != nil {
		l = m.Watcher.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *Terminated) Size() (n int) {
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
			wire |= (uint64(b) & 0x7F) << shift
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
			wire |= (uint64(b) & 0x7F) << shift
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
			wire |= (uint64(b) & 0x7F) << shift
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
			wire |= (uint64(b) & 0x7F) << shift
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
			wire |= (uint64(b) & 0x7F) << shift
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
				v |= (int(b) & 0x7F) << shift
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
			wire |= (uint64(b) & 0x7F) << shift
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
	// 275 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0xe2, 0xe2, 0x29, 0x28, 0xca, 0x2f,
	0xc9, 0x2f, 0xd6, 0x03, 0x53, 0x42, 0xac, 0x89, 0xc9, 0x25, 0xf9, 0x45, 0x52, 0xba, 0xe9, 0x99,
	0x25, 0x19, 0xa5, 0x49, 0x7a, 0xc9, 0xf9, 0xb9, 0xfa, 0xe9, 0xf9, 0xe9, 0xf9, 0xfa, 0x60, 0xd9,
	0xa4, 0xd2, 0x34, 0x30, 0x0f, 0xcc, 0x01, 0xb3, 0x20, 0xba, 0x94, 0x4c, 0xb9, 0x98, 0x03, 0x3c,
	0x5d, 0x84, 0x24, 0xb8, 0xd8, 0x1d, 0x53, 0x52, 0x8a, 0x52, 0x8b, 0x8b, 0x25, 0x18, 0x15, 0x18,
	0x35, 0x38, 0x83, 0x60, 0x5c, 0x21, 0x3e, 0x2e, 0x26, 0xcf, 0x14, 0x09, 0x26, 0xb0, 0x20, 0x93,
	0x67, 0x8a, 0x15, 0x4b, 0xc3, 0x1d, 0x05, 0x06, 0x25, 0x1e, 0x2e, 0xae, 0x80, 0xfc, 0xcc, 0xe2,
	0xfc, 0xbc, 0x80, 0xcc, 0x9c, 0x1c, 0x25, 0x5d, 0x2e, 0xd6, 0xf0, 0xc4, 0x92, 0xe4, 0x0c, 0x21,
	0x15, 0x2e, 0xf6, 0x72, 0x10, 0x23, 0xb5, 0x08, 0x6c, 0x0c, 0xb7, 0x11, 0x97, 0x1e, 0xd8, 0x55,
	0x7a, 0x01, 0x9e, 0x2e, 0x41, 0x30, 0x29, 0x25, 0x7d, 0x2e, 0xf6, 0xd0, 0xbc, 0x72, 0x12, 0x34,
	0x44, 0x70, 0x71, 0x85, 0xa4, 0x16, 0xe5, 0x66, 0xe6, 0x25, 0x96, 0xa4, 0xa6, 0x08, 0xc9, 0x70,
	0x31, 0x97, 0x67, 0xe4, 0x63, 0x51, 0x0f, 0x12, 0x16, 0xd2, 0xe1, 0x12, 0x84, 0x3a, 0x1d, 0xa1,
	0x05, 0xec, 0x7c, 0x8e, 0x20, 0x4c, 0x09, 0x25, 0x36, 0x2e, 0x96, 0xe0, 0x92, 0xfc, 0x02, 0x27,
	0x9d, 0x0b, 0x0f, 0xe5, 0x18, 0x6e, 0x3c, 0x94, 0x63, 0xf8, 0xf0, 0x50, 0x8e, 0xa1, 0xe1, 0x91,
	0x1c, 0xe3, 0x8a, 0x47, 0x72, 0x8c, 0x27, 0x1e, 0xc9, 0x31, 0x5e, 0x78, 0x24, 0xc7, 0xf8, 0xe0,
	0x91, 0x1c, 0xe3, 0x8b, 0x47, 0x72, 0x0c, 0x1f, 0x1e, 0xc9, 0x31, 0x4e, 0x78, 0x2c, 0xc7, 0x90,
	0xc4, 0x06, 0x0e, 0x3b, 0x63, 0x40, 0x00, 0x00, 0x00, 0xff, 0xff, 0x5b, 0x17, 0x80, 0x36, 0x81,
	0x01, 0x00, 0x00,
}
