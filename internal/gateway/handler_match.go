package gateway

import (
	"context"
	"time"

	"go-snake-game/pkg/errcode"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
	"go-snake-game/pkg/proto/msg"

	"google.golang.org/protobuf/proto"
)

// MatchStartHandler 发起匹配请求处理器。
func MatchStartHandler(s *Session, packet *network.Packet) {
	playerID := s.PlayerID()
	logger.Info("收到发起匹配请求", "session_id", s.logID(), "player_id", playerID, "seq_id", packet.SeqID)

	// 校验玩家是否已登录
	if playerID == 0 {
		logger.Warn("匹配请求未登录", "session_id", s.logID())
		s.SendError(errcode.ErrNotLogin, "请先登录")
		return
	}

	// 创建 gRPC 请求上下文（5秒超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 调用游戏服 gRPC 发起匹配接口
	// 注意：MatchStartReq 为空消息，昵称暂不传递
	resp, err := GlobalGameClient.StartMatch(ctx, playerID, "")
	if err != nil {
		logger.Error("调用游戏服发起匹配接口失败", "session_id", s.logID(), "player_id", playerID, "error", err)
		s.SendError(errcode.ErrSystem, "匹配失败，请稍后重试")
		return
	}

	// 封装匹配响应
	matchResp := &msg.MatchStartResp{
		Code:      resp.Code,
		Msg:       resp.Msg,
		RoomId:    resp.RoomId,
		IsMatched: resp.IsMatched,
	}

	// 序列化响应体
	body, err := proto.Marshal(matchResp)
	if err != nil {
		logger.Error("匹配响应序列化失败", "session_id", s.logID(), "player_id", playerID, "error", err)
		s.SendError(errcode.ErrSystem, "系统错误")
		return
	}

	// 发送响应给客户端
	s.Send(&network.Packet{
		MsgID: network.MsgIDMatchStartResp,
		SeqID: packet.SeqID,
		Body:  body,
	})

	logger.Info("匹配响应发送成功", "session_id", s.logID(), "player_id", playerID, "code", resp.Code, "is_matched", resp.IsMatched)
}

// MatchCancelHandler 取消匹配请求处理器。
func MatchCancelHandler(s *Session, packet *network.Packet) {
	playerID := s.PlayerID()
	logger.Info("收到取消匹配请求", "session_id", s.logID(), "player_id", playerID, "seq_id", packet.SeqID)

	// 校验玩家是否已登录
	if playerID == 0 {
		logger.Warn("取消匹配请求未登录", "session_id", s.logID())
		s.SendError(errcode.ErrNotLogin, "请先登录")
		return
	}

	// 创建 gRPC 请求上下文（5秒超时）
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 调用游戏服 gRPC 取消匹配接口
	resp, err := GlobalGameClient.CancelMatch(ctx, playerID)
	if err != nil {
		logger.Error("调用游戏服取消匹配接口失败", "session_id", s.logID(), "player_id", playerID, "error", err)
		s.SendError(errcode.ErrSystem, "取消匹配失败，请稍后重试")
		return
	}

	// 封装取消匹配响应
	cancelResp := &msg.MatchCancelResp{
		Code: resp.Code,
		Msg:  resp.Msg,
	}

	// 序列化响应体
	body, err := proto.Marshal(cancelResp)
	if err != nil {
		logger.Error("取消匹配响应序列化失败", "session_id", s.logID(), "player_id", playerID, "error", err)
		s.SendError(errcode.ErrSystem, "系统错误")
		return
	}

	// 发送响应给客户端
	s.Send(&network.Packet{
		MsgID: network.MsgIDMatchCancelResp,
		SeqID: packet.SeqID,
		Body:  body,
	})

	logger.Info("取消匹配响应发送成功", "session_id", s.logID(), "player_id", playerID, "code", resp.Code)
}
