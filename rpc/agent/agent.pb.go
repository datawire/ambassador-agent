// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.0
// 	protoc        v3.17.3
// source: agent/agent.proto

package agent

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// IngressInfoRequest represents the intercept properties required to resolve ingress info.
type IngressInfoRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Namespace         string `protobuf:"bytes,1,opt,name=namespace,proto3" json:"namespace,omitempty"`
	ServiceId         string `protobuf:"bytes,2,opt,name=service_id,json=serviceId,proto3" json:"service_id,omitempty"`
	ServiceName       string `protobuf:"bytes,3,opt,name=service_name,json=serviceName,proto3" json:"service_name,omitempty"`
	ServicePortName   string `protobuf:"bytes,4,opt,name=service_port_name,json=servicePortName,proto3" json:"service_port_name,omitempty"`
	ServicePortNumber int32  `protobuf:"varint,5,opt,name=service_port_number,json=servicePortNumber,proto3" json:"service_port_number,omitempty"`
}

func (x *IngressInfoRequest) Reset() {
	*x = IngressInfoRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_agent_agent_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *IngressInfoRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IngressInfoRequest) ProtoMessage() {}

func (x *IngressInfoRequest) ProtoReflect() protoreflect.Message {
	mi := &file_agent_agent_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use IngressInfoRequest.ProtoReflect.Descriptor instead.
func (*IngressInfoRequest) Descriptor() ([]byte, []int) {
	return file_agent_agent_proto_rawDescGZIP(), []int{0}
}

func (x *IngressInfoRequest) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *IngressInfoRequest) GetServiceId() string {
	if x != nil {
		return x.ServiceId
	}
	return ""
}

func (x *IngressInfoRequest) GetServiceName() string {
	if x != nil {
		return x.ServiceName
	}
	return ""
}

func (x *IngressInfoRequest) GetServicePortName() string {
	if x != nil {
		return x.ServicePortName
	}
	return ""
}

func (x *IngressInfoRequest) GetServicePortNumber() int32 {
	if x != nil {
		return x.ServicePortNumber
	}
	return 0
}

// IngressInfoResponse represents the ingress properties required to configure a preview url.
type IngressInfoResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	L3Host string `protobuf:"bytes,1,opt,name=l3_host,json=l3Host,proto3" json:"l3_host,omitempty"`
	Port   int32  `protobuf:"varint,2,opt,name=port,proto3" json:"port,omitempty"`
	UseTls bool   `protobuf:"varint,3,opt,name=use_tls,json=useTls,proto3" json:"use_tls,omitempty"`
	L5Host string `protobuf:"bytes,4,opt,name=l5_host,json=l5Host,proto3" json:"l5_host,omitempty"`
}

func (x *IngressInfoResponse) Reset() {
	*x = IngressInfoResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_agent_agent_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *IngressInfoResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*IngressInfoResponse) ProtoMessage() {}

