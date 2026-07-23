// Package middleware 提供网关中间件，包括全链路追踪、日志记录、限流和鉴权。
// rate_limit.go 实现玩家级令牌桶限流中间件，每个玩家独立维护一个令牌桶，防止单个玩家发送过多请求导致服务过载。
// 使用 golang.org/x/time/rate 实现标准令牌桶算法。

package middleware

import (
	"sync"

	"go-snake-game/internal/gateway/handler"
	"go-snake-game/pkg/errcode"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"

	"golang.org/x/time/rate"
)

// RateLimitMiddleware 玩家级令牌桶限流中间件。
// 每个已登录玩家（playerID != 0）拥有独立的令牌桶，未登录玩家不受限制。
// 请求到达时先检查令牌桶是否有可用令牌：
//   - 有令牌：消耗一个令牌，继续执行后续中间件和业务处理
//   - 无令牌：返回 ErrOpTooFast 错误，拒绝请求
func RateLimitMiddleware(next handler.HandlerFunc) handler.HandlerFunc {
	return func(s handler.Session, packet *network.Packet) {
		playerID := s.PlayerID()

		if playerID == 0 {
			next(s, packet)
			return
		}

		limiter := getPlayerLimiter(playerID)
		if !limiter.Allow() {
			logger.Warn("请求限流拦截",
				"player_id", playerID,
				"trace_id", s.TraceID(),
				"msg_id", packet.MsgID,
			)
			s.SendError(errcode.ErrOpTooFast, "操作过于频繁，请稍后再试")
			return
		}

		next(s, packet)
	}
}

// playerLimiters 存储所有玩家的限流实例，key 为 playerID，value 为 *rate.Limiter。
// 使用 sync.Map 确保并发安全，每个玩家首次请求时自动创建对应的令牌桶。
var playerLimiters sync.Map

// getPlayerLimiter 根据玩家 ID 获取或创建对应的令牌桶实例。
// 使用 sync.Map.LoadOrStore 实现原子操作，避免并发竞态。
// 限流参数：每秒产生 10 个令牌（limitRate），最多存储 20 个令牌（burstLimit）。
//   - limitRate=10：限制玩家每秒最多发送 10 次请求
//   - burstLimit=20：允许玩家短时间内发送最多 20 次请求（应对突发操作）
func getPlayerLimiter(playerID uint64) *rate.Limiter {
	const (
		limitRate  = 10
		burstLimit = 20
	)
	val, _ := playerLimiters.LoadOrStore(playerID, rate.NewLimiter(rate.Limit(limitRate), burstLimit))
	return val.(*rate.Limiter)
}
