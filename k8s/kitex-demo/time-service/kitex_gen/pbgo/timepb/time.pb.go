// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.26.0
// 	protoc        v3.8.0
// source: time.proto

package timepb

import (
	context "context"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "kitex-demo/time-service/kitex_gen/google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GetTimeReq struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TimeZone string `protobuf:"bytes,1,opt,name=TimeZone,proto3" json:"TimeZone,omitempty"`
}

func (x *GetTimeReq) Reset() {
	*x = GetTimeReq{}
	if protoimpl.UnsafeEnabled {
		mi := &file_time_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetTimeReq) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetTimeReq) ProtoMessage() {}

func (x *GetTimeReq) ProtoReflect() protoreflect.Message {
	mi := &file_time_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetTimeReq.ProtoReflect.Descriptor instead.
func (*GetTimeReq) Descriptor() ([]byte, []int) {
	return file_time_proto_rawDescGZIP(), []int{0}
}

func (x *GetTimeReq) GetTimeZone() string {
	if x != nil {
		return x.TimeZone
	}
	return ""
}

type GetTimeResp struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Time *timestamppb.Timestamp `protobuf:"bytes,1,opt,name=Time,proto3" json:"Time,omitempty"`
}

func (x *GetTimeResp) Reset() {
	*x = GetTimeResp{}
	if protoimpl.UnsafeEnabled {
		mi := &file_time_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetTimeResp) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetTimeResp) ProtoMessage() {}

func (x *GetTimeResp) ProtoReflect() protoreflect.Message {
	mi := &file_time_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetTimeResp.ProtoReflect.Descriptor instead.
func (*GetTimeResp) Descriptor() ([]byte, []int) {
	return file_time_proto_rawDescGZIP(), []int{1}
}

func (x *GetTimeResp) GetTime() *timestamppb.Timestamp {
	if x != nil {
		return x.Time
	}
	return nil
}

var File_time_proto protoreflect.FileDescriptor

var file_time_proto_rawDesc = []byte{
	0x0a, 0x0a, 0x74, 0x69, 0x6d, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x06, 0x74, 0x69,
	0x6d, 0x65, 0x70, 0x62, 0x1a, 0x1f, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x28, 0x0a, 0x0a, 0x47, 0x65, 0x74, 0x54, 0x69, 0x6d, 0x65,
	0x52, 0x65, 0x71, 0x12, 0x1a, 0x0a, 0x08, 0x54, 0x69, 0x6d, 0x65, 0x5a, 0x6f, 0x6e, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x54, 0x69, 0x6d, 0x65, 0x5a, 0x6f, 0x6e, 0x65, 0x22,
	0x3d, 0x0a, 0x0b, 0x47, 0x65, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x52, 0x65, 0x73, 0x70, 0x12, 0x2e,
	0x0a, 0x04, 0x54, 0x69, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54,
	0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x04, 0x54, 0x69, 0x6d, 0x65, 0x32, 0x41,
	0x0a, 0x0b, 0x54, 0x69, 0x6d, 0x65, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x32, 0x0a,
	0x07, 0x47, 0x65, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x12, 0x2e, 0x74, 0x69, 0x6d, 0x65, 0x70,
	0x62, 0x2e, 0x47, 0x65, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x52, 0x65, 0x71, 0x1a, 0x13, 0x2e, 0x74,
	0x69, 0x6d, 0x65, 0x70, 0x62, 0x2e, 0x47, 0x65, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x52, 0x65, 0x73,
	0x70, 0x42, 0x2f, 0x5a, 0x2d, 0x6b, 0x69, 0x74, 0x65, 0x78, 0x2d, 0x64, 0x65, 0x6d, 0x6f, 0x2f,
	0x74, 0x69, 0x6d, 0x65, 0x2d, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2f, 0x6b, 0x69, 0x74,
	0x65, 0x78, 0x5f, 0x67, 0x65, 0x6e, 0x2f, 0x70, 0x62, 0x67, 0x6f, 0x2f, 0x74, 0x69, 0x6d, 0x65,
	0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_time_proto_rawDescOnce sync.Once
	file_time_proto_rawDescData = file_time_proto_rawDesc
)

func file_time_proto_rawDescGZIP() []byte {
	file_time_proto_rawDescOnce.Do(func() {
		file_time_proto_rawDescData = protoimpl.X.CompressGZIP(file_time_proto_rawDescData)
	})
	return file_time_proto_rawDescData
}

var file_time_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_time_proto_goTypes = []interface{}{
	(*GetTimeReq)(nil),            // 0: timepb.GetTimeReq
	(*GetTimeResp)(nil),           // 1: timepb.GetTimeResp
	(*timestamppb.Timestamp)(nil), // 2: google.protobuf.Timestamp
}
var file_time_proto_depIdxs = []int32{
	2, // 0: timepb.GetTimeResp.Time:type_name -> google.protobuf.Timestamp
	0, // 1: timepb.TimeService.GetTime:input_type -> timepb.GetTimeReq
	1, // 2: timepb.TimeService.GetTime:output_type -> timepb.GetTimeResp
	2, // [2:3] is the sub-list for method output_type
	1, // [1:2] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_time_proto_init() }
func file_time_proto_init() {
	if File_time_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_time_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetTimeReq); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_time_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetTimeResp); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_time_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_time_proto_goTypes,
		DependencyIndexes: file_time_proto_depIdxs,
		MessageInfos:      file_time_proto_msgTypes,
	}.Build()
	File_time_proto = out.File
	file_time_proto_rawDesc = nil
	file_time_proto_goTypes = nil
	file_time_proto_depIdxs = nil
}

var _ context.Context

// Code generated by Kitex v0.1.4. DO NOT EDIT.

type TimeService interface {
	GetTime(ctx context.Context, req *GetTimeReq) (res *GetTimeResp, err error)
}