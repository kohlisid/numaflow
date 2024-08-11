//
//Copyright 2022 The Numaproj Authors.
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.34.2
// 	protoc        v5.27.2
// source: pkg/apis/proto/mvtxdaemon/mvtxdaemon.proto

package mvtxdaemon

import (
	_ "google.golang.org/genproto/googleapis/api/annotations"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// MonoVertexMetrics is used to provide information about the mono vertex including processing rate.
type MonoVertexMetrics struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	MonoVertex string `protobuf:"bytes,1,opt,name=monoVertex,proto3" json:"monoVertex,omitempty"`
	// Processing rate in the past period of time, 1m, 5m, 15m, default
	ProcessingRates map[string]*wrapperspb.DoubleValue `protobuf:"bytes,2,rep,name=processingRates,proto3" json:"processingRates,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// Pending in the past period of time, 1m, 5m, 15m, default
	Pendings map[string]*wrapperspb.Int64Value `protobuf:"bytes,3,rep,name=pendings,proto3" json:"pendings,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
}

func (x *MonoVertexMetrics) Reset() {
	*x = MonoVertexMetrics{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MonoVertexMetrics) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MonoVertexMetrics) ProtoMessage() {}

func (x *MonoVertexMetrics) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MonoVertexMetrics.ProtoReflect.Descriptor instead.
func (*MonoVertexMetrics) Descriptor() ([]byte, []int) {
	return file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_rawDescGZIP(), []int{0}
}

func (x *MonoVertexMetrics) GetMonoVertex() string {
	if x != nil {
		return x.MonoVertex
	}
	return ""
}

func (x *MonoVertexMetrics) GetProcessingRates() map[string]*wrapperspb.DoubleValue {
	if x != nil {
		return x.ProcessingRates
	}
	return nil
}

func (x *MonoVertexMetrics) GetPendings() map[string]*wrapperspb.Int64Value {
	if x != nil {
		return x.Pendings
	}
	return nil
}

type GetMonoVertexMetricsResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Metrics *MonoVertexMetrics `protobuf:"bytes,1,opt,name=metrics,proto3" json:"metrics,omitempty"`
}

func (x *GetMonoVertexMetricsResponse) Reset() {
	*x = GetMonoVertexMetricsResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetMonoVertexMetricsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetMonoVertexMetricsResponse) ProtoMessage() {}

