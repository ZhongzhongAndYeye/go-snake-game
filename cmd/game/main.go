package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"go-snake-game/internal/game"
	"go-snake-game/pkg/config"
	"go-snake-game/pkg/db"
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

	// 3. 初始化 MySQL
	db.InitMySQL(&db.MySQLConfig{
		DSN:            config.GlobalCfg.Mysql.DSN,
		MaxOpen:        config.GlobalCfg.Mysql.MaxOpen,
		MaxIdle:        config.GlobalCfg.Mysql.MaxIdle,
		MaxLifeMinutes: config.GlobalCfg.Mysql.MaxLifeMinutes,
	})

	// 4. 初始化 Redis
	db.InitRedis(&db.RedisConfig{
		Addr:         config.GlobalCfg.Redis.Addr,
		DB:           config.GlobalCfg.Redis.DB,
		Password:     config.GlobalCfg.Redis.Password,
		PoolSize:     config.GlobalCfg.Redis.PoolSize,
		MinIdleConns: config.GlobalCfg.Redis.MinIdleConns,
		MaxRetries:   config.GlobalCfg.Redis.MaxRetries,
		DialTimeout:  config.GlobalCfg.Redis.DialTimeout,
		ReadTimeout:  config.GlobalCfg.Redis.ReadTimeout,
		WriteTimeout: config.GlobalCfg.Redis.WriteTimeout,
		PoolTimeout:  config.GlobalCfg.Redis.PoolTimeout,
	})

	// 5. 创建 gRPC 服务并注册 GameService
	grpcServer := grpc.NewServer()
	gameServer := game.NewGameServer()
	pb.RegisterGameServiceServer(grpcServer, gameServer)

	// 6. 监听配置中的 gRPC 地址
	grpcAddr := config.GlobalCfg.Game.GrpcAddr
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		logger.Error("游戏服监听失败", "grpc_addr", grpcAddr, "error", err.Error())
		os.Exit(1)
	}

	// 7. 捕获系统信号，优雅停止
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	// 启动 gRPC 服务
	go func() {
		logger.Info("游戏服启动成功", "grpc_addr", grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("游戏服启动失败", "error", err.Error())
			os.Exit(1)
		}
	}()

	// 8. 等待信号，优雅停止
	<-stop
	logger.Info("游戏服正在关闭")
	grpcServer.GracefulStop()
	logger.Info("游戏服已关闭")
}