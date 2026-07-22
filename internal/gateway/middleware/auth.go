package middleware

import (
	"errors"

	"go-snake-game/internal/gateway/handler"
	"go-snake-game/internal/gateway/rpc"
	"go-snake-game/pkg/errcode"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
)

// AuthMiddleware 鉴权中间件，验证客户端是否已登录。
// 如果 Session 已登录（通过 LoginHandler 设置），直接放行；
// 否则尝试从请求体中提取 Token 进行校验。
func AuthMiddleware(next handler.HandlerFunc) handler.HandlerFunc {
	return func(s handler.Session, packet *network.Packet) {
		// Session 已登录（LoginHandler 已设置 playerID 和 isLogin），直接放行
		if s.PlayerID() != 0 {
			next(s, packet)
			return
		}

		logger.Info("鉴权中间件处理", "session_id", s.LogID(), "msg_id", packet.MsgID, "seq_id", packet.SeqID)

		token, err := extractTokenFromRequest(packet.Body)
		if err != nil {
			logger.Warn("从请求中提取 Token 失败", "session_id", s.LogID(), "error", err)
			s.SendError(errcode.ErrNotLogin, "请先登录")
			return
		}

		if token == "" {
			logger.Warn("请求中未携带 Token", "session_id", s.LogID())
			s.SendError(errcode.ErrNotLogin, "请先登录")
			return
		}

		ctx, cancel := handler.ContextWithTraceID(s)
		defer cancel()

		resp, err := rpc.GlobalLoginClient.VerifyToken(ctx, token)
		if err != nil {
			logger.Error("调用登录服 VerifyToken 接口失败", "session_id", s.LogID(), "token", maskToken(token), "error", err)
			s.SendError(errcode.ErrSystem, "鉴权失败，请稍后重试")
			return
		}

		if resp.Code != 0 {
			logger.Warn("Token 校验失败", "session_id", s.LogID(), "token", maskToken(token), "code", resp.Code, "msg", resp.Msg)
			s.SendError(errcode.ErrNotLogin, resp.Msg)
			return
		}

		playerID := uint64(resp.PlayerId)
		s.SetPlayerID(playerID)
		s.SetLogin(true)
		logger.Info("Token 校验通过，Session 绑定玩家 ID", "session_id", s.LogID(), "player_id", playerID, "token", maskToken(token))

		next(s, packet)
	}
}

func extractTokenFromRequest(body []byte) (string, error) {
	if len(body) == 0 {
		return "", errors.New("请求体为空")
	}
	// 尝试从请求体中提取 Token 字段
	// 当前协议设计中 Token 由 LoginHandler 在登录时设置到 Session，
	// 后续请求通过 Session.isLogin 判断，不再从请求体提取 Token。
	return "", errors.New("请先登录")
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}
