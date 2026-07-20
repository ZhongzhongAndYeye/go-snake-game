package gateway

import (
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
	"go-snake-game/pkg/utils"
)

// TraceMiddleware 全链路追踪中间件，符合 MiddlewareFunc 类型。
// 每条请求进入时生成唯一 TraceID，注入 Session 以便后续所有日志自动携带 trace_id。
// 洋葱模型：请求进入 → 生成 TraceID → next 执行业务 → 返回。
func TraceMiddleware(next HandlerFunc) HandlerFunc {
	return func(s *Session, packet *network.Packet) {
		// 生成唯一 TraceID 并注入 Session
		traceID := utils.GenerateTraceID()
		s.SetTraceID(traceID)

		// 使用携带 trace_id 的 Logger 记录请求入口
		if traceLog := logger.WithTraceID(traceID); traceLog != nil {
			traceLog.Info("请求进入",
				"session_id", s.logID(),
				"msg_id", packet.MsgID,
				"seq_id", packet.SeqID,
			)
		}

		// 调用 next 执行业务逻辑（可能是下一个中间件或业务 Handler）
		next(s, packet)
	}
}
