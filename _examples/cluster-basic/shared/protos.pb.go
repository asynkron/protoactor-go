package shared

import (
	context "context"
	fmt "fmt"
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

type Noop struct {
}

func (m *Noop) Reset()      { *m = Noop{} }
func (*Noop) ProtoMessage() {}
func (*Noop) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{0}
}
func (m *Noop) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *Noop) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_Noop.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *Noop) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Noop.Merge(m, src)
}
func (m *Noop) XXX_Size() int {
	return m.Size()
}
func (m *Noop) XXX_DiscardUnknown() {
	xxx_messageInfo_Noop.DiscardUnknown(m)
}

var xxx_messageInfo_Noop proto.InternalMessageInfo

type NumberRequest struct {
	Number int64 `protobuf:"varint,1,opt,name=number,proto3" json:"number,omitempty"`
}

func (m *NumberRequest) Reset()      { *m = NumberRequest{} }
func (*NumberRequest) ProtoMessage() {}
func (*NumberRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{1}
}
func (m *NumberRequest) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *NumberRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_NumberRequest.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *NumberRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_NumberRequest.Merge(m, src)
}
func (m *NumberRequest) XXX_Size() int {
	return m.Size()
}
func (m *NumberRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_NumberRequest.DiscardUnknown(m)
}

var xxx_messageInfo_NumberRequest proto.InternalMessageInfo

func (m *NumberRequest) GetNumber() int64 {
	if m != nil {
		return m.Number
	}
	return 0
}

type CountResponse struct {
	Number int64 `protobuf:"varint,1,opt,name=number,proto3" json:"number,omitempty"`
}

func (m *CountResponse) Reset()      { *m = CountResponse{} }
func (*CountResponse) ProtoMessage() {}
func (*CountResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{2}
}
func (m *CountResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *CountResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_CountResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *CountResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CountResponse.Merge(m, src)
}
func (m *CountResponse) XXX_Size() int {
	return m.Size()
}
func (m *CountResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_CountResponse.DiscardUnknown(m)
}

var xxx_messageInfo_CountResponse proto.InternalMessageInfo

func (m *CountResponse) GetNumber() int64 {
	if m != nil {
		return m.Number
	}
	return 0
}

type RegisterMessage struct {
	GrainId string `protobuf:"bytes,1,opt,name=grain_id,json=grainId,proto3" json:"grain_id,omitempty"`
}

func (m *RegisterMessage) Reset()      { *m = RegisterMessage{} }
func (*RegisterMessage) ProtoMessage() {}
func (*RegisterMessage) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{3}
}
func (m *RegisterMessage) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *RegisterMessage) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_RegisterMessage.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *RegisterMessage) XXX_Merge(src proto.Message) {
	xxx_messageInfo_RegisterMessage.Merge(m, src)
}
func (m *RegisterMessage) XXX_Size() int {
	return m.Size()
}
func (m *RegisterMessage) XXX_DiscardUnknown() {
	xxx_messageInfo_RegisterMessage.DiscardUnknown(m)
}

var xxx_messageInfo_RegisterMessage proto.InternalMessageInfo

func (m *RegisterMessage) GetGrainId() string {
	if m != nil {
		return m.GrainId
	}
	return ""
}

type TotalsResponse struct {
	Totals map[string]int64 `protobuf:"bytes,1,rep,name=totals,proto3" json:"totals,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"varint,2,opt,name=value,proto3"`
}

func (m *TotalsResponse) Reset()      { *m = TotalsResponse{} }
func (*TotalsResponse) ProtoMessage() {}
func (*TotalsResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_5da3cbeb884d181c, []int{4}
}
func (m *TotalsResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *TotalsResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_TotalsResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *TotalsResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_TotalsResponse.Merge(m, src)
}
func (m *TotalsResponse) XXX_Size() int {
	return m.Size()
}
func (m *TotalsResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_TotalsResponse.DiscardUnknown(m)
}

var xxx_messageInfo_TotalsResponse proto.InternalMessageInfo

func (m *TotalsResponse) GetTotals() map[string]int64 {
	if m != nil {
		return m.Totals
	}
	return nil
}

