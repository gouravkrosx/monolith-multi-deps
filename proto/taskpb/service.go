package taskpb

import (
	"context"

	"google.golang.org/grpc"
)

// ----- AuthService -----

type AuthServiceServer interface {
	Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error)
}

func _Auth_Login_Handler(srv any, ctx context.Context, dec func(any) error, ic grpc.UnaryServerInterceptor) (any, error) {
	in := new(LoginRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if ic == nil {
		return srv.(AuthServiceServer).Login(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/taskmanager.v1.AuthService/Login"}
	h := func(ctx context.Context, req any) (any, error) {
		return srv.(AuthServiceServer).Login(ctx, req.(*LoginRequest))
	}
	return ic(ctx, in, info, h)
}

var AuthServiceDesc = grpc.ServiceDesc{
	ServiceName: "taskmanager.v1.AuthService",
	HandlerType: (*AuthServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "Login", Handler: _Auth_Login_Handler},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/task.proto",
}

func RegisterAuthServiceServer(s grpc.ServiceRegistrar, srv AuthServiceServer) {
	s.RegisterService(&AuthServiceDesc, srv)
}

type AuthServiceClient interface {
	Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*LoginResponse, error)
}

type authServiceClient struct{ cc grpc.ClientConnInterface }

func NewAuthServiceClient(cc grpc.ClientConnInterface) AuthServiceClient {
	return &authServiceClient{cc: cc}
}

func (c *authServiceClient) Login(ctx context.Context, in *LoginRequest, opts ...grpc.CallOption) (*LoginResponse, error) {
	out := new(LoginResponse)
	if err := c.cc.Invoke(ctx, "/taskmanager.v1.AuthService/Login", in, out, opts...); err != nil {
		return nil, err
	}
	return out, nil
}

// ----- TaskService -----

type TaskServiceServer interface {
	CreateTask(ctx context.Context, req *CreateTaskRequest) (*Task, error)
	GetTask(ctx context.Context, req *GetTaskRequest) (*Task, error)
	ListTasks(ctx context.Context, req *ListTasksRequest) (*ListTasksResponse, error)
	UpdateTask(ctx context.Context, req *UpdateTaskRequest) (*Task, error)
	DeleteTask(ctx context.Context, req *DeleteTaskRequest) (*DeleteTaskResponse, error)
	AssignTask(ctx context.Context, req *AssignTaskRequest) (*AssignResponse, error)
	UnassignTask(ctx context.Context, req *UnassignTaskRequest) (*AssignResponse, error)
}

func taskHandler[Req any, Resp any](
	method string,
	call func(srv TaskServiceServer, ctx context.Context, in *Req) (*Resp, error),
) grpc.MethodDesc {
	return grpc.MethodDesc{
		MethodName: method,
		Handler: func(srv any, ctx context.Context, dec func(any) error, ic grpc.UnaryServerInterceptor) (any, error) {
			in := new(Req)
			if err := dec(in); err != nil {
				return nil, err
			}
			if ic == nil {
				return call(srv.(TaskServiceServer), ctx, in)
			}
			info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/taskmanager.v1.TaskService/" + method}
			h := func(ctx context.Context, req any) (any, error) {
				return call(srv.(TaskServiceServer), ctx, req.(*Req))
			}
			return ic(ctx, in, info, h)
		},
	}
}

var TaskServiceDesc = grpc.ServiceDesc{
	ServiceName: "taskmanager.v1.TaskService",
	HandlerType: (*TaskServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		taskHandler("CreateTask", func(s TaskServiceServer, ctx context.Context, in *CreateTaskRequest) (*Task, error) {
			return s.CreateTask(ctx, in)
		}),
		taskHandler("GetTask", func(s TaskServiceServer, ctx context.Context, in *GetTaskRequest) (*Task, error) {
			return s.GetTask(ctx, in)
		}),
		taskHandler("ListTasks", func(s TaskServiceServer, ctx context.Context, in *ListTasksRequest) (*ListTasksResponse, error) {
			return s.ListTasks(ctx, in)
		}),
		taskHandler("UpdateTask", func(s TaskServiceServer, ctx context.Context, in *UpdateTaskRequest) (*Task, error) {
			return s.UpdateTask(ctx, in)
		}),
		taskHandler("DeleteTask", func(s TaskServiceServer, ctx context.Context, in *DeleteTaskRequest) (*DeleteTaskResponse, error) {
			return s.DeleteTask(ctx, in)
		}),
		taskHandler("AssignTask", func(s TaskServiceServer, ctx context.Context, in *AssignTaskRequest) (*AssignResponse, error) {
			return s.AssignTask(ctx, in)
		}),
		taskHandler("UnassignTask", func(s TaskServiceServer, ctx context.Context, in *UnassignTaskRequest) (*AssignResponse, error) {
			return s.UnassignTask(ctx, in)
		}),
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/task.proto",
}

func RegisterTaskServiceServer(s grpc.ServiceRegistrar, srv TaskServiceServer) {
	s.RegisterService(&TaskServiceDesc, srv)
}

type TaskServiceClient interface {
	CreateTask(ctx context.Context, in *CreateTaskRequest, opts ...grpc.CallOption) (*Task, error)
	GetTask(ctx context.Context, in *GetTaskRequest, opts ...grpc.CallOption) (*Task, error)
	ListTasks(ctx context.Context, in *ListTasksRequest, opts ...grpc.CallOption) (*ListTasksResponse, error)
	UpdateTask(ctx context.Context, in *UpdateTaskRequest, opts ...grpc.CallOption) (*Task, error)
	DeleteTask(ctx context.Context, in *DeleteTaskRequest, opts ...grpc.CallOption) (*DeleteTaskResponse, error)
	AssignTask(ctx context.Context, in *AssignTaskRequest, opts ...grpc.CallOption) (*AssignResponse, error)
	UnassignTask(ctx context.Context, in *UnassignTaskRequest, opts ...grpc.CallOption) (*AssignResponse, error)
}

type taskServiceClient struct{ cc grpc.ClientConnInterface }

func NewTaskServiceClient(cc grpc.ClientConnInterface) TaskServiceClient {
	return &taskServiceClient{cc: cc}
}

func (c *taskServiceClient) invoke(ctx context.Context, method string, in, out any, opts ...grpc.CallOption) error {
	return c.cc.Invoke(ctx, "/taskmanager.v1.TaskService/"+method, in, out, opts...)
}

func (c *taskServiceClient) CreateTask(ctx context.Context, in *CreateTaskRequest, opts ...grpc.CallOption) (*Task, error) {
	out := new(Task)
	return out, c.invoke(ctx, "CreateTask", in, out, opts...)
}
func (c *taskServiceClient) GetTask(ctx context.Context, in *GetTaskRequest, opts ...grpc.CallOption) (*Task, error) {
	out := new(Task)
	return out, c.invoke(ctx, "GetTask", in, out, opts...)
}
func (c *taskServiceClient) ListTasks(ctx context.Context, in *ListTasksRequest, opts ...grpc.CallOption) (*ListTasksResponse, error) {
	out := new(ListTasksResponse)
	return out, c.invoke(ctx, "ListTasks", in, out, opts...)
}
func (c *taskServiceClient) UpdateTask(ctx context.Context, in *UpdateTaskRequest, opts ...grpc.CallOption) (*Task, error) {
	out := new(Task)
	return out, c.invoke(ctx, "UpdateTask", in, out, opts...)
}
func (c *taskServiceClient) DeleteTask(ctx context.Context, in *DeleteTaskRequest, opts ...grpc.CallOption) (*DeleteTaskResponse, error) {
	out := new(DeleteTaskResponse)
	return out, c.invoke(ctx, "DeleteTask", in, out, opts...)
}
func (c *taskServiceClient) AssignTask(ctx context.Context, in *AssignTaskRequest, opts ...grpc.CallOption) (*AssignResponse, error) {
	out := new(AssignResponse)
	return out, c.invoke(ctx, "AssignTask", in, out, opts...)
}
func (c *taskServiceClient) UnassignTask(ctx context.Context, in *UnassignTaskRequest, opts ...grpc.CallOption) (*AssignResponse, error) {
	out := new(AssignResponse)
	return out, c.invoke(ctx, "UnassignTask", in, out, opts...)
}
