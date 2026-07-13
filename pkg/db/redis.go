package db

import (
	"context"
	"sync"
	"time"

	"go-snake-game/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// GlobalRedis 全局 Redis 实例，InitRedis 调用后赋值，全局可访问。
var GlobalRedis *redis.Client

var (
	redisOnce sync.Once
)

// RedisConfig Redis 数据库连接配置。
type RedisConfig struct {
	Addr         string // Redis 地址，格式：host:port，如 127.0.0.1:6379
	DB           int    // 数据库编号，默认 0
	Password     string // 密码，无密码为空字符串
	PoolSize     int    // 连接池最大连接数
	MinIdleConns int    // 最小空闲连接数
	MaxRetries   int    // 操作失败最大重试次数
	DialTimeout  int    // 连接超时（秒）
	ReadTimeout  int    // 读超时（秒）
	WriteTimeout int    // 写超时（秒）
	PoolTimeout  int    // 从连接池获取连接的超时时间（秒）
}

// InitRedis 初始化 Redis 客户端，将实例赋值给全局变量 GlobalRedis。
func InitRedis(cfg *RedisConfig) {
	redisOnce.Do(func() {
		// 创建 Redis 客户端
		rdb := redis.NewClient(&redis.Options{
			Addr:         cfg.Addr,
			DB:           cfg.DB,
			Password:     cfg.Password,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
			MaxRetries:   cfg.MaxRetries,
			DialTimeout:  time.Duration(cfg.DialTimeout) * time.Second,
			ReadTimeout:  time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout: time.Duration(cfg.WriteTimeout) * time.Second,
			PoolTimeout:  time.Duration(cfg.PoolTimeout) * time.Second,
		})

		// 创建一个五秒后自动过期的上下文
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel() // 一连上直接停止ctx中的定时器，优雅退出

		// 传给Ping，执行 Ping 验证连通性（5s连不上即超时）
		if err := rdb.Ping(ctx).Err(); err != nil {
			logger.Error("Redis 初始化失败", "error", err.Error())
			panic("Redis 初始化失败: " + err.Error())
		}

		// 赋值全局变量
		GlobalRedis = rdb

		logger.Info("Redis 初始化成功",
			"addr", cfg.Addr,
			"db", cfg.DB,
			"pool_size", cfg.PoolSize,
			"min_idle", cfg.MinIdleConns,
			"max_retries", cfg.MaxRetries,
		)
	})
}
