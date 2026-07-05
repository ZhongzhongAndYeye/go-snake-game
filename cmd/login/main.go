package main

import (
	"log"

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

	// 3. 打印登录服启动信息，携带 gRPC 监听地址字段
	logger.Info("登录服启动成功", "grpc_addr", config.GlobalCfg.Login.GrpcAddr)
}
