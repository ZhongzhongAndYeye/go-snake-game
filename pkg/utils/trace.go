package utils

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"google.golang.org/grpc/metadata"
)

// TraceIDKey 上下文 key，用于在 context 中存取 TraceID。
const TraceIDKey = "trace_id"

// contextKey 自定义 context key 类型，避免与内置 string 类型冲突。
type contextKey struct{}

// GenerateTraceID 生成 16 位随机十六进制字符串作为全链路追踪 ID。
// 使用 crypto/rand 保证密码学安全的随机性，避免碰撞。
func GenerateTraceID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// SetTraceIDToCtx 将 TraceID 注入 context，返回新的 context。
func SetTraceIDToCtx(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, contextKey{}, traceID)
}

// GetTraceIDFromCtx 从 context 中提取 TraceID。
// 不存在或类型不匹配时返回空字符串。
func GetTraceIDFromCtx(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v := ctx.Value(contextKey{})
	if v == nil {
		return ""
	}
	traceID, ok := v.(string)
	if !ok {
		return ""
	}
	return traceID
}

// GetTraceIDFromMetadata 从 gRPC 请求的 metadata 中提取 TraceID。
// 用于下游服务（登录服、游戏服）从网关透传的 metadata 中获取 TraceID。
func GetTraceIDFromMetadata(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	vals := md.Get(TraceIDKey)
	if len(vals) == 0 {
		return ""
	}
	return vals[0]
}
