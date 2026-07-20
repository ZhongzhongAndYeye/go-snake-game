package gateway

import (
	"context"
	"time"

	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
	"go-snake-game/pkg/proto/msg"

	"google.golang.org/protobuf/proto"
)

// GameOperationHandler 游戏操作请求处理器。
// 处理客户端发送的方向变更等游戏操作，通过 gRPC 转发至游戏服处理。
func GameOperationHandler(s *Session, packet *network.Packet) {
	playerID := s.PlayerID()
	logger.Info("收到游戏操作请求", "session_id", s.logID(), "player_id", playerID, "seq_id", packet.SeqID)

	// 校验玩家是否已登录
	if playerID == 0 {
		logger.Warn("游戏操作请求未登录", "session_id", s.logID())
		s.SendError(ErrCodeNotLogin, "请先登录")
		return
	}

	// 解析请求体
	req := &msg.GameOperationReq{}
	if err := proto.Unmarshal(packet.Body, req); err != nil {
		logger.Warn("游戏操作请求解析失败", "session_id", s.logID(), "player_id", playerID, "error", err)
		s.SendError(ErrCodeParamError, "请求参数格式错误")
		return
	}

	direction := req.GetDirection()
	logger.Info("游戏操作请求", "session_id", s.logID(), "player_id", playerID, "direction", direction)

	// 创建 gRPC 请求上下文（5秒超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 调用游戏服 gRPC 玩家操作接口
	resp, err := GlobalGameClient.PlayerOperation(ctx, playerID, "", direction)
	if err != nil {
		logger.Error("调用游戏服玩家操作接口失败", "session_id", s.logID(), "player_id", playerID, "error", err)
		s.SendError(ErrCodeSystemError, "操作失败，请稍后重试")
		return
	}

	logger.Info("游戏操作成功", "session_id", s.logID(), "player_id", playerID, "code", resp.Code, "msg", resp.Msg)

	// 发送响应给客户端
	respBody, _ := proto.Marshal(&msg.GameOperationResp{Code: int32(resp.Code), Msg: resp.Msg})
	s.Send(&network.Packet{
		MsgID: network.MsgIDGameOperationReq,
		SeqID: packet.SeqID,
		Body:  respBody,
	})
}

// RoomInfoQueryHandler 房间信息查询请求处理器。
// 客户端查询当前房间的游戏状态，包括蛇位置、食物、帧数等。
func RoomInfoQueryHandler(s *Session, packet *network.Packet) {
	playerID := s.PlayerID()
	logger.Info("收到房间信息查询请求", "session_id", s.logID(), "player_id", playerID, "seq_id", packet.SeqID)

	// 校验玩家是否已登录
	if playerID == 0 {
		logger.Warn("房间信息查询请求未登录", "session_id", s.logID())
		s.SendError(ErrCodeNotLogin, "请先登录")
		return
	}

	// 解析请求体
	req := &msg.RoomInfoQueryReq{}
	roomID := ""
	if len(packet.Body) > 0 {
		if err := proto.Unmarshal(packet.Body, req); err == nil {
			roomID = req.GetRoomId()
		}
	}

	// 创建 gRPC 请求上下文（5秒超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 调用游戏服 gRPC 获取房间信息接口
	resp, err := GlobalGameClient.GetRoomInfo(ctx, roomID)
	if err != nil {
		logger.Error("调用游戏服房间信息接口失败", "session_id", s.logID(), "player_id", playerID, "error", err)
		s.SendError(ErrCodeSystemError, "查询房间信息失败")
		return
	}

	// 将 rpc 响应序列化为 proto 发送给客户端
	body, err := proto.Marshal(resp)
	if err != nil {
		logger.Error("房间信息响应序列化失败", "session_id", s.logID(), "player_id", playerID, "error", err)
		s.SendError(ErrCodeSystemError, "系统错误")
		return
	}

	// 发送响应给客户端
	s.Send(&network.Packet{
		MsgID: network.MsgIDGameRoomInfoResp,
		SeqID: packet.SeqID,
		Body:  body,
	})

	logger.Info("房间信息查询响应发送成功", "session_id", s.logID(), "player_id", playerID, "room_id", resp.GetRoomId(), "status", resp.GetStatus())
}
