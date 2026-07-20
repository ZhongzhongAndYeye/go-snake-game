package gateway

import (
	"context"
	"time"

	"go-snake-game/pkg/utils"

	"google.golang.org/grpc/metadata"
)

// contextWithTraceID 创建一个带超时和 TraceID 的 gRPC 调用上下文。
// 从 Session 中提取 TraceID，通过 gRPC metadata 透传到下游服务。
// 如果 Session 没有 TraceID，则生成一个新的。
func contextWithTraceID(s *Session) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	traceID := s.TraceID()
	if traceID == "" {
		traceID = utils.GenerateTraceID()
		s.SetTraceID(traceID)
	}

	// 将 TraceID 注入 gRPC metadata，透传到下游服务
	md := metadata.Pairs(utils.TraceIDKey, traceID)
	ctx = metadata.NewOutgoingContext(ctx, md)

	return ctx, cancel
}
