package handler

import (
	"go-snake-game/pkg/errcode"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
	"go-snake-game/pkg/proto/msg"
	"go-snake-game/internal/gateway/rpc"

	"google.golang.org/protobuf/proto"
)

// GameOperationHandler 游戏操作请求处理器。
func GameOperationHandler(s Session, packet *network.Packet) {
	playerID := s.PlayerID()
	logger.Info("收到游戏操作请求", "session_id", s.LogID(), "player_id", playerID, "seq_id", packet.SeqID)

	if playerID == 0 {
		logger.Warn("游戏操作请求未登录", "session_id", s.LogID())
		s.SendError(errcode.ErrNotLogin, "请先登录")
		return
	}

	req := &msg.GameOperationReq{}
	if err := proto.Unmarshal(packet.Body, req); err != nil {
		logger.Warn("游戏操作请求解析失败", "session_id", s.LogID(), "player_id", playerID, "error", err)
		s.SendError(errcode.ErrParam, "请求参数格式错误")
		return
	}

	direction := req.GetDirection()
	logger.Info("游戏操作请求", "session_id", s.LogID(), "player_id", playerID, "direction", direction)

	ctx, cancel := ContextWithTraceID(s)
	defer cancel()

	resp, err := rpc.GlobalGameClient.PlayerOperation(ctx, playerID, "", direction)
	if err != nil {
		logger.Error("调用游戏服玩家操作接口失败", "session_id", s.LogID(), "player_id", playerID, "error", err)
		s.SendError(errcode.ErrSystem, "操作失败，请稍后重试")
		return
	}

	logger.Info("游戏操作成功", "session_id", s.LogID(), "player_id", playerID, "code", resp.Code, "msg", resp.Msg)

	s.SendProtoResponse(network.MsgIDGameOperationResp, packet.SeqID, &msg.GameOperationResp{
		Code: int32(resp.Code),
		Msg:  resp.Msg,
	})
}

// RoomInfoQueryHandler 房间信息查询请求处理器。
func RoomInfoQueryHandler(s Session, packet *network.Packet) {
	playerID := s.PlayerID()
	logger.Info("收到房间信息查询请求", "session_id", s.LogID(), "player_id", playerID, "seq_id", packet.SeqID)

	if playerID == 0 {
		logger.Warn("房间信息查询请求未登录", "session_id", s.LogID())
		s.SendError(errcode.ErrNotLogin, "请先登录")
		return
	}

	req := &msg.RoomInfoQueryReq{}
	roomID := ""
	if len(packet.Body) > 0 {
		if err := proto.Unmarshal(packet.Body, req); err == nil {
			roomID = req.GetRoomId()
		}
	}

	ctx, cancel := ContextWithTraceID(s)
	defer cancel()

	resp, err := rpc.GlobalGameClient.GetRoomInfo(ctx, roomID)
	if err != nil {
		logger.Error("调用游戏服房间信息接口失败", "session_id", s.LogID(), "player_id", playerID, "error", err)
		s.SendError(errcode.ErrSystem, "查询房间信息失败")
		return
	}

	s.SendProtoResponse(network.MsgIDGameRoomInfoResp, packet.SeqID, resp)
	logger.Info("房间信息查询响应发送成功", "session_id", s.LogID(), "player_id", playerID, "room_id", resp.GetRoomId(), "status", resp.GetStatus())
}

// RankQueryHandler 排行榜查询请求处理器。
func RankQueryHandler(s Session, packet *network.Packet) {
	playerID := s.PlayerID()
	logger.Info("收到排行榜查询请求", "session_id", s.LogID(), "player_id", playerID, "seq_id", packet.SeqID)

	if playerID == 0 {
		logger.Warn("排行榜查询请求未登录", "session_id", s.LogID())
		s.SendError(errcode.ErrNotLogin, "请先登录")
		return
	}

	ctx, cancel := ContextWithTraceID(s)
	defer cancel()

	resp, err := rpc.GlobalGameClient.GetGlobalRank(ctx)
	if err != nil {
		logger.Error("调用游戏服排行榜接口失败", "session_id", s.LogID(), "player_id", playerID, "error", err)
		s.SendError(errcode.ErrSystem, "查询排行榜失败")
		return
	}

	rankResp := &msg.RankQueryResp{
		Code: resp.Code,
		Msg:  resp.Msg,
		List: make([]*msg.RankItem, 0, len(resp.List)),
	}
	for _, item := range resp.List {
		rankResp.List = append(rankResp.List, &msg.RankItem{
			PlayerId: item.PlayerId,
			Score:    item.Score,
			Rank:     item.Rank,
		})
	}

	s.SendProtoResponse(network.MsgIDRankQueryResp, packet.SeqID, rankResp)
	logger.Info("排行榜查询响应发送成功", "session_id", s.LogID(), "player_id", playerID, "count", len(rankResp.List))
}