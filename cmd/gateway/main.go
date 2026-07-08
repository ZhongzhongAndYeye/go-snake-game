package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go-snake-game/internal/gateway"
	"go-snake-game/pkg/config"
	"go-snake-game/pkg/logger"
)

func main() {
	// 1. 加载配置文件
	if err := config.InitConfig("configs/dev.yaml"); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 2. 初始化日志
	if err := logger.InitLogger(config.GlobalCfg.Log); err != nil {
		log.Fatalf("初始化日志失败: %v", err)
	}

	// 3. 启动监听服务
	listenAddr := config.GlobalCfg.Gateway.ListenAddr
	logger.Info("网关服务启动成功", "listen_addr", listenAddr)

	server := gateway.NewGatewayServer(listenAddr)

	stop := make(chan os.Signal, 1)
	// syscall.SIGINT ： Ctrl+C 信号，用户在终端按 Ctrl+C 时触发
	// syscall.SIGTERM ： 优雅终止 信号， kill 命令或 Docker/K8s 停止容器时触发
	// 执行以上这两种操作时 会往stop通道发送信号，触发后续的关闭操作
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("网关服务启动失败", "error", err.Error())
			os.Exit(1)
		}
	}()

	<-stop // 触发关闭网关服务

	if err := server.Stop(); err != nil {
		logger.Error("网关服务停止失败", "error", err.Error())
		os.Exit(1)
	}
}
