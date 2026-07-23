// Package rpc 提供网关与后端服务（登录服、游戏服）的 gRPC 通信封装。
// 游戏服可通过 gRPC 调用网关，向指定房间或单个玩家主动推送消息。
// 使用函数变量注入模式避免循环依赖（rpc 包不导入 gateway 或 handler）。

package rpc

import (
	"context"

	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
	pb "go-snake-game/pkg/proto/rpc"
)

// 以下函数变量由 gateway 包在初始化时注入，用于桥接 rpc 包与 gateway 包
var (
	// BroadcastToRoom 向指定房间广播消息，由 gateway 包注入
	BroadcastToRoom func(roomID string, pkt *network.Packet) int

	// SendToPlayer 向指定玩家发送消息，由 gateway 包注入
	SendToPlayer func(playerID uint64, pkt *network.Packet) bool

	// JoinRoom 将玩家加入网关的房间分组，由 gateway 包注入
	JoinRoom func(playerID uint64, roomID string)
)

// GatewayRpcServer 网关 gRPC 推送服务实现。
type GatewayRpcServer struct {
	pb.UnimplementedGatewayServiceServer
}

// NewGatewayRpcServer 创建网关 gRPC 推送服务实例。
func NewGatewayRpcServer() *GatewayRpcServer {
	return &GatewayRpcServer{}
}

// PushToRoom 向指定房间所有玩家广播消息。
func (s *GatewayRpcServer) PushToRoom(ctx context.Context, req *pb.PushToRoomRequest) (*pb.PushToRoomResponse, error) {
	roomID := req.GetRoomId()
	msgID := req.GetMsgId()
	body := req.GetBody()

	logger.Info("gRPC PushToRoom 请求", "room_id", roomID, "msg_id", msgID)

	if roomID == "" {
		return &pb.PushToRoomResponse{Code: 1, Msg: "房间 ID 不能为空"}, nil
	}
	if msgID == 0 {
		return &pb.PushToRoomResponse{Code: 1, Msg: "消息 ID 不能为空"}, nil
	}

	pkt := &network.Packet{
		MsgID: uint16(msgID),
		SeqID: 0,
		Body:  body,
	}

	if BroadcastToRoom == nil {
		return &pb.PushToRoomResponse{Code: 1, Msg: "推送服务未初始化"}, nil
	}
	count := BroadcastToRoom(roomID, pkt)

	logger.Info("gRPC PushToRoom 成功", "room_id", roomID, "msg_id", msgID, "count", count)
	return &pb.PushToRoomResponse{Code: 0, Msg: "推送成功", Count: int32(count)}, nil
}

// PushToPlayer 向指定单个玩家发送消息。
func (s *GatewayRpcServer) PushToPlayer(ctx context.Context, req *pb.PushToPlayerRequest) (*pb.PushToPlayerResponse, error) {
	playerID := req.GetPlayerId()
	msgID := req.GetMsgId()
	body := req.GetBody()

	logger.Info("gRPC PushToPlayer 请求", "player_id", playerID, "msg_id", msgID)

	if playerID == 0 {
		return &pb.PushToPlayerResponse{Code: 1, Msg: "玩家 ID 不能为空"}, nil
	}
	if msgID == 0 {
		return &pb.PushToPlayerResponse{Code: 1, Msg: "消息 ID 不能为空"}, nil
	}

	pkt := &network.Packet{
		MsgID: uint16(msgID),
		SeqID: 0,
		Body:  body,
	}

	if SendToPlayer == nil {
		return &pb.PushToPlayerResponse{Code: 2, Msg: "推送服务未初始化"}, nil
	}
	if !SendToPlayer(playerID, pkt) {
		return &pb.PushToPlayerResponse{Code: 2, Msg: "玩家不在线"}, nil
	}

	logger.Info("gRPC PushToPlayer 成功", "player_id", playerID, "msg_id", msgID)
	return &pb.PushToPlayerResponse{Code: 0, Msg: "推送成功"}, nil
}

// JoinRoom 将玩家加入网关的房间分组。
func (s *GatewayRpcServer) JoinRoom(ctx context.Context, req *pb.JoinRoomRequest) (*pb.JoinRoomResponse, error) {
	playerID := req.GetPlayerId()
	roomID := req.GetRoomId()

	logger.Info("gRPC JoinRoom 请求", "player_id", playerID, "room_id", roomID)

	if playerID == 0 || roomID == "" {
		return &pb.JoinRoomResponse{Code: 1, Msg: "参数无效"}, nil
	}

	if JoinRoom == nil {
		return &pb.JoinRoomResponse{Code: 1, Msg: "推送服务未初始化"}, nil
	}
	JoinRoom(playerID, roomID)

	logger.Info("gRPC JoinRoom 成功", "player_id", playerID, "room_id", roomID)
	return &pb.JoinRoomResponse{Code: 0, Msg: "成功"}, nil
}
