package middleware

import (
	"time"

	"go-snake-game/internal/gateway/handler"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
)

// LogMiddleware 日志中间件，记录每条消息的处理耗时和 panic 捕获。
func LogMiddleware(next handler.HandlerFunc) handler.HandlerFunc {
	return func(s handler.Session, packet *network.Packet) {
		start := time.Now()
		msgID := packet.MsgID
		seqID := packet.SeqID

		defer func() {
			elapsed := time.Since(start).Milliseconds()
			if r := recover(); r != nil {
				logger.Error("消息处理 panic",
					"msg_id", msgID,
					"seq_id", seqID,
					"panic", r,
					"elapsed_ms", elapsed,
				)
				return
			}
			logger.Info("消息处理完成",
				"msg_id", msgID,
				"seq_id", seqID,
				"elapsed_ms", elapsed,
			)
		}()

		next(s, packet)
	}
}
