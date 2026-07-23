// 网关服务入口，负责 WebSocket 连接管理、消息路由转发、gRPC 推送服务。
package main

import (
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go-snake-game/internal/gateway"
	gwRpc "go-snake-game/internal/gateway/rpc"
	"go-snake-game/pkg/config"
	"go-snake-game/pkg/health"
	"go-snake-game/pkg/logger"
	pb "go-snake-game/pkg/proto/rpc"

	"google.golang.org/grpc"
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

	// 3. 初始化登录服 gRPC 客户端
	loginRpcAddr := config.GlobalCfg.Gateway.LoginRpcAddr
	logger.Info("初始化登录服 gRPC 连接", "addr", loginRpcAddr)
	gwRpc.InitLoginRpcClient(loginRpcAddr)

	// 4. 初始化游戏服 gRPC 客户端
	gameRpcAddr := config.GlobalCfg.Gateway.GameRpcAddr
	logger.Info("初始化游戏服 gRPC 连接", "addr", gameRpcAddr)
	gwRpc.InitGameRpcClient(gameRpcAddr)

	// 5. 启动 pprof 性能分析与健康检查（独立端口）
	if config.GlobalCfg.Gateway.PprofEnabled {
		health.StartPprofServer("gateway", config.GlobalCfg.Gateway.PprofAddr)
	}

	// 6. 启动 WebSocket 监听服务
	listenAddr := config.GlobalCfg.Gateway.ListenAddr
	logger.Info("网关 WebSocket 服务启动", "listen_addr", listenAddr)

	server := gateway.NewGatewayServer(listenAddr)

	// 6. 启动 gRPC 推送服务
	grpcAddr := config.GlobalCfg.Gateway.GrpcAddr
	logger.Info("网关 gRPC 推送服务启动", "grpc_addr", grpcAddr)

	grpcServer := grpc.NewServer()
	gatewayRpcServer := gwRpc.NewGatewayRpcServer()
	pb.RegisterGatewayServiceServer(grpcServer, gatewayRpcServer)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Error("网关 gRPC 监听失败", "grpc_addr", grpcAddr, "error", err.Error())
		os.Exit(1)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// 启动 WebSocket 服务（goroutine）
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			logger.Error("网关 WebSocket 服务启动失败", "error", err.Error())
			os.Exit(1)
		}
	}()

	// 启动 gRPC 服务（goroutine）
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("网关 gRPC 服务启动失败", "error", err.Error())
			os.Exit(1)
		}
	}()

	<-stop // 等待关闭信号

	// 优雅关闭 gRPC 服务
	logger.Info("正在关闭网关 gRPC 服务")
	grpcServer.GracefulStop()

	// 优雅关闭 WebSocket 服务
	logger.Info("正在关闭网关 WebSocket 服务")
	if err := server.Stop(); err != nil {
		logger.Error("网关 WebSocket 服务停止失败", "error", err.Error())
		os.Exit(1)
	}
}
