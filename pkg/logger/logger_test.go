package logger

import (
	"testing"

	"go-snake-game/pkg/config"
)

func TestLogLevels(t *testing.T) {
	// 初始化日志，控制台模式，debug 级别（确保所有级别都能输出）
	// - 设置 level: "debug" → 所有级别都输出（测试用例用这个确保都能看到）
	// - 设置 level: "info" → 只输出 INFO、WARN、ERROR，DEBUG 被忽略
	// - 设置 level: "error" → 只输出 ERROR
	cfg := config.LogConfig{
		Level:   "debug",
		Console: true,
		File:    "",
	}
	if err := InitLogger(cfg); err != nil {
		t.Fatalf("初始化日志失败: %v", err)
	}

	// 测试四个级别日志输出
	Debug("调试信息", "key", "debug_value")
	Info("普通信息", "key", "info_value")
	Warn("警告信息", "key", "warn_value")
	Error("错误信息", "key", "error_value")
}

// TestWithTraceIDUsage 测试 WithTraceID 的用法
// 演示如何在请求链路中使用 trace_id 进行日志追踪
func TestWithTraceIDUsage(t *testing.T) {
	// 初始化日志
	cfg := config.LogConfig{
		Level:   "debug",
		Console: true,
		File:    "",
	}
	if err := InitLogger(cfg); err != nil {
		t.Fatalf("初始化日志失败: %v", err)
	}

	// 模拟一个用户请求链路，使用相同的 trace_id
	traceID := "req-20260705-abc123"
	traceLog := WithTraceID(traceID)

	// 整个请求链路的日志都会自动携带 trace_id
	traceLog.Info("用户登录", "uid", 1001, "ip", "192.168.1.100")
	traceLog.Debug("验证 token", "token", "xxx...xxx")
	traceLog.Info("开始匹配游戏", "mode", "rank")
	traceLog.Info("匹配成功", "room_id", "room-456", "players", 4)
	traceLog.Info("游戏开始", "map_size", "10x10")

	// 模拟另一个请求，使用不同的 trace_id
	traceID2 := "req-20260705-def456"
	traceLog2 := WithTraceID(traceID2)
	traceLog2.Info("用户登录", "uid", 1002, "ip", "192.168.1.101")

	// 验证：通过 trace_id 可以追踪同一请求的所有日志
	// 在日志系统中搜索 "req-20260705-abc123" 就能看到第一个请求的完整链路
}
