package gateway

import (
	"context"
	"errors"

	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/proto/rpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// 客户端通过websocket连接到网关服务器，网关服务器再通过rpc_client.go 对接客户端 ==> 即gRPC调用登录服务。

// GlobalLoginClient 登录服 gRPC 客户端全局单例，InitLoginRpcClient 调用后初始化。
var GlobalLoginClient *LoginRpcClient

// 业务错误常量，用于区分不同的 gRPC 客户端错误类型
var (
	ErrRpcClientNotInit     = errors.New("gRPC 客户端未初始化")
	ErrRpcServerUnavailable = errors.New("登录服不可用")
	ErrRpcTimeout           = errors.New("gRPC 请求超时")
	ErrRpcUnknown           = errors.New("gRPC 未知错误")
)

// LoginRpcClient 登录服 gRPC 客户端封装。
// 负责与登录服建立 gRPC 连接，并提供注册、登录、Token 校验等业务方法。
type LoginRpcClient struct {
	cc     *grpc.ClientConn       // gRPC 客户端连接，用于管理底层网络连接
	client rpc.LoginServiceClient // 登录服务客户端，由 NewLoginServiceClient 创建
}

// InitLoginRpcClient 初始化登录服 gRPC 客户端。
// 参数 addr: 登录服地址，格式如 "127.0.0.1:9090"
func InitLoginRpcClient(addr string) {
	logger.Info("初始化登录服 gRPC 客户端", "addr", addr)

	// 创建 gRPC 客户端选项
	// insecure.NewCredentials(): 开发环境禁用 TLS
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// 建立 gRPC 连接（新 API，替代 deprecated 的 DialContext）
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		logger.Error("连接登录服失败", "addr", addr, "error", err)
		panic("登录服 gRPC 连接失败: " + err.Error())
	}

	// 创建登录服务客户端
	client := rpc.NewLoginServiceClient(conn)

	// 赋值全局单例
	GlobalLoginClient = &LoginRpcClient{
		cc:     conn,
		client: client,
	}

	logger.Info("登录服 gRPC 客户端初始化成功", "addr", addr)
}

// Register 调用登录服注册接口
func (c *LoginRpcClient) Register(ctx context.Context, username, password string) (*rpc.RegisterResponse, error) {
	// 检查客户端是否已初始化
	if c == nil {
		return nil, ErrRpcClientNotInit
	}

	// 记录请求日志，便于追踪
	logger.Info("gRPC Register 请求", "username", username)

	// 调用 gRPC 接口
	resp, err := c.client.Register(ctx, &rpc.RegisterRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		// 将 gRPC 错误转换为业务错误
		return nil, wrapGrpcError(err)
	}

	// 记录响应日志
	logger.Info("gRPC Register 响应", "username", username, "code", resp.Code, "player_id", resp.PlayerId)
	return resp, nil
}

// Login 调用登录服登录接口。
func (c *LoginRpcClient) Login(ctx context.Context, username, password string) (*rpc.LoginResponse, error) {
	if c == nil {
		return nil, ErrRpcClientNotInit
	}

	logger.Info("gRPC Login 请求", "username", username)

	resp, err := c.client.Login(ctx, &rpc.LoginRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		return nil, wrapGrpcError(err)
	}

	logger.Info("gRPC Login 响应", "username", username, "code", resp.Code, "player_id", resp.PlayerId)
	return resp, nil
}

// VerifyToken 调用登录服 Token 校验接口
func (c *LoginRpcClient) VerifyToken(ctx context.Context, token string) (*rpc.VerifyTokenResponse, error) {
	if c == nil {
		return nil, ErrRpcClientNotInit
	}

	logger.Info("gRPC VerifyToken 请求", "token", token)

	resp, err := c.client.VerifyToken(ctx, &rpc.VerifyTokenRequest{
		Token: token,
	})
	if err != nil {
		return nil, wrapGrpcError(err)
	}

	logger.Info("gRPC VerifyToken 响应", "token", token, "code", resp.Code, "player_id", resp.PlayerId)
	return resp, nil
}

// wrapGrpcError 将 gRPC 错误转换为业务层易懂的错误
func wrapGrpcError(err error) error {
	// 从错误中提取 gRPC 状态信息
	st, ok := status.FromError(err)
	if !ok {
		// 不是 gRPC 错误，返回未知错误
		return ErrRpcUnknown
	}

	// 根据 gRPC 状态码返回对应的业务错误
	// code 14: UNAVAILABLE - 服务不可用
	// code 4: DEADLINE_EXCEEDED - 请求超时
	switch st.Code() {
	case 14:
		return ErrRpcServerUnavailable
	case 4:
		return ErrRpcTimeout
	default:
		// 其他错误码，记录日志并返回原始消息
		logger.Error("gRPC 错误", "code", st.Code(), "message", st.Message())
		return errors.New(st.Message())
	}
}