func init() {
	proto.RegisterType((*Noop)(nil), "shared.Noop")
	proto.RegisterType((*NumberRequest)(nil), "shared.NumberRequest")
	proto.RegisterType((*CountResponse)(nil), "shared.CountResponse")
	proto.RegisterType((*RegisterMessage)(nil), "shared.RegisterMessage")
	proto.RegisterType((*TotalsResponse)(nil), "shared.TotalsResponse")
	proto.RegisterMapType((map[string]int64)(nil), "shared.TotalsResponse.TotalsEntry")
}

func init() { proto.RegisterFile("protos.proto", fileDescriptor_5da3cbeb884d181c) }

var fileDescriptor_5da3cbeb884d181c = []byte{
	// 406 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x9c, 0x52, 0xb1, 0xae, 0x12, 0x41,
	0x14, 0x9d, 0x79, 0xab, 0xfb, 0x9e, 0xf7, 0xbd, 0x27, 0x66, 0xa2, 0x88, 0x14, 0x13, 0xb2, 0x8d,
	0x14, 0x86, 0x02, 0x34, 0x41, 0x62, 0x23, 0x68, 0x88, 0x85, 0x14, 0x2b, 0xbd, 0x19, 0x76, 0x6f,
	0x90, 0xb0, 0xee, 0xac, 0x33, 0xb3, 0x26, 0x74, 0x76, 0xb6, 0x7e, 0x86, 0x1f, 0x60, 0x6b, 0x6f,
	0x49, 0x49, 0x29, 0x4b, 0x63, 0xc9, 0x27, 0x98, 0x1d, 0x16, 0xe2, 0x12, 0x29, 0xb4, 0xda, 0x3d,
	0x27, 0xe7, 0xdc, 0x7b, 0x72, 0xee, 0xc0, 0x55, 0xa2, 0xa4, 0x91, 0xba, 0x65, 0x3f, 0xcc, 0xd5,
	0xef, 0x84, 0xc2, 0xd0, 0x73, 0xe1, 0xc6, 0x48, 0xca, 0xc4, 0x7b, 0x08, 0xd7, 0xa3, 0xf4, 0xfd,
	0x04, 0x95, 0x8f, 0x1f, 0x52, 0xd4, 0x86, 0x55, 0xc1, 0x8d, 0x2d, 0x51, 0xa3, 0x0d, 0xda, 0x74,
	0xfc, 0x02, 0xe5, 0xc2, 0x81, 0x4c, 0x63, 0xe3, 0xa3, 0x4e, 0x64, 0xac, 0xf1, 0xa4, 0xf0, 0x11,
	0x54, 0x7c, 0x9c, 0xce, 0xb4, 0x41, 0xf5, 0x1a, 0xb5, 0x16, 0x53, 0x64, 0x0f, 0xe0, 0x62, 0xaa,
	0xc4, 0x2c, 0x7e, 0x3b, 0x0b, 0xad, 0xf8, 0x96, 0x7f, 0x6e, 0xf1, 0xab, 0xd0, 0xfb, 0x4c, 0xe1,
	0xf6, 0x58, 0x1a, 0x11, 0xe9, 0xc3, 0xe0, 0x1e, 0xb8, 0xc6, 0x32, 0x35, 0xda, 0x70, 0x9a, 0x97,
	0x6d, 0xaf, 0xb5, 0xcb, 0xdc, 0x2a, 0xeb, 0x0a, 0xf8, 0x32, 0x36, 0x6a, 0xe1, 0x17, 0x8e, 0xfa,
	0x53, 0xb8, 0xfc, 0x83, 0x66, 0x77, 0xc0, 0x99, 0xe3, 0xa2, 0xd8, 0x99, 0xff, 0xb2, 0xbb, 0x70,
	0xf3, 0xa3, 0x88, 0x52, 0xac, 0x9d, 0xd9, 0xd0, 0x3b, 0xd0, 0x3b, 0xeb, 0xd2, 0xf6, 0x37, 0x0a,
	0x30, 0x10, 0x51, 0x90, 0x46, 0xc2, 0x48, 0xc5, 0x9e, 0x80, 0xf3, 0x3c, 0x0c, 0xd9, 0xbd, 0xfd,
	0xf2, 0x52, 0x4b, 0xf5, 0x03, 0x5d, 0xea, 0xc4, 0x23, 0xac, 0x07, 0x17, 0x6f, 0xd2, 0x89, 0x51,
	0x22, 0x30, 0xff, 0xec, 0xed, 0x00, 0x0c, 0xd1, 0x0c, 0x52, 0xa5, 0x30, 0x36, 0xec, 0xea, 0xe0,
	0x96, 0x32, 0x39, 0x69, 0x6a, 0x7f, 0xa7, 0x70, 0x3e, 0x56, 0x22, 0x98, 0xa3, 0x62, 0x5d, 0xb8,
	0xde, 0x57, 0x3f, 0xcc, 0xfb, 0x65, 0xf7, 0xf7, 0xae, 0xa3, 0x8b, 0xd4, 0x4b, 0xc3, 0x6d, 0xec,
	0xca, 0x0b, 0x54, 0xff, 0xe7, 0x7d, 0x06, 0xac, 0xaf, 0xa4, 0x08, 0x03, 0xa1, 0x4d, 0x9e, 0x3f,
	0x0f, 0xa8, 0x8f, 0xe2, 0x57, 0xff, 0x7e, 0x43, 0x8f, 0xf4, 0x1f, 0x2f, 0xd7, 0x9c, 0xac, 0xd6,
	0x9c, 0x6c, 0xd7, 0x9c, 0x7e, 0xca, 0x38, 0xfd, 0x9a, 0x71, 0xfa, 0x23, 0xe3, 0x74, 0x99, 0x71,
	0xfa, 0x33, 0xe3, 0xf4, 0x57, 0xc6, 0xc9, 0x36, 0xe3, 0xf4, 0xcb, 0x86, 0x93, 0xe5, 0x86, 0x93,
	0xd5, 0x86, 0x93, 0x89, 0x6b, 0x5f, 0x73, 0xe7, 0x77, 0x00, 0x00, 0x00, 0xff, 0xff, 0xe3, 0x34,
	0x7a, 0x1f, 0xdd, 0x02, 0x00, 0x00,
}

