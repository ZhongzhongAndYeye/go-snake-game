package db

import (
	"context"
	"testing"
	"time"

	"go-snake-game/internal/model"
	"go-snake-game/pkg/config"
)

func TestMySQLAndRedisConnection(t *testing.T) {
	t.Log("=== 开始测试 MySQL 和 Redis 连接 ===")

	t.Log("1. 加载配置文件...")
	err := config.InitConfig("../../configs/dev.yaml")
	if err != nil {
		t.Fatalf("加载配置文件失败: %v", err)
	}
	t.Log("配置文件加载成功")

	t.Log("2. 初始化 MySQL 连接...")
	cfg := config.GlobalCfg.Mysql
	InitMySQL(&MySQLConfig{
		DSN:            cfg.DSN,
		MaxOpen:        cfg.MaxOpen,
		MaxIdle:        cfg.MaxIdle,
		MaxLifeMinutes: cfg.MaxLifeMinutes,
	})
	t.Log("MySQL 连接初始化成功")

	t.Log("3. 执行 MySQL Ping...")
	sqlDB, err := GlobalDB.DB()
	if err != nil {
		t.Fatalf("获取底层 sql.DB 失败: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("MySQL Ping 失败: %v", err)
	}
	t.Log("MySQL Ping 成功")

	t.Log("4. 执行 AutoMigrate 创建 player 表...")
	if err := GlobalDB.AutoMigrate(&model.Player{}); err != nil {
		t.Fatalf("AutoMigrate 失败: %v", err)
	}
	t.Log("player 表创建成功")

	t.Log("5. 初始化 Redis 连接...")
	redisCfg := config.GlobalCfg.Redis
	InitRedis(&RedisConfig{
		Addr:         redisCfg.Addr,
		DB:           redisCfg.DB,
		Password:     redisCfg.Password,
		PoolSize:     redisCfg.PoolSize,
		MinIdleConns: redisCfg.MinIdleConns,
		MaxRetries:   redisCfg.MaxRetries,
		DialTimeout:  redisCfg.DialTimeout,
		ReadTimeout:  redisCfg.ReadTimeout,
		WriteTimeout: redisCfg.WriteTimeout,
		PoolTimeout:  redisCfg.PoolTimeout,
	})
	t.Log("Redis 连接初始化成功")

	t.Log("6. 执行 Redis Ping...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := GlobalRedis.Ping(ctx).Err(); err != nil {
		t.Fatalf("Redis Ping 失败: %v", err)
	}
	t.Log("Redis Ping 成功")

	t.Log("=== 所有测试通过 ===")
}
