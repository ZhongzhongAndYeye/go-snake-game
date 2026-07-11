package gateway

import (
	"fmt"

	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
)

// 【路由】

// HandlerFunc 消息处理函数类型。
// 接收当前会话和收到的消息包，负责解析包体并执行业务逻辑。
type HandlerFunc func(s *Session, packet *network.Packet)

// MiddlewareFunc 中间件函数类型。
// 接收一个 HandlerFunc，返回一个新的 HandlerFunc。
// 中间件可以在目标 Handler 执行前后插入额外逻辑（如日志、鉴权、统计）。
type MiddlewareFunc func(next HandlerFunc) HandlerFunc

// Router 注册式消息路由器
type Router struct {
	// routes 路由表，key 为消息 ID（uint16），value 为处理函数
	routes map[uint16]HandlerFunc

	// middlewares 全局中间件切片，按注册顺序排列
	middlewares []MiddlewareFunc
}

// NewRouter 创建路由器实例。
// 默认挂载 LogMiddleware 日志中间件，自动记录每条消息的处理耗时。
func NewRouter() *Router {
	r := &Router{
		routes:      make(map[uint16]HandlerFunc),
		middlewares: make([]MiddlewareFunc, 0),
	}
	// 默认挂载日志中间件，自动记录每条消息的 msg_id、seq_id、处理耗时
	// 同时捕获业务 Handler 的 panic，防止单个消息崩溃导致整个连接断开
	r.Use(LogMiddleware)
	return r
}

// Register 注册消息处理函数。
// 将指定的 MsgID 与处理函数绑定，同一个 MsgID 重复注册会覆盖旧的处理函数。
func (r *Router) Register(msgID uint16, handler HandlerFunc) {
	r.routes[msgID] = handler
	logger.Info("注册消息路由",
		"msg_id", msgID,
	)
}

// Use 添加全局中间件。
// 中间件按注册顺序执行，先注册的在外层（先执行），后注册的在内层（靠近业务 Handler）。
// 比如：
//
//	router.Use(LoggingMiddleware)   // 第1个注册，最先执行
//	router.Use(AuthMiddleware)      // 第2个注册，在 Logging 之后执行
//	// 实际执行顺序：Logging → Auth → 业务Handler
func (r *Router) Use(mw MiddlewareFunc) {
	r.middlewares = append(r.middlewares, mw)
}

// Handle 根据消息 ID 路由请求到对应的处理函数。
// 客户端发来一条消息 MsgID=1003
//
//	│
//	▼
//
// Handle(session, packet)
//
//	│
//	├── 查路由表：routes[1003] → 找到 handleLogin ✅
//	│
//	├── 套中间件链：
//	│     handleLogin                          ← 最内层（业务）
//	│     └── 日志中间件(handleLogin)           ← 第2层
//	│     └── 鉴权中间件(日志中间件(handleLogin)) ← 最外层
//	│
//	└── 执行：
//	     鉴权中间件 → 日志中间件 → handleLogin → 返回
func (r *Router) Handle(s *Session, packet *network.Packet) error {
	// 第一步：根据 MsgID 查找路由表
	handler, ok := r.routes[packet.MsgID]
	if !ok {
		// 找不到对应的处理函数，返回错误
		return fmt.Errorf("unknown message id: %d", packet.MsgID)
	}

	// 第二步：构建中间件链
	// 中间件是洋葱模型：先注册的在最外层，最后注册的紧贴业务 Handler
	// 例如注册了 mw1, mw2，最终链为：mw1(mw2(handler))
	// 执行顺序：mw1 前置 → mw2 前置 → handler → mw2 后置 → mw1 后置
	finalHandler := handler
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		finalHandler = r.middlewares[i](finalHandler)
	}

	// 第三步：执行中间件链（最终调用业务 Handler）
	finalHandler(s, packet)

	return nil
}
