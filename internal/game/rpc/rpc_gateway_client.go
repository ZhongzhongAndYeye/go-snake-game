// 网关 gRPC 客户端封装
// 游戏服通过此客户端调用网关推送服务，向房间广播或单推消息

package rpc

import (
	"context"
	"time"

	"go-snake-game/pkg/logger"
	pb "go-snake-game/pkg/proto/rpc"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GlobalGatewayClient 网关 gRPC 客户端全局单例，InitGatewayRpcClient 调用后初始化。
var GlobalGatewayClient *GatewayRpcClient

// GatewayRpcClient 网关 gRPC 客户端封装。
// 负责与网关 gRPC 推送服务建立连接，提供房间广播和玩家单推能力。
type GatewayRpcClient struct {
	cc     *grpc.ClientConn        // gRPC 客户端连接
	client pb.GatewayServiceClient // 网关推送服务客户端
}

// InitGatewayRpcClient 初始化网关 gRPC 客户端。
// 参数 addr: 网关 gRPC 推送服务地址，格式如 "127.0.0.1:9000"
func InitGatewayRpcClient(addr string) {
	logger.Info("初始化网关 gRPC 客户端", "addr", addr)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		logger.Error("连接网关推送服务失败", "addr", addr, "error", err)
		panic("网关 gRPC 连接失败: " + err.Error())
	}

	client := pb.NewGatewayServiceClient(conn)

	GlobalGatewayClient = &GatewayRpcClient{
		cc:     conn,
		client: client,
	}

	logger.Info("网关 gRPC 客户端初始化成功", "addr", addr)
}

// BroadcastRoomMsg 向指定房间广播消息。
// 调用网关的 PushToRoom 接口，将消息推送给房间内所有玩家。
// roomID: 目标房间 ID
// msgID: 消息 ID，对应 WebSocket 协议中的 MsgID
// body: 消息体，由调用方序列化后的 protobuf 二进制数据
func (c *GatewayRpcClient) BroadcastRoomMsg(roomID string, msgID uint16, body []byte) {
	if c == nil {
		logger.Error("BroadcastRoomMsg 失败：网关客户端未初始化")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.PushToRoom(ctx, &pb.PushToRoomRequest{
		RoomId: roomID,
		MsgId:  uint32(msgID),
		Body:   body,
	})
	if err != nil {
		logger.Error("BroadcastRoomMsg gRPC 调用失败",
			"room_id", roomID, "msg_id", msgID, "error", err.Error())
		return
	}

	if resp.Code != 0 {
		logger.Warn("BroadcastRoomMsg 业务失败",
			"room_id", roomID, "msg_id", msgID, "code", resp.Code, "msg", resp.Msg)
		return
	}

	logger.Info("BroadcastRoomMsg 成功",
		"room_id", roomID, "msg_id", msgID, "count", resp.Count)
}

// SendPlayerMsg 向指定玩家发送消息。
// 调用网关的 PushToPlayer 接口，将消息推送给单个玩家。
// playerID: 目标玩家 ID
// msgID: 消息 ID，对应 WebSocket 协议中的 MsgID
// body: 消息体，由调用方序列化后的 protobuf 二进制数据
func (c *GatewayRpcClient) SendPlayerMsg(playerID uint64, msgID uint16, body []byte) {
	if c == nil {
		logger.Error("SendPlayerMsg 失败：网关客户端未初始化")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.PushToPlayer(ctx, &pb.PushToPlayerRequest{
		PlayerId: playerID,
		MsgId:    uint32(msgID),
		Body:     body,
	})
	if err != nil {
		logger.Error("SendPlayerMsg gRPC 调用失败",
			"player_id", playerID, "msg_id", msgID, "error", err.Error())
		return
	}

	if resp.Code != 0 {
		logger.Warn("SendPlayerMsg 业务失败",
			"player_id", playerID, "msg_id", msgID, "code", resp.Code, "msg", resp.Msg)
		return
	}

	logger.Info("SendPlayerMsg 成功",
		"player_id", playerID, "msg_id", msgID)
}

// JoinRoom 将玩家加入网关的房间分组。
// 调用网关的 JoinRoom 接口，使玩家能收到该房间的广播消息。
func (c *GatewayRpcClient) JoinRoom(playerID uint64, roomID string) {
	if c == nil {
		logger.Error("JoinRoom 失败：网关客户端未初始化")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := c.client.JoinRoom(ctx, &pb.JoinRoomRequest{
		PlayerId: playerID,
		RoomId:   roomID,
	})
	if err != nil {
		logger.Error("JoinRoom gRPC 调用失败",
			"player_id", playerID, "room_id", roomID, "error", err.Error())
		return
	}

	if resp.Code != 0 {
		logger.Warn("JoinRoom 业务失败",
			"player_id", playerID, "room_id", roomID, "code", resp.Code, "msg", resp.Msg)
		return
	}

	logger.Info("JoinRoom 成功",
		"player_id", playerID, "room_id", roomID)
}
