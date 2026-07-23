// Package handler 提供消息处理函数类型定义和 Session 接口，避免子包间循环依赖。
package handler

import (
	"context"
	"time"

	"go-snake-game/pkg/utils"

	"google.golang.org/grpc/metadata"
)

// ContextWithTraceID 创建一个带超时和 TraceID 的 gRPC 调用上下文。
// 从 Session 中提取 TraceID，通过 gRPC metadata 透传到下游服务。
func ContextWithTraceID(s Session) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	traceID := s.TraceID()
	if traceID == "" {
		traceID = utils.GenerateTraceID()
		s.SetTraceID(traceID)
	}

	md := metadata.Pairs(utils.TraceIDKey, traceID)
	ctx = metadata.NewOutgoingContext(ctx, md)

	return ctx, cancel
}
