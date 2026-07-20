// 网关 gRPC 推送服务
// 游戏服可通过 gRPC 调用网关，向指定房间或单个玩家主动推送消息

package gateway

import (
	"context"

	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
	pb "go-snake-game/pkg/proto/rpc"
)

// 【网关 gRPC 推送服务实现】

// GatewayRpcServer 网关 gRPC 推送服务实现。
// 嵌入 UnimplementedGatewayServiceServer 保证向前兼容，
// 持有 SessionManager 全局单例完成消息推送。
type GatewayRpcServer struct {
	pb.UnimplementedGatewayServiceServer
}

// NewGatewayRpcServer 创建网关 gRPC 推送服务实例。
func NewGatewayRpcServer() *GatewayRpcServer {
	return &GatewayRpcServer{}
}

// PushToRoom 向指定房间所有玩家广播消息。
// 游戏服调用此接口，将消息通过网关的 WebSocket 连接推送给房间内所有玩家。
func (s *GatewayRpcServer) PushToRoom(ctx context.Context, req *pb.PushToRoomRequest) (*pb.PushToRoomResponse, error) {
	roomID := req.GetRoomId()
	msgID := req.GetMsgId()
	body := req.GetBody()

	logger.Info("gRPC PushToRoom 请求", "room_id", roomID, "msg_id", msgID)

	// 参数校验
	if roomID == "" {
		logger.Warn("gRPC PushToRoom 参数无效", "room_id", roomID)
		return &pb.PushToRoomResponse{Code: 1, Msg: "房间 ID 不能为空"}, nil
	}
	if msgID == 0 {
		logger.Warn("gRPC PushToRoom 参数无效", "msg_id", msgID)
		return &pb.PushToRoomResponse{Code: 1, Msg: "消息 ID 不能为空"}, nil
	}

	// 组装自定义二进制 Packet
	pkt := &network.Packet{
		MsgID: uint16(msgID),
		SeqID: 0, // 推送消息不需要序列号
		Body:  body,
	}

	// 调用 SessionManager 的 BroadcastToRoom 广播
	count := GetManager().BroadcastToRoom(roomID, pkt)

	logger.Info("gRPC PushToRoom 成功", "room_id", roomID, "msg_id", msgID, "count", count)
	return &pb.PushToRoomResponse{
		Code:  0,
		Msg:   "推送成功",
		Count: int32(count),
	}, nil
}

// PushToPlayer 向指定单个玩家发送消息。
// 游戏服调用此接口，将消息通过网关的 WebSocket 连接推送给指定玩家。
func (s *GatewayRpcServer) PushToPlayer(ctx context.Context, req *pb.PushToPlayerRequest) (*pb.PushToPlayerResponse, error) {
	playerID := req.GetPlayerId()
	msgID := req.GetMsgId()
	body := req.GetBody()

	logger.Info("gRPC PushToPlayer 请求", "player_id", playerID, "msg_id", msgID)

	// 参数校验
	if playerID == 0 {
		logger.Warn("gRPC PushToPlayer 参数无效", "player_id", playerID)
		return &pb.PushToPlayerResponse{Code: 1, Msg: "玩家 ID 不能为空"}, nil
	}
	if msgID == 0 {
		logger.Warn("gRPC PushToPlayer 参数无效", "msg_id", msgID)
		return &pb.PushToPlayerResponse{Code: 1, Msg: "消息 ID 不能为空"}, nil
	}

	// 查询玩家会话
	session := GetManager().GetSessionByPlayerID(playerID)
	if session == nil {
		logger.Warn("gRPC PushToPlayer 玩家不在线", "player_id", playerID)
		return &pb.PushToPlayerResponse{Code: 2, Msg: "玩家不在线"}, nil
	}

	// 组装自定义二进制 Packet 并发送
	pkt := &network.Packet{
		MsgID: uint16(msgID),
		SeqID: 0, // 推送消息不需要序列号
		Body:  body,
	}
	session.Send(pkt)

	logger.Info("gRPC PushToPlayer 成功", "player_id", playerID, "msg_id", msgID)
	return &pb.PushToPlayerResponse{Code: 0, Msg: "推送成功"}, nil
}
