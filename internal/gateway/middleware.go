package gateway

import (
	"time"

	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
)

// 【日志中间件 默认在初始化路由时挂载】。
//
// 功能：
//  1. 记录请求开始时间
//  2. 调用 next 执行业务逻辑
//  3. 记录请求耗时
//  4. 捕获业务 Handler 中的 panic，防止单个消息崩溃导致整个连接断开
//
// 执行流程：
//
//	记录开始时间
//	   │
//	   ▼
//	defer recover 捕获 panic
//	   │
//	   ▼
//	next(s, packet)  ← 执行业务逻辑
//	   │
//	   ▼
//	计算耗时、打印日志
//
// 使用方式：在 NewRouter 中默认挂载，无需手动调用。
func LogMiddleware(next HandlerFunc) HandlerFunc {
	return func(s *Session, packet *network.Packet) {
		// 记录请求开始时间，用于计算处理耗时
		start := time.Now()

		// 获取消息 ID 和序列号，用于日志标识
		msgID := packet.MsgID
		seqID := packet.SeqID

		// defer 保证无论业务逻辑是否 panic，都能记录日志
		defer func() {
			// 计算耗时（毫秒）
			elapsed := time.Since(start).Milliseconds()

			// ---- 异常捕获 ----
			// 业务 Handler 发生 panic 时，recover 会捕获到 panic 的值
			// 如果 recover() 返回 nil，说明没有 panic，正常运行
			if r := recover(); r != nil {
				// 记录 panic 错误日志，包含 panic 内容和堆栈信息
				logger.Error("消息处理 panic",
					"msg_id", msgID,
					"seq_id", seqID,
					"panic", r,
					"elapsed_ms", elapsed,
				)
				// 不向外抛出 panic，保证连接不会崩溃
				return
			}

			// 正常处理完成，记录请求日志
			logger.Info("消息处理完成",
				"msg_id", msgID, // 消息类型 ID
				"seq_id", seqID, // 消息序列号
				"elapsed_ms", elapsed, // 处理耗时（毫秒）
			)
		}()

		// 调用下一个 Handler（可能是下一个中间件，也可能是业务 Handler）
		next(s, packet)
	}
}
