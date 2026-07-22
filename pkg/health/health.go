// Package health 提供 pprof 性能分析与健康检查 HTTP 服务。
// 各服务在启动时调用 StartPprofServer 即可接入。
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/pprof"
	"time"

	"go-snake-game/pkg/db"
	"go-snake-game/pkg/logger"
)

// Status 服务健康状态。
type Status struct {
	Service   string `json:"service"`    // 服务名称
	Status    string `json:"status"`     // "ok" 或 "degraded"
	StartTime string `json:"start_time"` // 服务启动时间
	MySQL     string `json:"mysql"`      // MySQL 连接状态
	Redis     string `json:"redis"`      // Redis 连接状态
}

var startTime = time.Now().Format(time.RFC3339)

// StartPprofServer 启动 pprof 性能分析与健康检查 HTTP 服务。
// serviceName 为服务名称（gateway/login/game），addr 为监听地址。
// 注册路由：
//   - /debug/pprof/  — pprof 性能分析
//   - /health         — 健康检查
func StartPprofServer(serviceName, addr string) {
	mux := http.NewServeMux()

	// 注册 pprof 路由
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	// 注册健康检查路由
	mux.HandleFunc("/health", healthHandler(serviceName))

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		logger.Info("pprof 服务启动",
			"service", serviceName,
			"addr", addr,
			"pprof_url", "http://"+addr+"/debug/pprof/",
			"health_url", "http://"+addr+"/health",
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("pprof 服务异常退出", "service", serviceName, "error", err)
		}
	}()
}

// healthHandler 返回健康检查处理函数。
func healthHandler(serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := Status{
			Service:   serviceName,
			Status:    "ok",
			StartTime: startTime,
			MySQL:     checkMySQL(),
			Redis:     checkRedis(),
		}

		// 如果任一依赖异常，标记为 degraded
		if status.MySQL != "ok" || status.Redis != "ok" {
			status.Status = "degraded"
		}

		code := http.StatusOK
		if status.Status == "degraded" {
			code = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(status)
	}
}

// checkMySQL 检查 MySQL 连接状态。
func checkMySQL() string {
	if db.GlobalDB == nil {
		return "not_initialized"
	}
	sqlDB, err := db.GlobalDB.DB()
	if err != nil {
		return "error: " + err.Error()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return "error: " + err.Error()
	}
	return "ok"
}

// checkRedis 检查 Redis 连接状态。
func checkRedis() string {
	if db.GlobalRedis == nil {
		return "not_initialized"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := db.GlobalRedis.Ping(ctx).Err(); err != nil {
		return "error: " + err.Error()
	}
	return "ok"
}
