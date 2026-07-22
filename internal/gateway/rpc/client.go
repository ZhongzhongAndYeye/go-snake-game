// 网关 gRPC 客户端封装
// 网关通过此客户端调用登录服和游戏服的 gRPC 接口

package rpc

import (
	"context"
	"errors"

	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/proto/rpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// GlobalLoginClient 登录服 gRPC 客户端全局单例。
var GlobalLoginClient *LoginRpcClient

// GlobalGameClient 游戏服 gRPC 客户端全局单例。
var GlobalGameClient *GameRpcClient

// 业务错误常量
var (
	ErrRpcClientNotInit     = errors.New("gRPC 客户端未初始化")
	ErrRpcServerUnavailable = errors.New("登录服不可用")
	ErrRpcTimeout           = errors.New("gRPC 请求超时")
	ErrRpcUnknown           = errors.New("gRPC 未知错误")
)

// LoginRpcClient 登录服 gRPC 客户端封装。
type LoginRpcClient struct {
	cc     *grpc.ClientConn
	client rpc.LoginServiceClient
}

// GameRpcClient 游戏服 gRPC 客户端封装。
type GameRpcClient struct {
	cc     *grpc.ClientConn
	client rpc.GameServiceClient
}

// InitLoginRpcClient 初始化登录服 gRPC 客户端。
func InitLoginRpcClient(addr string) {
	logger.Info("初始化登录服 gRPC 客户端", "addr", addr)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		logger.Error("连接登录服失败", "addr", addr, "error", err)
		panic("登录服 gRPC 连接失败: " + err.Error())
	}

	client := rpc.NewLoginServiceClient(conn)
	GlobalLoginClient = &LoginRpcClient{
		cc:     conn,
		client: client,
	}

	logger.Info("登录服 gRPC 客户端初始化成功", "addr", addr)
}

// InitGameRpcClient 初始化游戏服 gRPC 客户端。
func InitGameRpcClient(addr string) {
	logger.Info("初始化游戏服 gRPC 客户端", "addr", addr)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		logger.Error("连接游戏服失败", "addr", addr, "error", err)
		panic("游戏服 gRPC 连接失败: " + err.Error())
	}

	client := rpc.NewGameServiceClient(conn)
	GlobalGameClient = &GameRpcClient{
		cc:     conn,
		client: client,
	}

	logger.Info("游戏服 gRPC 客户端初始化成功", "addr", addr)
}

// Register 调用登录服注册接口。
func (c *LoginRpcClient) Register(ctx context.Context, username, password string) (*rpc.RegisterResponse, error) {
	if c == nil {
		return nil, ErrRpcClientNotInit
	}
	logger.Info("gRPC Register 请求", "username", username)
	resp, err := c.client.Register(ctx, &rpc.RegisterRequest{Username: username, Password: password})
	if err != nil {
		return nil, wrapGrpcError(err)
	}
	logger.Info("gRPC Register 响应", "username", username, "code", resp.Code, "player_id", resp.PlayerId)
	return resp, nil
}

// Login 调用登录服登录接口。
func (c *LoginRpcClient) Login(ctx context.Context, username, password string) (*rpc.LoginResponse, error) {
	if c == nil {
		return nil, ErrRpcClientNotInit
	}
	logger.Info("gRPC Login 请求", "username", username)
	resp, err := c.client.Login(ctx, &rpc.LoginRequest{Username: username, Password: password})
	if err != nil {
		return nil, wrapGrpcError(err)
	}
	logger.Info("gRPC Login 响应", "username", username, "code", resp.Code, "player_id", resp.PlayerId)
	return resp, nil
}

// VerifyToken 调用登录服 Token 校验接口。
func (c *LoginRpcClient) VerifyToken(ctx context.Context, token string) (*rpc.VerifyTokenResponse, error) {
	if c == nil {
		return nil, ErrRpcClientNotInit
	}
	logger.Info("gRPC VerifyToken 请求", "token", token)
	resp, err := c.client.VerifyToken(ctx, &rpc.VerifyTokenRequest{Token: token})
	if err != nil {
		return nil, wrapGrpcError(err)
	}
	logger.Info("gRPC VerifyToken 响应", "token", token, "code", resp.Code, "player_id", resp.PlayerId)
	return resp, nil
}

// StartMatch 调用游戏服发起匹配接口。
func (c *GameRpcClient) StartMatch(ctx context.Context, playerID uint64, nickname string) (*rpc.StartMatchResponse, error) {
	if c == nil {
		return nil, ErrRpcClientNotInit
	}
	logger.Info("gRPC StartMatch 请求", "player_id", playerID)
	resp, err := c.client.StartMatch(ctx, &rpc.StartMatchRequest{PlayerId: playerID, Nickname: nickname})
	if err != nil {
		return nil, wrapGrpcError(err)
	}
	logger.Info("gRPC StartMatch 响应", "player_id", playerID, "code", resp.Code, "room_id", resp.RoomId, "is_matched", resp.IsMatched)
	return resp, nil
}

