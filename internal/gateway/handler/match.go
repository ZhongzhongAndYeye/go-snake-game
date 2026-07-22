package handler

import (
	"go-snake-game/pkg/errcode"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
	"go-snake-game/pkg/proto/msg"
	"go-snake-game/internal/gateway/rpc"
)

// MatchStartHandler 发起匹配请求处理器。
func MatchStartHandler(s Session, packet *network.Packet) {
	playerID := s.PlayerID()
	logger.Info("收到发起匹配请求", "session_id", s.LogID(), "player_id", playerID, "seq_id", packet.SeqID)

	if playerID == 0 {
		logger.Warn("匹配请求未登录", "session_id", s.LogID())
		s.SendError(errcode.ErrNotLogin, "请先登录")
		return
	}

	ctx, cancel := ContextWithTraceID(s)
	defer cancel()

	resp, err := rpc.GlobalGameClient.StartMatch(ctx, playerID, "")
	if err != nil {
		logger.Error("调用游戏服发起匹配接口失败", "session_id", s.LogID(), "player_id", playerID, "error", err)
		s.SendError(errcode.ErrSystem, "匹配失败，请稍后重试")
		return
	}

	s.SendProtoResponse(network.MsgIDMatchStartResp, packet.SeqID, &msg.MatchStartResp{
		Code:      resp.Code,
		Msg:       resp.Msg,
		RoomId:    resp.RoomId,
		IsMatched: resp.IsMatched,
	})

	logger.Info("匹配响应发送成功", "session_id", s.LogID(), "player_id", playerID, "code", resp.Code, "is_matched", resp.IsMatched)
}

// MatchCancelHandler 取消匹配请求处理器。
func MatchCancelHandler(s Session, packet *network.Packet) {
	playerID := s.PlayerID()
	logger.Info("收到取消匹配请求", "session_id", s.LogID(), "player_id", playerID, "seq_id", packet.SeqID)

	if playerID == 0 {
		logger.Warn("取消匹配请求未登录", "session_id", s.LogID())
		s.SendError(errcode.ErrNotLogin, "请先登录")
		return
	}

	ctx, cancel := ContextWithTraceID(s)
	defer cancel()

	resp, err := rpc.GlobalGameClient.CancelMatch(ctx, playerID)
	if err != nil {
		logger.Error("调用游戏服取消匹配接口失败", "session_id", s.LogID(), "player_id", playerID, "error", err)
		s.SendError(errcode.ErrSystem, "取消匹配失败，请稍后重试")
		return
	}

	s.SendProtoResponse(network.MsgIDMatchCancelResp, packet.SeqID, &msg.MatchCancelResp{
		Code: resp.Code,
		Msg:  resp.Msg,
	})

	logger.Info("取消匹配响应发送成功", "session_id", s.LogID(), "player_id", playerID, "code", resp.Code)
}