package router

import (
	fmt "fmt"
	actor "github.com/AsynkronIT/protoactor-go/actor"
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

type AddRoutee struct {
	PID *actor.PID `protobuf:"bytes,1,opt,name=PID,proto3" json:"PID,omitempty"`
}

func (m *AddRoutee) Reset()      { *m = AddRoutee{} }
func (*AddRoutee) ProtoMessage() {}
func (*AddRoutee) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{0}
}
func (m *AddRoutee) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *AddRoutee) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_AddRoutee.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *AddRoutee) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AddRoutee.Merge(m, src)
}
func (m *AddRoutee) XXX_Size() int {
	return m.Size()
}
func (m *AddRoutee) XXX_DiscardUnknown() {
	xxx_messageInfo_AddRoutee.DiscardUnknown(m)
}

var xxx_messageInfo_AddRoutee proto.InternalMessageInfo

func (m *AddRoutee) GetPID() *actor.PID {
	if m != nil {
		return m.PID
	}
	return nil
}

type RemoveRoutee struct {
	PID *actor.PID `protobuf:"bytes,1,opt,name=PID,proto3" json:"PID,omitempty"`
}

func (m *RemoveRoutee) Reset()      { *m = RemoveRoutee{} }
func (*RemoveRoutee) ProtoMessage() {}
func (*RemoveRoutee) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{1}
}
func (m *RemoveRoutee) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *RemoveRoutee) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_RemoveRoutee.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *RemoveRoutee) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RemoveRoutee.Merge(m, src)
}
func (m *RemoveRoutee) XXX_Size() int {
	return m.Size()
}
func (m *RemoveRoutee) XXX_DiscardUnknown() {
	xxx_messageInfo_RemoveRoutee.DiscardUnknown(m)
}

var xxx_messageInfo_RemoveRoutee proto.InternalMessageInfo

func (m *RemoveRoutee) GetPID() *actor.PID {
	if m != nil {
		return m.PID
	}
	return nil
}

type AdjustPoolSize struct {
	Change int32 `protobuf:"varint,1,opt,name=change,proto3" json:"change,omitempty"`
}

func (m *AdjustPoolSize) Reset()      { *m = AdjustPoolSize{} }
func (*AdjustPoolSize) ProtoMessage() {}
func (*AdjustPoolSize) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{2}
}
func (m *AdjustPoolSize) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *AdjustPoolSize) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_AdjustPoolSize.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *AdjustPoolSize) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AdjustPoolSize.Merge(m, src)
}
func (m *AdjustPoolSize) XXX_Size() int {
	return m.Size()
}
func (m *AdjustPoolSize) XXX_DiscardUnknown() {
	xxx_messageInfo_AdjustPoolSize.DiscardUnknown(m)
}

var xxx_messageInfo_AdjustPoolSize proto.InternalMessageInfo

func (m *AdjustPoolSize) GetChange() int32 {
	if m != nil {
		return m.Change
	}
	return 0
}

type GetRoutees struct {
}

func (m *GetRoutees) Reset()      { *m = GetRoutees{} }
func (*GetRoutees) ProtoMessage() {}
func (*GetRoutees) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{3}
}
func (m *GetRoutees) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *GetRoutees) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_GetRoutees.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *GetRoutees) XXX_Merge(src proto.Message) {
	xxx_messageInfo_GetRoutees.Merge(m, src)
}
func (m *GetRoutees) XXX_Size() int {
	return m.Size()
}
func (m *GetRoutees) XXX_DiscardUnknown() {
	xxx_messageInfo_GetRoutees.DiscardUnknown(m)
}

var xxx_messageInfo_GetRoutees proto.InternalMessageInfo

type Routees struct {
	PIDs []*actor.PID `protobuf:"bytes,1,rep,name=PIDs,proto3" json:"PIDs,omitempty"`
}

func (m *Routees) Reset()      { *m = Routees{} }
func (*Routees) ProtoMessage() {}
func (*Routees) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{4}
}
func (m *Routees) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Routees) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Routees.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Routees) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Routees.Merge(m, src)
}
func (m *Routees) XXX_Size() int {
	return m.Size()
}
func (m *Routees) XXX_DiscardUnknown() {
	xxx_messageInfo_Routees.DiscardUnknown(m)
}

var xxx_messageInfo_Routees proto.InternalMessageInfo

func (m *Routees) GetPIDs() []*actor.PID {
	if m != nil {
		return m.PIDs
	}
	return nil
}

func init() {
	proto.RegisterType((*AddRoutee)(nil), "router.AddRoutee")
	proto.RegisterType((*RemoveRoutee)(nil), "router.RemoveRoutee")
	proto.RegisterType((*AdjustPoolSize)(nil), "router.AdjustPoolSize")
	proto.RegisterType((*GetRoutees)(nil), "router.GetRoutees")
	proto.RegisterType((*Routees)(nil), "router.Routees")
}

func init() { proto.RegisterFile("protos.proto", fileDescriptor_5da3cbeb884d181c) }

