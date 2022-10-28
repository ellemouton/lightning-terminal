// Code generated by protoc-gen-go-grpc. DO NOT EDIT.

package litrpc

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// StatusClient is the client API for Status service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type StatusClient interface {
	GetSubServerState(ctx context.Context, in *GetSubServerStatusReq, opts ...grpc.CallOption) (*GetSubServerStatusResp, error)
}

type statusClient struct {
	cc grpc.ClientConnInterface
}

func NewStatusClient(cc grpc.ClientConnInterface) StatusClient {
	return &statusClient{cc}
}

func (c *statusClient) GetSubServerState(ctx context.Context, in *GetSubServerStatusReq, opts ...grpc.CallOption) (*GetSubServerStatusResp, error) {
	out := new(GetSubServerStatusResp)
	err := c.cc.Invoke(ctx, "/litrpc.Status/GetSubServerState", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StatusServer is the server API for Status service.
// All implementations must embed UnimplementedStatusServer
// for forward compatibility
type StatusServer interface {
	GetSubServerState(context.Context, *GetSubServerStatusReq) (*GetSubServerStatusResp, error)
	mustEmbedUnimplementedStatusServer()
}

// UnimplementedStatusServer must be embedded to have forward compatible implementations.
type UnimplementedStatusServer struct {
}

func (UnimplementedStatusServer) GetSubServerState(context.Context, *GetSubServerStatusReq) (*GetSubServerStatusResp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSubServerState not implemented")
}
func (UnimplementedStatusServer) mustEmbedUnimplementedStatusServer() {}

// UnsafeStatusServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to StatusServer will
// result in compilation errors.
type UnsafeStatusServer interface {
	mustEmbedUnimplementedStatusServer()
}

func RegisterStatusServer(s grpc.ServiceRegistrar, srv StatusServer) {
	s.RegisterService(&Status_ServiceDesc, srv)
}

func _Status_GetSubServerState_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetSubServerStatusReq)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StatusServer).GetSubServerState(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/litrpc.Status/GetSubServerState",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StatusServer).GetSubServerState(ctx, req.(*GetSubServerStatusReq))
	}
	return interceptor(ctx, in, info, handler)
}

// Status_ServiceDesc is the grpc.ServiceDesc for Status service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Status_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "litrpc.Status",
	HandlerType: (*StatusServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetSubServerState",
			Handler:    _Status_GetSubServerState_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "lit-status.proto",
}
