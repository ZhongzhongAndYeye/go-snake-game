package middleware

import (
	"go-snake-game/internal/gateway/handler"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
	"go-snake-game/pkg/utils"
)

// TraceMiddleware 全链路追踪中间件，为每条请求生成唯一 TraceID 并注入 Session。
func TraceMiddleware(next handler.HandlerFunc) handler.HandlerFunc {
	return func(s handler.Session, packet *network.Packet) {
		traceID := utils.GenerateTraceID()
		s.SetTraceID(traceID)

		if traceLog := logger.WithTraceID(traceID); traceLog != nil {
			traceLog.Info("请求进入",
				"session_id", s.LogID(),
				"msg_id", packet.MsgID,
				"seq_id", packet.SeqID,
			)
		}

		next(s, packet)
	}
}