func (x *IngressInfoResponse) ProtoReflect() protoreflect.Message {
	mi := &file_agent_agent_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use IngressInfoResponse.ProtoReflect.Descriptor instead.
func (*IngressInfoResponse) Descriptor() ([]byte, []int) {
	return file_agent_agent_proto_rawDescGZIP(), []int{1}
}

func (x *IngressInfoResponse) GetL3Host() string {
	if x != nil {
		return x.L3Host
	}
	return ""
}

func (x *IngressInfoResponse) GetPort() int32 {
	if x != nil {
		return x.Port
	}
	return 0
}

func (x *IngressInfoResponse) GetUseTls() bool {
	if x != nil {
		return x.UseTls
	}
	return false
}

func (x *IngressInfoResponse) GetL5Host() string {
	if x != nil {
		return x.L5Host
	}
	return ""
}

var File_agent_agent_proto protoreflect.FileDescriptor

var file_agent_agent_proto_rawDesc = []byte{
	0x0a, 0x11, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2f, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x05, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x22, 0xd0, 0x01, 0x0a, 0x12, 0x49,
	0x6e, 0x67, 0x72, 0x65, 0x73, 0x73, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x1c, 0x0a, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x12,
	0x1d, 0x0a, 0x0a, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x09, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x49, 0x64, 0x12, 0x21,
	0x0a, 0x0c, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x4e, 0x61, 0x6d,
	0x65, 0x12, 0x2a, 0x0a, 0x11, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x70, 0x6f, 0x72,
	0x74, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f, 0x73, 0x65,
	0x72, 0x76, 0x69, 0x63, 0x65, 0x50, 0x6f, 0x72, 0x74, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x2e, 0x0a,
	0x13, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x5f, 0x70, 0x6f, 0x72, 0x74, 0x5f, 0x6e, 0x75,
	0x6d, 0x62, 0x65, 0x72, 0x18, 0x05, 0x20, 0x01, 0x28, 0x05, 0x52, 0x11, 0x73, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x50, 0x6f, 0x72, 0x74, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x22, 0x74, 0x0a,
	0x13, 0x49, 0x6e, 0x67, 0x72, 0x65, 0x73, 0x73, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x12, 0x17, 0x0a, 0x07, 0x6c, 0x33, 0x5f, 0x68, 0x6f, 0x73, 0x74, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x6c, 0x33, 0x48, 0x6f, 0x73, 0x74, 0x12, 0x12, 0x0a,
	0x04, 0x70, 0x6f, 0x72, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x04, 0x70, 0x6f, 0x72,
	0x74, 0x12, 0x17, 0x0a, 0x07, 0x75, 0x73, 0x65, 0x5f, 0x74, 0x6c, 0x73, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x08, 0x52, 0x06, 0x75, 0x73, 0x65, 0x54, 0x6c, 0x73, 0x12, 0x17, 0x0a, 0x07, 0x6c, 0x35,
	0x5f, 0x68, 0x6f, 0x73, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x6c, 0x35, 0x48,
	0x6f, 0x73, 0x74, 0x32, 0x50, 0x0a, 0x05, 0x41, 0x67, 0x65, 0x6e, 0x74, 0x12, 0x47, 0x0a, 0x0e,
	0x52, 0x65, 0x73, 0x6f, 0x6c, 0x76, 0x65, 0x49, 0x6e, 0x67, 0x72, 0x65, 0x73, 0x73, 0x12, 0x19,
	0x2e, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x49, 0x6e, 0x67, 0x72, 0x65, 0x73, 0x73, 0x49, 0x6e,
	0x66, 0x6f, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1a, 0x2e, 0x61, 0x67, 0x65, 0x6e,
	0x74, 0x2e, 0x49, 0x6e, 0x67, 0x72, 0x65, 0x73, 0x73, 0x49, 0x6e, 0x66, 0x6f, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x30, 0x5a, 0x2e, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x64, 0x61, 0x74, 0x61, 0x77, 0x69, 0x72, 0x65, 0x2f, 0x61, 0x6d, 0x62,
	0x61, 0x73, 0x73, 0x61, 0x64, 0x6f, 0x72, 0x2d, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x2f, 0x72, 0x70,
	0x63, 0x2f, 0x61, 0x67, 0x65, 0x6e, 0x74, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_agent_agent_proto_rawDescOnce sync.Once
	file_agent_agent_proto_rawDescData = file_agent_agent_proto_rawDesc
)

func file_agent_agent_proto_rawDescGZIP() []byte {
	file_agent_agent_proto_rawDescOnce.Do(func() {
		file_agent_agent_proto_rawDescData = protoimpl.X.CompressGZIP(file_agent_agent_proto_rawDescData)
	})
	return file_agent_agent_proto_rawDescData
}

var file_agent_agent_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_agent_agent_proto_goTypes = []interface{}{
	(*IngressInfoRequest)(nil),  // 0: agent.IngressInfoRequest
	(*IngressInfoResponse)(nil), // 1: agent.IngressInfoResponse
}
var file_agent_agent_proto_depIdxs = []int32{
	0, // 0: agent.Agent.ResolveIngress:input_type -> agent.IngressInfoRequest
	1, // 1: agent.Agent.ResolveIngress:output_type -> agent.IngressInfoResponse
	1, // [1:2] is the sub-list for method output_type
	0, // [0:1] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_agent_agent_proto_init() }
func file_agent_agent_proto_init() {
	if File_agent_agent_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_agent_agent_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*IngressInfoRequest); i {
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
		file_agent_agent_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*IngressInfoResponse); i {
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
			RawDescriptor: file_agent_agent_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_agent_agent_proto_goTypes,
		DependencyIndexes: file_agent_agent_proto_depIdxs,
		MessageInfos:      file_agent_agent_proto_msgTypes,
	}.Build()
	File_agent_agent_proto = out.File
	file_agent_agent_proto_rawDesc = nil
	file_agent_agent_proto_goTypes = nil
	file_agent_agent_proto_depIdxs = nil
}