func (this *Noop) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*Noop)
	if !ok {
		that2, ok := that.(Noop)
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
func (this *NumberRequest) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*NumberRequest)
	if !ok {
		that2, ok := that.(NumberRequest)
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
	if this.Number != that1.Number {
		return false
	}
	return true
}
func (this *CountResponse) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*CountResponse)
	if !ok {
		that2, ok := that.(CountResponse)
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
	if this.Number != that1.Number {
		return false
	}
	return true
}
func (this *RegisterMessage) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*RegisterMessage)
	if !ok {
		that2, ok := that.(RegisterMessage)
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
	if this.GrainId != that1.GrainId {
		return false
	}
	return true
}
func (this *TotalsResponse) Equal(that interface{}) bool {
	if that == nil {
		return this == nil
	}

	that1, ok := that.(*TotalsResponse)
	if !ok {
		that2, ok := that.(TotalsResponse)
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
	if len(this.Totals) != len(that1.Totals) {
		return false
	}
	for i := range this.Totals {
		if this.Totals[i] != that1.Totals[i] {
			return false
		}
	}
	return true
}
func (this *Noop) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 4)
	s = append(s, "&shared.Noop{")
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *NumberRequest) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 5)
	s = append(s, "&shared.NumberRequest{")
	s = append(s, "Number: "+fmt.Sprintf("%#v", this.Number)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *CountResponse) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 5)
	s = append(s, "&shared.CountResponse{")
	s = append(s, "Number: "+fmt.Sprintf("%#v", this.Number)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *RegisterMessage) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 5)
	s = append(s, "&shared.RegisterMessage{")
	s = append(s, "GrainId: "+fmt.Sprintf("%#v", this.GrainId)+",\n")
	s = append(s, "}")
	return strings.Join(s, "")
}
func (this *TotalsResponse) GoString() string {
	if this == nil {
		return "nil"
	}
	s := make([]string, 0, 5)
	s = append(s, "&shared.TotalsResponse{")
	keysForTotals := make([]string, 0, len(this.Totals))
	for k, _ := range this.Totals {
		keysForTotals = append(keysForTotals, k)
	}
	github_com_gogo_protobuf_sortkeys.Strings(keysForTotals)
	mapStringForTotals := "map[string]int64{"
	for _, k := range keysForTotals {
		mapStringForTotals += fmt.Sprintf("%#v: %#v,", k, this.Totals[k])
	}
	mapStringForTotals += "}"
	if this.Totals != nil {
		s = append(s, "Totals: "+mapStringForTotals+",\n")
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

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// CalculatorClient is the client API for Calculator service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type CalculatorClient interface {
	Add(ctx context.Context, in *NumberRequest, opts ...grpc.CallOption) (*CountResponse, error)
	Subtract(ctx context.Context, in *NumberRequest, opts ...grpc.CallOption) (*CountResponse, error)
	GetCurrent(ctx context.Context, in *Noop, opts ...grpc.CallOption) (*CountResponse, error)
}

type calculatorClient struct {
	cc *grpc.ClientConn
}

func NewCalculatorClient(cc *grpc.ClientConn) CalculatorClient {
	return &calculatorClient{cc}
}

func (c *calculatorClient) Add(ctx context.Context, in *NumberRequest, opts ...grpc.CallOption) (*CountResponse, error) {
	out := new(CountResponse)
	err := c.cc.Invoke(ctx, "/shared.Calculator/Add", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *calculatorClient) Subtract(ctx context.Context, in *NumberRequest, opts ...grpc.CallOption) (*CountResponse, error) {
	out := new(CountResponse)
	err := c.cc.Invoke(ctx, "/shared.Calculator/Subtract", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *calculatorClient) GetCurrent(ctx context.Context, in *Noop, opts ...grpc.CallOption) (*CountResponse, error) {
	out := new(CountResponse)
	err := c.cc.Invoke(ctx, "/shared.Calculator/GetCurrent", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CalculatorServer is the server API for Calculator service.
type CalculatorServer interface {
	Add(context.Context, *NumberRequest) (*CountResponse, error)
	Subtract(context.Context, *NumberRequest) (*CountResponse, error)
	GetCurrent(context.Context, *Noop) (*CountResponse, error)
}

// UnimplementedCalculatorServer can be embedded to have forward compatible implementations.
type UnimplementedCalculatorServer struct {
}

func (*UnimplementedCalculatorServer) Add(ctx context.Context, req *NumberRequest) (*CountResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Add not implemented")
}
func (*UnimplementedCalculatorServer) Subtract(ctx context.Context, req *NumberRequest) (*CountResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Subtract not implemented")
}
func (*UnimplementedCalculatorServer) GetCurrent(ctx context.Context, req *Noop) (*CountResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetCurrent not implemented")
}

func RegisterCalculatorServer(s *grpc.Server, srv CalculatorServer) {
	s.RegisterService(&_Calculator_serviceDesc, srv)
}

func _Calculator_Add_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NumberRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CalculatorServer).Add(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/shared.Calculator/Add",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CalculatorServer).Add(ctx, req.(*NumberRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Calculator_Subtract_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(NumberRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CalculatorServer).Subtract(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/shared.Calculator/Subtract",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CalculatorServer).Subtract(ctx, req.(*NumberRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _Calculator_GetCurrent_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Noop)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(CalculatorServer).GetCurrent(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/shared.Calculator/GetCurrent",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(CalculatorServer).GetCurrent(ctx, req.(*Noop))
	}
	return interceptor(ctx, in, info, handler)
}

var _Calculator_serviceDesc = grpc.ServiceDesc{
	ServiceName: "shared.Calculator",
	HandlerType: (*CalculatorServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Add",
			Handler:    _Calculator_Add_Handler,
		},
		{
			MethodName: "Subtract",
			Handler:    _Calculator_Subtract_Handler,
		},
		{
			MethodName: "GetCurrent",
			Handler:    _Calculator_GetCurrent_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "protos.proto",
}

// TrackerClient is the client API for Tracker service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type TrackerClient interface {
	RegisterGrain(ctx context.Context, in *RegisterMessage, opts ...grpc.CallOption) (*Noop, error)
	DeregisterGrain(ctx context.Context, in *RegisterMessage, opts ...grpc.CallOption) (*Noop, error)
	BroadcastGetCounts(ctx context.Context, in *Noop, opts ...grpc.CallOption) (*TotalsResponse, error)
}

type trackerClient struct {
	cc *grpc.ClientConn
}

func NewTrackerClient(cc *grpc.ClientConn) TrackerClient {
	return &trackerClient{cc}
}

func (c *trackerClient) RegisterGrain(ctx context.Context, in *RegisterMessage, opts ...grpc.CallOption) (*Noop, error) {
	out := new(Noop)
	err := c.cc.Invoke(ctx, "/shared.Tracker/RegisterGrain", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *trackerClient) DeregisterGrain(ctx context.Context, in *RegisterMessage, opts ...grpc.CallOption) (*Noop, error) {
	out := new(Noop)
	err := c.cc.Invoke(ctx, "/shared.Tracker/DeregisterGrain", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *trackerClient) BroadcastGetCounts(ctx context.Context, in *Noop, opts ...grpc.CallOption) (*TotalsResponse, error) {
	out := new(TotalsResponse)
	err := c.cc.Invoke(ctx, "/shared.Tracker/BroadcastGetCounts", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// TrackerServer is the server API for Tracker service.
type TrackerServer interface {
	RegisterGrain(context.Context, *RegisterMessage) (*Noop, error)
	DeregisterGrain(context.Context, *RegisterMessage) (*Noop, error)
	BroadcastGetCounts(context.Context, *Noop) (*TotalsResponse, error)
}

// UnimplementedTrackerServer can be embedded to have forward compatible implementations.
type UnimplementedTrackerServer struct {
}

func (*UnimplementedTrackerServer) RegisterGrain(ctx context.Context, req *RegisterMessage) (*Noop, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterGrain not implemented")
}
func (*UnimplementedTrackerServer) DeregisterGrain(ctx context.Context, req *RegisterMessage) (*Noop, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeregisterGrain not implemented")
}
func (*UnimplementedTrackerServer) BroadcastGetCounts(ctx context.Context, req *Noop) (*TotalsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method BroadcastGetCounts not implemented")
}

func RegisterTrackerServer(s *grpc.Server, srv TrackerServer) {
	s.RegisterService(&_Tracker_serviceDesc, srv)
}

func _Tracker_RegisterGrain_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TrackerServer).RegisterGrain(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/shared.Tracker/RegisterGrain",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TrackerServer).RegisterGrain(ctx, req.(*RegisterMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _Tracker_DeregisterGrain_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterMessage)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TrackerServer).DeregisterGrain(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/shared.Tracker/DeregisterGrain",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TrackerServer).DeregisterGrain(ctx, req.(*RegisterMessage))
	}
	return interceptor(ctx, in, info, handler)
}

func _Tracker_BroadcastGetCounts_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Noop)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(TrackerServer).BroadcastGetCounts(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/shared.Tracker/BroadcastGetCounts",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(TrackerServer).BroadcastGetCounts(ctx, req.(*Noop))
	}
	return interceptor(ctx, in, info, handler)
}

var _Tracker_serviceDesc = grpc.ServiceDesc{
	ServiceName: "shared.Tracker",
	HandlerType: (*TrackerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "RegisterGrain",
			Handler:    _Tracker_RegisterGrain_Handler,
		},
		{
			MethodName: "DeregisterGrain",
			Handler:    _Tracker_DeregisterGrain_Handler,
		},
		{
			MethodName: "BroadcastGetCounts",
			Handler:    _Tracker_BroadcastGetCounts_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "protos.proto",
}

func (m *Noop) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *Noop) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *Noop) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	return len(dAtA) - i, nil
}

func (m *NumberRequest) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *NumberRequest) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *NumberRequest) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Number != 0 {
		i = encodeVarintProtos(dAtA, i, uint64(m.Number))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *CountResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *CountResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *CountResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Number != 0 {
		i = encodeVarintProtos(dAtA, i, uint64(m.Number))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *RegisterMessage) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *RegisterMessage) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *RegisterMessage) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.GrainId) > 0 {
		i -= len(m.GrainId)
		copy(dAtA[i:], m.GrainId)
		i = encodeVarintProtos(dAtA, i, uint64(len(m.GrainId)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *TotalsResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *TotalsResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *TotalsResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if len(m.Totals) > 0 {
		for k := range m.Totals {
			v := m.Totals[k]
			baseI := i
			i = encodeVarintProtos(dAtA, i, uint64(v))
			i--
			dAtA[i] = 0x10
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
func (m *Noop) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	return n
}

func (m *NumberRequest) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Number != 0 {
		n += 1 + sovProtos(uint64(m.Number))
	}
	return n
}

func (m *CountResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Number != 0 {
		n += 1 + sovProtos(uint64(m.Number))
	}
	return n
}

func (m *RegisterMessage) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.GrainId)
	if l > 0 {
		n += 1 + l + sovProtos(uint64(l))
	}
	return n
}

func (m *TotalsResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if len(m.Totals) > 0 {
		for k, v := range m.Totals {
			_ = k
			_ = v
			mapEntrySize := 1 + len(k) + sovProtos(uint64(len(k))) + 1 + sovProtos(uint64(v))
			n += mapEntrySize + 1 + sovProtos(uint64(mapEntrySize))
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
func (this *Noop) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&Noop{`,
		`}`,
	}, "")
	return s
}
func (this *NumberRequest) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&NumberRequest{`,
		`Number:` + fmt.Sprintf("%v", this.Number) + `,`,
		`}`,
	}, "")
	return s
}
func (this *CountResponse) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&CountResponse{`,
		`Number:` + fmt.Sprintf("%v", this.Number) + `,`,
		`}`,
	}, "")
	return s
}
func (this *RegisterMessage) String() string {
	if this == nil {
		return "nil"
	}
	s := strings.Join([]string{`&RegisterMessage{`,
		`GrainId:` + fmt.Sprintf("%v", this.GrainId) + `,`,
		`}`,
	}, "")
	return s
}
func (this *TotalsResponse) String() string {
	if this == nil {
		return "nil"
	}
	keysForTotals := make([]string, 0, len(this.Totals))
	for k, _ := range this.Totals {
		keysForTotals = append(keysForTotals, k)
	}
	github_com_gogo_protobuf_sortkeys.Strings(keysForTotals)
	mapStringForTotals := "map[string]int64{"
	for _, k := range keysForTotals {
		mapStringForTotals += fmt.Sprintf("%v: %v,", k, this.Totals[k])
	}
	mapStringForTotals += "}"
	s := strings.Join([]string{`&TotalsResponse{`,
		`Totals:` + mapStringForTotals + `,`,
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
func (m *Noop) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: Noop: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: Noop: illegal tag %d (wire type %d)", fieldNum, wire)
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
func (m *NumberRequest) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: NumberRequest: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: NumberRequest: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Number", wireType)
			}
			m.Number = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Number |= int64(b&0x7F) << shift
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
func (m *CountResponse) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: CountResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: CountResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Number", wireType)
			}
			m.Number = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowProtos
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.Number |= int64(b&0x7F) << shift
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
func (m *RegisterMessage) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: RegisterMessage: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: RegisterMessage: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field GrainId", wireType)
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
			m.GrainId = string(dAtA[iNdEx:postIndex])
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
func (m *TotalsResponse) Unmarshal(dAtA []byte) error {
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
			return fmt.Errorf("proto: TotalsResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: TotalsResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Totals", wireType)
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
			if m.Totals == nil {
				m.Totals = make(map[string]int64)
			}
			var mapkey string
			var mapvalue int64
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
					for shift := uint(0); ; shift += 7 {
						if shift >= 64 {
							return ErrIntOverflowProtos
						}
						if iNdEx >= l {
							return io.ErrUnexpectedEOF
						}
						b := dAtA[iNdEx]
						iNdEx++
						mapvalue |= int64(b&0x7F) << shift
						if b < 0x80 {
							break
						}
					}
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
			m.Totals[mapkey] = mapvalue
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
