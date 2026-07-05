//go:build exclude

package config

import (
	"testing"
)

func TestInitConfig(t *testing.T) {
	// 加载开发环境配置
	err := InitConfig("../../configs/dev.yaml")
	if err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}

	// 打印网关端口
	gatewayAddr := GlobalCfg.Gateway.ListenAddr
	t.Logf("网关监听地址: %s", gatewayAddr)

	// 打印 MySQL DSN
	mysqlDSN := GlobalCfg.Mysql.DSN
	t.Logf("MySQL DSN: %s", mysqlDSN)

	// 验证网关端口不为空
	if gatewayAddr == "" {
		t.Error("网关监听地址不应为空")
	}

	// 验证 MySQL DSN 不为空
	if mysqlDSN == "" {
		t.Error("MySQL DSN 不应为空")
	}

	// 验证 MySQL DSN 包含预期内容
	if GlobalCfg.Mysql.MaxOpen != 20 {
		t.Errorf("MySQL 最大连接数期望 20，实际 %d", GlobalCfg.Mysql.MaxOpen)
	}
}
