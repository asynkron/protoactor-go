package cluster

import proto "github.com/gogo/protobuf/proto"
import fmt "fmt"
import math "math"
import _ "github.com/gogo/protobuf/gogoproto"
import actor "github.com/AsynkronIT/protoactor-go/actor"

import bytes "bytes"

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

type TakeOwnership struct {
	Pid  *actor.PID `protobuf:"bytes,1,opt,name=pid" json:"pid,omitempty"`
	Name string     `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
}

func (m *TakeOwnership) Reset()                    { *m = TakeOwnership{} }
func (*TakeOwnership) ProtoMessage()               {}
func (*TakeOwnership) Descriptor() ([]byte, []int) { return fileDescriptorProtos, []int{0} }

func (m *TakeOwnership) GetPid() *actor.PID {
	if m != nil {
		return m.Pid
	}
	return nil
}

type GrainRequest struct {
	Method      string `protobuf:"bytes,1,opt,name=method,proto3" json:"method,omitempty"`
	MessageData []byte `protobuf:"bytes,2,opt,name=message_data,json=messageData,proto3" json:"message_data,omitempty"`
}

func (m *GrainRequest) Reset()                    { *m = GrainRequest{} }
func (*GrainRequest) ProtoMessage()               {}
func (*GrainRequest) Descriptor() ([]byte, []int) { return fileDescriptorProtos, []int{1} }

type GrainResponse struct {
	MessageData []byte `protobuf:"bytes,1,opt,name=message_data,json=messageData,proto3" json:"message_data,omitempty"`
}

func (m *GrainResponse) Reset()                    { *m = GrainResponse{} }
func (*GrainResponse) ProtoMessage()               {}
func (*GrainResponse) Descriptor() ([]byte, []int) { return fileDescriptorProtos, []int{2} }

type GrainErrorResponse struct {
	Err string `protobuf:"bytes,1,opt,name=err,proto3" json:"err,omitempty"`
}

func (m *GrainErrorResponse) Reset()                    { *m = GrainErrorResponse{} }
func (*GrainErrorResponse) ProtoMessage()               {}
func (*GrainErrorResponse) Descriptor() ([]byte, []int) { return fileDescriptorProtos, []int{3} }

func init() {
	proto.RegisterType((*TakeOwnership)(nil), "cluster.TakeOwnership")
	proto.RegisterType((*GrainRequest)(nil), "cluster.GrainRequest")
	proto.RegisterType((*GrainResponse)(nil), "cluster.GrainResponse")
	proto.RegisterType((*GrainErrorResponse)(nil), "cluster.GrainErrorResponse")
}
func (this *TakeOwnership) Equal(that interface{}) bool {
	if that == nil {
		if this == nil {
			return true
		}
		return false
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
	if this.Name != that1.Name {
		return false
	}
	return true
}
func (this *GrainRequest) Equal(that interface{}) bool {
	if that == nil {
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
	} else if this == nil {
		return false
	}
	if this.Method != that1.Method {
		return false
	}
	if !bytes.Equal(this.MessageData, that1.MessageData) {
		return false
	}
	return true
}
func (this *GrainResponse) Equal(that interface{}) bool {
	if that == nil {
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
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
		if this == nil {
			return true
		}
		return false
	} else if this == nil {
		return false
	}
	if this.Err != that1.Err {
		return false
	}
	return true
}
func (m *TakeOwnership) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *TakeOwnership) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if m.Pid != nil {
		dAtA[i] = 0xa
		i++
		i = encodeVarintProtos(dAtA, i, uint64(m.Pid.Size()))
		n1, err := m.Pid.MarshalTo(dAtA[i:])
		if err != nil {
			return 0, err
		}
		i += n1
	}
	if len(m.Name) > 0 {
		dAtA[i] = 0x12
		i++
		i = encodeVarintProtos(dAtA, i, uint64(len(m.Name)))
		i += copy(dAtA[i:], m.Name)
	}
	return i, nil
}

func (m *GrainRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GrainRequest) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Method) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintProtos(dAtA, i, uint64(len(m.Method)))
		i += copy(dAtA[i:], m.Method)
	}
	if len(m.MessageData) > 0 {
		dAtA[i] = 0x12
		i++
		i = encodeVarintProtos(dAtA, i, uint64(len(m.MessageData)))
		i += copy(dAtA[i:], m.MessageData)
	}
	return i, nil
}

func (m *GrainResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GrainResponse) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.MessageData) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintProtos(dAtA, i, uint64(len(m.MessageData)))
		i += copy(dAtA[i:], m.MessageData)
	}
	return i, nil
}

func (m *GrainErrorResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalTo(dAtA)
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GrainErrorResponse) MarshalTo(dAtA []byte) (int, error) {
	var i int
	_ = i
	var l int
	_ = l
	if len(m.Err) > 0 {
		dAtA[i] = 0xa
		i++
		i = encodeVarintProtos(dAtA, i, uint64(len(m.Err)))
		i += copy(dAtA[i:], m.Err)
	}
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
func (m *TakeOwnership) Size() (n int) {
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
	var l int
	_ = l
	l = len(m.Method)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	l = len(m.MessageData)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *GrainResponse) Size() (n int) {
	var l int
	_ = l
	l = len(m.MessageData)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *GrainErrorResponse) Size() (n int) {
	var l int
	_ = l
	l = len(m.Err)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
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
		`Method:` + fmt.Sprintf("%v", this.Method) + `,`,
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
			wire |= (uint64(b) & 0x7F) << shift
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
			wire |= (uint64(b) & 0x7F) << shift
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
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Method", wireType)
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
			m.Method = string(dAtA[iNdEx:postIndex])
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
			wire |= (uint64(b) & 0x7F) << shift
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
			wire |= (uint64(b) & 0x7F) << shift
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
	// 307 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x09, 0x6e, 0x88, 0x02, 0xff, 0x64, 0x90, 0xc1, 0x4a, 0xc3, 0x30,
	0x18, 0xc7, 0x1b, 0x27, 0x93, 0x65, 0x1d, 0x48, 0x0e, 0x32, 0x86, 0x84, 0xb9, 0x83, 0xec, 0xe0,
	0x5a, 0x98, 0xe0, 0x7d, 0x32, 0x91, 0x9e, 0x94, 0xb2, 0xbb, 0xa4, 0x5d, 0x6c, 0xcb, 0x6c, 0x52,
	0x93, 0x14, 0xf1, 0xe6, 0x23, 0xf8, 0x18, 0x3e, 0x8a, 0xc7, 0x1d, 0x3d, 0xda, 0x78, 0xf1, 0xb8,
	0x47, 0x90, 0x7d, 0x2d, 0x32, 0xd8, 0x29, 0xbf, 0xff, 0x97, 0xfc, 0xfe, 0x09, 0xc1, 0x6e, 0xa1,
	0xa4, 0x91, 0xda, 0x83, 0x85, 0x1c, 0xc5, 0x4f, 0xa5, 0x36, 0x5c, 0x0d, 0x26, 0x49, 0x66, 0xd2,
	0x32, 0xf2, 0x62, 0x99, 0xfb, 0x89, 0x4c, 0xa4, 0x0f, 0xfb, 0x51, 0xf9, 0x08, 0x09, 0x02, 0x50,
	0xed, 0x0d, 0xae, 0x76, 0x8e, 0xcf, 0xf4, 0xab, 0x58, 0x29, 0x29, 0x82, 0x45, 0x2d, 0xb1, 0xd8,
	0x48, 0x35, 0x49, 0xa4, 0x0f, 0xe0, 0xef, 0xde, 0x37, 0x9a, 0xe1, 0xde, 0x82, 0xad, 0xf8, 0xdd,
	0x8b, 0xe0, 0x4a, 0xa7, 0x59, 0x41, 0x4e, 0x71, 0xab, 0xc8, 0x96, 0x7d, 0x34, 0x44, 0xe3, 0xee,
	0x14, 0x7b, 0xa0, 0x78, 0xf7, 0xc1, 0x3c, 0xdc, 0x8e, 0x09, 0xc1, 0x87, 0x82, 0xe5, 0xbc, 0x7f,
	0x30, 0x44, 0xe3, 0x4e, 0x08, 0x3c, 0x0a, 0xb0, 0x7b, 0xab, 0x58, 0x26, 0x42, 0xfe, 0x5c, 0x72,
	0x6d, 0xc8, 0x09, 0x6e, 0xe7, 0xdc, 0xa4, 0xb2, 0x2e, 0xe9, 0x84, 0x4d, 0x22, 0x67, 0xd8, 0xcd,
	0xb9, 0xd6, 0x2c, 0xe1, 0x0f, 0x4b, 0x66, 0x18, 0x74, 0xb8, 0x61, 0xb7, 0x99, 0xcd, 0x99, 0x61,
	0xa3, 0x29, 0xee, 0x35, 0x55, 0xba, 0x90, 0x42, 0xf3, 0x3d, 0x07, 0xed, 0x3b, 0xe7, 0x98, 0x80,
	0x73, 0xa3, 0x94, 0x54, 0xff, 0xe2, 0x31, 0x6e, 0x71, 0xa5, 0x9a, 0x17, 0x6c, 0xf1, 0xfa, 0x62,
	0x5d, 0x51, 0xe7, 0xab, 0xa2, 0xce, 0xa6, 0xa2, 0xce, 0x9b, 0xa5, 0xe8, 0xc3, 0x52, 0xf4, 0x69,
	0x29, 0x5a, 0x5b, 0x8a, 0xbe, 0x2d, 0x45, 0xbf, 0x96, 0x3a, 0x1b, 0x4b, 0xd1, 0xfb, 0x0f, 0x75,
	0xa2, 0x36, 0x7c, 0xcf, 0xe5, 0x5f, 0x00, 0x00, 0x00, 0xff, 0xff, 0xa7, 0xd0, 0xa5, 0x29, 0x9e,
	0x01, 0x00, 0x00,
}