// CancelMatch 调用游戏服取消匹配接口。
func (c *GameRpcClient) CancelMatch(ctx context.Context, playerID uint64) (*rpc.CancelMatchResponse, error) {
	if c == nil {
		return nil, ErrRpcClientNotInit
	}
	logger.Info("gRPC CancelMatch 请求", "player_id", playerID)
	resp, err := c.client.CancelMatch(ctx, &rpc.CancelMatchRequest{PlayerId: playerID})
	if err != nil {
		return nil, wrapGrpcError(err)
	}
	logger.Info("gRPC CancelMatch 响应", "player_id", playerID, "code", resp.Code)
	return resp, nil
}

// GetRoomInfo 调用游戏服获取房间信息接口。
func (c *GameRpcClient) GetRoomInfo(ctx context.Context, roomID string) (*rpc.GetRoomInfoResponse, error) {
	if c == nil {
		return nil, ErrRpcClientNotInit
	}
	logger.Info("gRPC GetRoomInfo 请求", "room_id", roomID)
	resp, err := c.client.GetRoomInfo(ctx, &rpc.GetRoomInfoRequest{RoomId: roomID})
	if err != nil {
		return nil, wrapGrpcError(err)
	}
	logger.Info("gRPC GetRoomInfo 响应", "room_id", roomID, "code", resp.Code, "player_count", len(resp.Players))
	return resp, nil
}

// PlayerOperation 调用游戏服玩家操作接口。
func (c *GameRpcClient) PlayerOperation(ctx context.Context, playerID uint64, roomID string, direction int32) (*rpc.PlayerOperationResponse, error) {
	if c == nil {
		return nil, ErrRpcClientNotInit
	}
	logger.Info("gRPC PlayerOperation 请求", "player_id", playerID, "room_id", roomID, "direction", direction)
	resp, err := c.client.PlayerOperation(ctx, &rpc.PlayerOperationRequest{PlayerId: playerID, RoomId: roomID, Direction: direction})
	if err != nil {
		return nil, wrapGrpcError(err)
	}
	logger.Info("gRPC PlayerOperation 响应", "player_id", playerID, "code", resp.Code)
	return resp, nil
}

// PlayerOffline 调用游戏服玩家离线通知接口。
func (c *GameRpcClient) PlayerOffline(ctx context.Context, playerID uint64, roomID string) (*rpc.PlayerOfflineResponse, error) {
	if c == nil {
		return nil, ErrRpcClientNotInit
	}
	logger.Info("gRPC PlayerOffline 请求", "player_id", playerID, "room_id", roomID)
	resp, err := c.client.PlayerOffline(ctx, &rpc.PlayerOfflineRequest{PlayerId: playerID, RoomId: roomID})
	if err != nil {
		return nil, wrapGrpcError(err)
	}
	logger.Info("gRPC PlayerOffline 响应", "player_id", playerID, "code", resp.Code, "msg", resp.Msg)
	return resp, nil
}

// GetGlobalRank 调用游戏服查询全服排行榜接口。
func (c *GameRpcClient) GetGlobalRank(ctx context.Context) (*rpc.GetGlobalRankResponse, error) {
	if c == nil {
		return nil, ErrRpcClientNotInit
	}
	logger.Info("gRPC GetGlobalRank 请求")
	resp, err := c.client.GetGlobalRank(ctx, &rpc.GetGlobalRankRequest{})
	if err != nil {
		return nil, wrapGrpcError(err)
	}
	logger.Info("gRPC GetGlobalRank 响应", "code", resp.Code, "count", len(resp.List))
	return resp, nil
}

// ClearMatchQueue 调用游戏服清空匹配队列接口。
func (c *GameRpcClient) ClearMatchQueue(ctx context.Context) (*rpc.ClearMatchQueueResponse, error) {
	if c == nil {
		return nil, ErrRpcClientNotInit
	}
	logger.Info("gRPC ClearMatchQueue 请求")
	resp, err := c.client.ClearMatchQueue(ctx, &rpc.ClearMatchQueueRequest{})
	if err != nil {
		return nil, wrapGrpcError(err)
	}
	logger.Info("gRPC ClearMatchQueue 响应", "code", resp.Code, "msg", resp.Msg)
	return resp, nil
}

func wrapGrpcError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return ErrRpcUnknown
	}
	switch st.Code() {
	case 14:
		return ErrRpcServerUnavailable
	case 4:
		return ErrRpcTimeout
	default:
		logger.Error("gRPC 错误", "code", st.Code(), "message", st.Message())
		return errors.New(st.Message())
	}
}
