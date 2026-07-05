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

	// 3. 打印网关启动信息，携带监听端口字段
	logger.Info("网关服启动成功", "listen_addr", config.GlobalCfg.Gateway.ListenAddr)
}
