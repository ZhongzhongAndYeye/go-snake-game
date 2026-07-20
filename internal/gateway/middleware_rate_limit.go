package gateway

import (
	"sync"

	"go-snake-game/pkg/errcode"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"

	"golang.org/x/time/rate"
)

// RateLimitMiddleware 玩家级令牌桶限流中间件，符合 MiddlewareFunc 类型。
//
// 令牌桶算法特点：
//   - 平滑限流：以固定速率生成令牌，请求需消耗令牌才能通过
//   - 支持突发流量：桶容量大于生成速率时，短时突发请求可消耗桶内积攒的令牌
//   - 玩家维度隔离：每个玩家独立一个限流器，单个玩家刷接口不影响其他用户
//
// 限流规则：
//   - 每秒生成 10 个令牌（rate=10）
//   - 桶容量 20 个令牌（burst=20），允许短时 2 倍突发
//   - 每次请求消耗 1 个令牌
//   - 令牌不足时直接返回 ErrOpTooFast 错误，不进入业务 Handler
func RateLimitMiddleware(next HandlerFunc) HandlerFunc {
	return func(s *Session, packet *network.Packet) {
		playerID := s.PlayerID()

		// 未登录玩家不限制，由后续 AuthMiddleware 处理
		if playerID == 0 {
			next(s, packet)
			return
		}

		// 获取或创建该玩家的限流器
		limiter := getPlayerLimiter(playerID)

		// 尝试消耗 1 个令牌
		if !limiter.Allow() {
			// 令牌不足，限流拦截
			logger.Warn("请求限流拦截",
				"player_id", playerID,
				"trace_id", s.TraceID(),
				"msg_id", packet.MsgID,
			)
			s.SendError(errcode.ErrOpTooFast, "操作过于频繁，请稍后再试")
			return
		}

		// 令牌充足，放行到下一个中间件或业务 Handler
		next(s, packet)
	}
}

// playerLimiters 存储所有玩家的令牌桶限流器，key 为 playerID。
// 使用 sync.Map 保证高并发下的读写安全，无需额外加锁。
var playerLimiters sync.Map

// getPlayerLimiter 获取或创建指定玩家的令牌桶限流器。
// 如果该玩家首次请求，自动创建新的限流器并存入 sync.Map。
// 并发安全：多个 goroutine 同时为同一玩家创建限流器时，
// LoadOrStore 保证只有一个实例被存储，避免重复创建。
func getPlayerLimiter(playerID uint64) *rate.Limiter {
	// 每 0.1 秒生成 1 个令牌（即每秒 10 个），桶容量 20
	const (
		limitRate  = 10 // 每秒生成的令牌数
		burstLimit = 20 // 桶容量，支持短时突发
	)

	val, _ := playerLimiters.LoadOrStore(playerID, rate.NewLimiter(rate.Limit(limitRate), burstLimit))
	return val.(*rate.Limiter)
}