func (x *GetMonoVertexMetricsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetMonoVertexMetricsResponse.ProtoReflect.Descriptor instead.
func (*GetMonoVertexMetricsResponse) Descriptor() ([]byte, []int) {
	return file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_rawDescGZIP(), []int{1}
}

func (x *GetMonoVertexMetricsResponse) GetMetrics() *MonoVertexMetrics {
	if x != nil {
		return x.Metrics
	}
	return nil
}

var File_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto protoreflect.FileDescriptor

var file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_rawDesc = []byte{
	0x0a, 0x2a, 0x70, 0x6b, 0x67, 0x2f, 0x61, 0x70, 0x69, 0x73, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x2f, 0x6d, 0x76, 0x74, 0x78, 0x64, 0x61, 0x65, 0x6d, 0x6f, 0x6e, 0x2f, 0x6d, 0x76, 0x74, 0x78,
	0x64, 0x61, 0x65, 0x6d, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0a, 0x6d, 0x76,
	0x74, 0x78, 0x64, 0x61, 0x65, 0x6d, 0x6f, 0x6e, 0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1b, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x1a, 0x1e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x75, 0x66, 0x2f, 0x77, 0x72, 0x61, 0x70, 0x70, 0x65, 0x72, 0x73, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x22, 0x96, 0x03, 0x0a, 0x11, 0x4d, 0x6f, 0x6e, 0x6f, 0x56, 0x65, 0x72, 0x74,
	0x65, 0x78, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x12, 0x1e, 0x0a, 0x0a, 0x6d, 0x6f, 0x6e,
	0x6f, 0x56, 0x65, 0x72, 0x74, 0x65, 0x78, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x6d,
	0x6f, 0x6e, 0x6f, 0x56, 0x65, 0x72, 0x74, 0x65, 0x78, 0x12, 0x5c, 0x0a, 0x0f, 0x70, 0x72, 0x6f,
	0x63, 0x65, 0x73, 0x73, 0x69, 0x6e, 0x67, 0x52, 0x61, 0x74, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x32, 0x2e, 0x6d, 0x76, 0x74, 0x78, 0x64, 0x61, 0x65, 0x6d, 0x6f, 0x6e, 0x2e,
	0x4d, 0x6f, 0x6e, 0x6f, 0x56, 0x65, 0x72, 0x74, 0x65, 0x78, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63,
	0x73, 0x2e, 0x50, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73, 0x69, 0x6e, 0x67, 0x52, 0x61, 0x74, 0x65,
	0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x0f, 0x70, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73, 0x69,
	0x6e, 0x67, 0x52, 0x61, 0x74, 0x65, 0x73, 0x12, 0x47, 0x0a, 0x08, 0x70, 0x65, 0x6e, 0x64, 0x69,
	0x6e, 0x67, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2b, 0x2e, 0x6d, 0x76, 0x74, 0x78,
	0x64, 0x61, 0x65, 0x6d, 0x6f, 0x6e, 0x2e, 0x4d, 0x6f, 0x6e, 0x6f, 0x56, 0x65, 0x72, 0x74, 0x65,
	0x78, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x50, 0x65, 0x6e, 0x64, 0x69, 0x6e, 0x67,
	0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x08, 0x70, 0x65, 0x6e, 0x64, 0x69, 0x6e, 0x67, 0x73,
	0x1a, 0x60, 0x0a, 0x14, 0x50, 0x72, 0x6f, 0x63, 0x65, 0x73, 0x73, 0x69, 0x6e, 0x67, 0x52, 0x61,
	0x74, 0x65, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x32, 0x0a, 0x05, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x6f, 0x75, 0x62,
	0x6c, 0x65, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02,
	0x38, 0x01, 0x1a, 0x58, 0x0a, 0x0d, 0x50, 0x65, 0x6e, 0x64, 0x69, 0x6e, 0x67, 0x73, 0x45, 0x6e,
	0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x31, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x49, 0x6e, 0x74, 0x36, 0x34, 0x56, 0x61, 0x6c, 0x75,
	0x65, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x57, 0x0a, 0x1c,
	0x47, 0x65, 0x74, 0x4d, 0x6f, 0x6e, 0x6f, 0x56, 0x65, 0x72, 0x74, 0x65, 0x78, 0x4d, 0x65, 0x74,
	0x72, 0x69, 0x63, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x37, 0x0a, 0x07,
	0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1d, 0x2e,
	0x6d, 0x76, 0x74, 0x78, 0x64, 0x61, 0x65, 0x6d, 0x6f, 0x6e, 0x2e, 0x4d, 0x6f, 0x6e, 0x6f, 0x56,
	0x65, 0x72, 0x74, 0x65, 0x78, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x52, 0x07, 0x6d, 0x65,
	0x74, 0x72, 0x69, 0x63, 0x73, 0x32, 0x8c, 0x01, 0x0a, 0x17, 0x4d, 0x6f, 0x6e, 0x6f, 0x56, 0x65,
	0x72, 0x74, 0x65, 0x78, 0x44, 0x61, 0x65, 0x6d, 0x6f, 0x6e, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63,
	0x65, 0x12, 0x71, 0x0a, 0x14, 0x47, 0x65, 0x74, 0x4d, 0x6f, 0x6e, 0x6f, 0x56, 0x65, 0x72, 0x74,
	0x65, 0x78, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74,
	0x79, 0x1a, 0x28, 0x2e, 0x6d, 0x76, 0x74, 0x78, 0x64, 0x61, 0x65, 0x6d, 0x6f, 0x6e, 0x2e, 0x47,
	0x65, 0x74, 0x4d, 0x6f, 0x6e, 0x6f, 0x56, 0x65, 0x72, 0x74, 0x65, 0x78, 0x4d, 0x65, 0x74, 0x72,
	0x69, 0x63, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x17, 0x82, 0xd3, 0xe4,
	0x93, 0x02, 0x11, 0x12, 0x0f, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x76, 0x31, 0x2f, 0x6d, 0x65, 0x74,
	0x72, 0x69, 0x63, 0x73, 0x42, 0x38, 0x5a, 0x36, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63,
	0x6f, 0x6d, 0x2f, 0x6e, 0x75, 0x6d, 0x61, 0x70, 0x72, 0x6f, 0x6a, 0x2f, 0x6e, 0x75, 0x6d, 0x61,
	0x66, 0x6c, 0x6f, 0x77, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x61, 0x70, 0x69, 0x73, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x2f, 0x6d, 0x76, 0x74, 0x78, 0x64, 0x61, 0x65, 0x6d, 0x6f, 0x6e, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_rawDescOnce sync.Once
	file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_rawDescData = file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_rawDesc
)

func file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_rawDescGZIP() []byte {
	file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_rawDescOnce.Do(func() {
		file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_rawDescData = protoimpl.X.CompressGZIP(file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_rawDescData)
	})
	return file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_rawDescData
}

var file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_goTypes = []any{
	(*MonoVertexMetrics)(nil),            // 0: mvtxdaemon.MonoVertexMetrics
	(*GetMonoVertexMetricsResponse)(nil), // 1: mvtxdaemon.GetMonoVertexMetricsResponse
	nil,                                  // 2: mvtxdaemon.MonoVertexMetrics.ProcessingRatesEntry
	nil,                                  // 3: mvtxdaemon.MonoVertexMetrics.PendingsEntry
	(*wrapperspb.DoubleValue)(nil),       // 4: google.protobuf.DoubleValue
	(*wrapperspb.Int64Value)(nil),        // 5: google.protobuf.Int64Value
	(*emptypb.Empty)(nil),                // 6: google.protobuf.Empty
}
var file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_depIdxs = []int32{
	2, // 0: mvtxdaemon.MonoVertexMetrics.processingRates:type_name -> mvtxdaemon.MonoVertexMetrics.ProcessingRatesEntry
	3, // 1: mvtxdaemon.MonoVertexMetrics.pendings:type_name -> mvtxdaemon.MonoVertexMetrics.PendingsEntry
	0, // 2: mvtxdaemon.GetMonoVertexMetricsResponse.metrics:type_name -> mvtxdaemon.MonoVertexMetrics
	4, // 3: mvtxdaemon.MonoVertexMetrics.ProcessingRatesEntry.value:type_name -> google.protobuf.DoubleValue
	5, // 4: mvtxdaemon.MonoVertexMetrics.PendingsEntry.value:type_name -> google.protobuf.Int64Value
	6, // 5: mvtxdaemon.MonoVertexDaemonService.GetMonoVertexMetrics:input_type -> google.protobuf.Empty
	1, // 6: mvtxdaemon.MonoVertexDaemonService.GetMonoVertexMetrics:output_type -> mvtxdaemon.GetMonoVertexMetricsResponse
	6, // [6:7] is the sub-list for method output_type
	5, // [5:6] is the sub-list for method input_type
	5, // [5:5] is the sub-list for extension type_name
	5, // [5:5] is the sub-list for extension extendee
	0, // [0:5] is the sub-list for field type_name
}

func init() { file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_init() }
func file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_init() {
	if File_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_msgTypes[0].Exporter = func(v any, i int) any {
			switch v := v.(*MonoVertexMetrics); i {
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
		file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_msgTypes[1].Exporter = func(v any, i int) any {
			switch v := v.(*GetMonoVertexMetricsResponse); i {
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
			RawDescriptor: file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_goTypes,
		DependencyIndexes: file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_depIdxs,
		MessageInfos:      file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_msgTypes,
	}.Build()
	File_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto = out.File
	file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_rawDesc = nil
	file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_goTypes = nil
	file_pkg_apis_proto_mvtxdaemon_mvtxdaemon_proto_depIdxs = nil
}
