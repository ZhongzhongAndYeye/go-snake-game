package gateway

import (
	"context"
	"errors"
	"time"

	"go-snake-game/pkg/errcode"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
)

// AuthMiddleware 鉴权中间件(鉴定客户端来的请求时是否已经登录)。
// 实现洋葱模型：前置校验 Token，通过则调用 next 执行业务 Handler。
//
// 鉴权流程：
//  1. 检查 Session 是否已登录（playerID > 0）
//  2. 已登录：直接调用 next 执行业务
//  3. 未登录：从请求中提取 Token，调用登录服 VerifyToken 校验
//  4. 校验通过：将 playerID 注入 Session，标记已登录，执行 next
//  5. 校验失败：返回未登录错误，不进入业务 Handler
func AuthMiddleware(next HandlerFunc) HandlerFunc {
	return func(s *Session, packet *network.Packet) {
		logger.Info("鉴权中间件处理", "session_id", s.logID(), "msg_id", packet.MsgID, "seq_id", packet.SeqID)

		// 步骤1：检查 Session 是否已登录（优先使用 IsLogin 标记）
		if s.isLogin {
			logger.Info("Session 已登录，跳过 Token 校验", "session_id", s.logID(), "player_id", s.PlayerID())
			next(s, packet)
			return
		}

		// 步骤2：未登录，从请求中提取 Token
		token, err := extractTokenFromRequest(packet.Body)
		if err != nil {
			logger.Warn("从请求中提取 Token 失败", "session_id", s.logID(), "error", err)
			s.SendError(errcode.ErrNotLogin, "请先登录")
			return
		}

		if token == "" {
			logger.Warn("请求中未携带 Token", "session_id", s.logID())
			s.SendError(errcode.ErrNotLogin, "请先登录")
			return
		}

		// 步骤3：调用登录服 VerifyToken 接口校验 Token
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		resp, err := GlobalLoginClient.VerifyToken(ctx, token)
		if err != nil {
			logger.Error("调用登录服 VerifyToken 接口失败", "session_id", s.logID(), "token", maskToken(token), "error", err)
			s.SendError(errcode.ErrSystem, "鉴权失败，请稍后重试")
			return
		}

		// 步骤4：校验结果处理
		if resp.Code != 0 {
			logger.Warn("Token 校验失败", "session_id", s.logID(), "token", maskToken(token), "code", resp.Code, "msg", resp.Msg)
			s.SendError(errcode.ErrNotLogin, resp.Msg)
			return
		}

		// 步骤5：校验通过，将 playerID 和登录状态注入 Session
		playerID := uint64(resp.PlayerId)
		s.SetPlayerID(playerID)
		s.SetLogin(true)
		logger.Info("Token 校验通过，Session 绑定玩家 ID", "session_id", s.logID(), "player_id", playerID, "token", maskToken(token))

		// 步骤6：调用 next 执行业务逻辑
		next(s, packet)
	}
}

// extractTokenFromRequest 从请求消息体中提取 Token。
// 由于当前 proto 文件中没有定义统一的 Token 请求消息格式，
// 这里实现一个简单的方案：尝试解析为常见消息类型，失败则返回错误。
// 在实际项目中，应该在 proto 文件中定义一个统一的消息包装器，包含 Token 字段。
func extractTokenFromRequest(body []byte) (string, error) {
	if len(body) == 0 {
		return "", errors.New("请求体为空")
	}

	// 尝试解析为几种常见的消息类型
	// HeartbeatReq、LoginReq、RegisterReq 都不包含 Token 字段

	// 返回错误，表示需要先登录
	// 在实际项目中，应该定义一个统一的消息格式来携带 Token
	return "", errors.New("请先登录")
}

// maskToken 对 Token 进行脱敏处理，保护日志中的敏感信息。
// 例如：abcd1234efgh5678 → abcd***5678
func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}
