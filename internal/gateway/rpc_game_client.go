package gateway

import (
	"context"

	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/proto/rpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GlobalGameClient 游戏服 gRPC 客户端全局单例，InitGameRpcClient 调用后初始化。
var GlobalGameClient *GameRpcClient

// GameRpcClient 游戏服 gRPC 客户端封装。
// 负责与游戏服建立 gRPC 连接，并提供匹配、取消匹配、房间信息查询等业务方法。
type GameRpcClient struct {
	cc     *grpc.ClientConn      // gRPC 客户端连接，用于管理底层网络连接
	client rpc.GameServiceClient // 游戏服务客户端，由 NewGameServiceClient 创建
}

// InitGameRpcClient 初始化游戏服 gRPC 客户端。
// 参数 addr: 游戏服地址，格式如 "127.0.0.1:9002"
func InitGameRpcClient(addr string) {
	logger.Info("初始化游戏服 gRPC 客户端", "addr", addr)

	// 创建 gRPC 客户端选项（开发环境禁用 TLS）
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	// 建立 gRPC 连接
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		logger.Error("连接游戏服失败", "addr", addr, "error", err)
		panic("游戏服 gRPC 连接失败: " + err.Error())
	}

	// 创建游戏服务客户端
	client := rpc.NewGameServiceClient(conn)

	// 赋值全局单例
	GlobalGameClient = &GameRpcClient{
		cc:     conn,
		client: client,
	}

	logger.Info("游戏服 gRPC 客户端初始化成功", "addr", addr)
}

// StartMatch 调用游戏服发起匹配接口。
func (c *GameRpcClient) StartMatch(ctx context.Context, playerID uint64, nickname string) (*rpc.StartMatchResponse, error) {
	if c == nil {
		return nil, ErrRpcClientNotInit
	}

	logger.Info("gRPC StartMatch 请求", "player_id", playerID)

	resp, err := c.client.StartMatch(ctx, &rpc.StartMatchRequest{
		PlayerId: playerID,
		Nickname: nickname,
	})
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

	resp, err := c.client.CancelMatch(ctx, &rpc.CancelMatchRequest{
		PlayerId: playerID,
	})
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

	resp, err := c.client.GetRoomInfo(ctx, &rpc.GetRoomInfoRequest{
		RoomId: roomID,
	})
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

	resp, err := c.client.PlayerOperation(ctx, &rpc.PlayerOperationRequest{
		PlayerId:  playerID,
		RoomId:    roomID,
		Direction: direction,
	})
	if err != nil {
		return nil, wrapGrpcError(err)
	}

	logger.Info("gRPC PlayerOperation 响应", "player_id", playerID, "code", resp.Code)
	return resp, nil
}

// PlayerOffline 调用游戏服玩家离线通知接口。
// 玩家 WebSocket 断开连接时调用，通知游戏服处理离线逻辑。
// 游戏服会根据玩家当前状态（游戏中/匹配中/无状态）做相应处理。
func (c *GameRpcClient) PlayerOffline(ctx context.Context, playerID uint64, roomID string) (*rpc.PlayerOfflineResponse, error) {
	if c == nil {
		return nil, ErrRpcClientNotInit
	}

	logger.Info("gRPC PlayerOffline 请求", "player_id", playerID, "room_id", roomID)

	resp, err := c.client.PlayerOffline(ctx, &rpc.PlayerOfflineRequest{
		PlayerId: playerID,
		RoomId:   roomID,
	})
	if err != nil {
		return nil, wrapGrpcError(err)
	}

	logger.Info("gRPC PlayerOffline 响应", "player_id", playerID, "code", resp.Code, "msg", resp.Msg)
	return resp, nil
}
