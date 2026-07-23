// Package gateway 提供 WebSocket 网关服务器，负责客户端连接管理、消息路由分发和 gRPC 回调注入。
package gateway

import (
	"fmt"

	"go-snake-game/internal/gateway/handler"
	"go-snake-game/internal/gateway/middleware"
	"go-snake-game/pkg/logger"
	"go-snake-game/pkg/network"
	"go-snake-game/pkg/utils"
)

// Router 注册式消息路由器，根据消息 ID 将请求分发到对应的处理函数。
// 支持注册需鉴权路由、免鉴权路由和全局中间件链。
type Router struct {
	routes       map[uint16]handler.HandlerFunc
	publicRoutes map[uint16]handler.HandlerFunc
	middlewares  []handler.MiddlewareFunc
}

// NewRouter 创建路由器实例，默认挂载 TraceMiddleware 和 LogMiddleware。
func NewRouter() *Router {
	r := &Router{
		routes:       make(map[uint16]handler.HandlerFunc),
		publicRoutes: make(map[uint16]handler.HandlerFunc),
		middlewares:  make([]handler.MiddlewareFunc, 0),
	}
	r.Use(middleware.TraceMiddleware)
	r.Use(middleware.LogMiddleware)
	return r
}

// Register 注册需要鉴权的消息处理函数。
func (r *Router) Register(msgID uint16, h handler.HandlerFunc) {
	r.routes[msgID] = h
	logger.Info("注册消息路由", "msg_id", msgID)
}

// RegisterPublic 注册免鉴权的消息处理函数。
func (r *Router) RegisterPublic(msgID uint16, h handler.HandlerFunc) {
	r.publicRoutes[msgID] = h
	logger.Info("注册免鉴权消息路由", "msg_id", msgID)
}

// Use 添加全局中间件。
func (r *Router) Use(mw handler.MiddlewareFunc) {
	r.middlewares = append(r.middlewares, mw)
}

// Handle 根据消息 ID 路由请求到对应的处理函数。
func (r *Router) Handle(s *Session, packet *network.Packet) error {
	if h, ok := r.publicRoutes[packet.MsgID]; ok {
		s.SetTraceID(utils.GenerateTraceID())
		h(s, packet)
		return nil
	}

	h, ok := r.routes[packet.MsgID]
	if !ok {
		return fmt.Errorf("unknown message id: %d", packet.MsgID)
	}

	finalHandler := h
	for i := len(r.middlewares) - 1; i >= 0; i-- {
		finalHandler = r.middlewares[i](finalHandler)
	}

	finalHandler(s, packet)
	return nil
}