var fileDescriptor_5da3cbeb884d181c = []byte{
	// 258 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xe2, 0x29, 0x28, 0xca, 0x2f,
	0xc9, 0x2f, 0xd6, 0x03, 0x53, 0x42, 0x6c, 0x45, 0xf9, 0xa5, 0x25, 0xa9, 0x45, 0x52, 0x66, 0xe9,
	0x99, 0x25, 0x19, 0xa5, 0x49, 0x7a, 0xc9, 0xf9, 0xb9, 0xfa, 0x8e, 0xc5, 0x95, 0x79, 0xd9, 0x45,
	0xf9, 0x79, 0x9e, 0x21, 0xfa, 0x60, 0x45, 0x89, 0xc9, 0x25, 0xf9, 0x45, 0xba, 0xe9, 0xf9, 0xfa,
	0x60, 0x86, 0x3e, 0xb2, 0x7e, 0x25, 0x4d, 0x2e, 0x4e, 0xc7, 0x94, 0x94, 0x20, 0x90, 0x21, 0xa9,
	0x42, 0x32, 0x5c, 0xcc, 0x01, 0x9e, 0x2e, 0x12, 0x8c, 0x0a, 0x8c, 0x1a, 0xdc, 0x46, 0x5c, 0x7a,
	0x60, 0xe5, 0x7a, 0x01, 0x9e, 0x2e, 0x41, 0x20, 0x61, 0x25, 0x1d, 0x2e, 0x9e, 0xa0, 0xd4, 0xdc,
	0xfc, 0xb2, 0x54, 0xa2, 0x54, 0x6b, 0x70, 0xf1, 0x39, 0xa6, 0x64, 0x95, 0x16, 0x97, 0x04, 0xe4,
	0xe7, 0xe7, 0x04, 0x67, 0x56, 0xa5, 0x0a, 0x89, 0x71, 0xb1, 0x25, 0x67, 0x24, 0xe6, 0xa5, 0xa7,
	0x82, 0xb5, 0xb0, 0x06, 0x41, 0x79, 0x4a, 0x3c, 0x5c, 0x5c, 0xee, 0xa9, 0x25, 0x10, 0x43, 0x8b,
	0x95, 0x34, 0xb9, 0xd8, 0xa1, 0x4c, 0x21, 0x39, 0x2e, 0x96, 0x00, 0x4f, 0x97, 0x62, 0x09, 0x46,
	0x05, 0x66, 0x34, 0x1b, 0xc0, 0xe2, 0x4e, 0x26, 0x17, 0x1e, 0xca, 0x31, 0xdc, 0x78, 0x28, 0xc7,
	0xf0, 0xe1, 0xa1, 0x1c, 0x63, 0xc3, 0x23, 0x39, 0xc6, 0x15, 0x8f, 0xe4, 0x18, 0x4f, 0x3c, 0x92,
	0x63, 0xbc, 0xf0, 0x48, 0x8e, 0xf1, 0xc1, 0x23, 0x39, 0xc6, 0x17, 0x8f, 0xe4, 0x18, 0x3e, 0x3c,
	0x92, 0x63, 0x9c, 0xf0, 0x58, 0x8e, 0xe1, 0xc2, 0x63, 0x39, 0x86, 0x1b, 0x8f, 0xe5, 0x18, 0x92,
	0xd8, 0xc0, 0x1e, 0x37, 0x06, 0x04, 0x00, 0x00, 0xff, 0xff, 0xce, 0x7b, 0xe8, 0x7d, 0x48, 0x01,
	0x00, 0x00,
}

func (this *AddRoutee) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*AddRoutee)
	if !ok {
		that2, ok := that.(AddRoutee)
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
	if !this.PID.Equal(that1.PID) {
		return false
	}
	return true
}
func (this *RemoveRoutee) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*RemoveRoutee)
	if !ok {
		that2, ok := that.(RemoveRoutee)
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
	if !this.PID.Equal(that1.PID) {
		return false
	}
	return true
}
func (this *AdjustPoolSize) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*AdjustPoolSize)
	if !ok {
		that2, ok := that.(AdjustPoolSize)
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
	if this.Change != that1.Change {
		return false
	}
	return true
}
func (this *GetRoutees) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*GetRoutees)
	if !ok {
		that2, ok := that.(GetRoutees)
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
func (this *Routees) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Routees)
	if !ok {
		that2, ok := that.(Routees)
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
	if len(this.PIDs) != len(that1.PIDs) {
		return false
	}
	for i := range this.PIDs {
		if !this.PIDs[i].Equal(that1.PIDs[i]) {
			return false
		}
	}
	return true
}
func (this *AddRoutee) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 5)
	s = append(s, "&router.AddRoutee{")
	if this.PID != nil {
		s = append(s, "PID: "+fmt.Sprintf("%#v", this.PID)+",\n")
	}
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *RemoveRoutee) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 5)
	s = append(s, "&router.RemoveRoutee{")
	if this.PID != nil {
		s = append(s, "PID: "+fmt.Sprintf("%#v", this.PID)+",\n")
	}
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *AdjustPoolSize) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 5)
	s = append(s, "&router.AdjustPoolSize{")
	s = append(s, "Change: "+fmt.Sprintf("%#v", this.Change)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *GetRoutees) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 4)
	s = append(s, "&router.GetRoutees{")
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *Routees) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 5)
	s = append(s, "&router.Routees{")
	if this.PIDs != nil {
		s = append(s, "PIDs: "+fmt.Sprintf("%#v", this.PIDs)+",\n")
	}
	s = append(s, "}")
	return strings.Join(s, "")
}
func valueToGoStringProtos(v interface{}, typ string) string {
	rv := reflect.ValueOf(v)
	if rv.IsNil() {
		return "nil"
	}
	pv := reflect.Indirect(rv).Interface()
	return fmt.Sprintf("func(v %v) *%v { return &v } ( %#v )", typ, typ, pv)
}
func (m *AddRoutee) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *AddRoutee) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *AddRoutee) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.PID != nil {
		{
			size, err := m.PID.MarshalToSizedBuffer(dAtA[:i])
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

func (m *RemoveRoutee) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *RemoveRoutee) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *RemoveRoutee) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.PID != nil {
		{
			size, err := m.PID.MarshalToSizedBuffer(dAtA[:i])
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

func (m *AdjustPoolSize) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *AdjustPoolSize) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *AdjustPoolSize) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Change != 0 {
		i = encodeVarintProtos(dAtA, i, uint64(m.Change))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *GetRoutees) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *GetRoutees) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *GetRoutees) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	return len(dAtA) - i, nil
}

func (m *Routees) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Routees) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Routees) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.PIDs) > 0 {
		for iNdEx := len(m.PIDs) - 1; iNdEx >= 0; iNdEx-- {
			{
				size, err := m.PIDs[iNdEx].MarshalToSizedBuffer(dAtA[:i])
				if err != nil {
					return 0, err
				}
				i -= size
				i = encodeVarintProtos(dAtA, i, uint64(size))
			}
			i--
			dAtA[i] = 0xa
		}
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
func (m *AddRoutee) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.PID != nil {
		l = m.PID.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *RemoveRoutee) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.PID != nil {
		l = m.PID.Size()
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *AdjustPoolSize) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Change != 0 {
		n += 1 + sovProtos(uint64(m.Change))
	}
	return n
}

func (m *GetRoutees) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	return n
}

func (m *Routees) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.PIDs) > 0 {
		for _, e := range m.PIDs {
			l = e.Size()
			n += 1 + l + sovProtos(uint64(l))
		}
	}
	return n
}

func sovProtos(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozProtos(x uint64) (n int) {
	return sovProtos(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (this *AddRoutee) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&AddRoutee{`,
		`PID:` + strings.Replace(fmt.Sprintf("%v", this.PID), "PID", "actor.PID", 1) + `,`,
		`}`,
	}, "")
	return s
}
func (this *RemoveRoutee) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&RemoveRoutee{`,
		`PID:` + strings.Replace(fmt.Sprintf("%v", this.PID), "PID", "actor.PID", 1) + `,`,
		`}`,
	}, "")
	return s
}
func (this *AdjustPoolSize) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&AdjustPoolSize{`,
		`Change:` + fmt.Sprintf("%v", this.Change) + `,`,
		`}`,
	}, "")
	return s
}
func (this *GetRoutees) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&GetRoutees{`,
		`}`,
	}, "")
	return s
}
func (this *Routees) String() string {
	if this == nil {
		return "nil"
	}
	repeatedStringForPIDs := "[]*PID{"
	for _, f := range this.PIDs {
		repeatedStringForPIDs += strings.Replace(fmt.Sprintf("%v", f), "PID", "actor.PID", 1) + ","
	}
	repeatedStringForPIDs += "}"
	s := strings.Join([]string{`&Routees{`,
		`PIDs:` + repeatedStringForPIDs + `,`,
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
func (m *AddRoutee) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: AddRoutee: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: AddRoutee: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PID", wireType)
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
			if m.PID == nil {
				m.PID = &actor.PID{}
			}
			if err := m.PID.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
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
func (m *RemoveRoutee) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: RemoveRoutee: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: RemoveRoutee: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PID", wireType)
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
			if m.PID == nil {
				m.PID = &actor.PID{}
			}
			if err := m.PID.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
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
func (m *AdjustPoolSize) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: AdjustPoolSize: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: AdjustPoolSize: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Change", wireType)
			}
			m.Change = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Change |= int32(b&0x7F) << shift
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
func (m *GetRoutees) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: GetRoutees: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: GetRoutees: illegal tag %d (wire type %d)", fieldNum, wire)
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
func (m *Routees) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: Routees: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Routees: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field PIDs", wireType)
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
			m.PIDs = append(m.PIDs, &actor.PID{})
			if err := m.PIDs[len(m.PIDs)-1].Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
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
