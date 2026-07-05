package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// GlobalCfg 全局配置单例，InitConfig 后可用
var GlobalCfg *AppConfig

// InitConfig 加载指定路径的 yaml 配置文件，同时支持环境变量覆盖。
// 环境变量格式：将配置键中的 "." 替换为 "_"，例如 gateway.listen_addr → GATEWAY_LISTEN_ADDR
func InitConfig(confPath string) error {
	v := viper.New()

	// 设置配置文件路径与类型
	v.SetConfigFile(confPath)
	v.SetConfigType("yaml")

	// 启用环境变量覆盖，键名中 "." 替换为 "_"
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败 [%s]: %w", confPath, err)
	}

	// 反序列化到全局配置结构体
	cfg := &AppConfig{}
	if err := v.Unmarshal(cfg); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	GlobalCfg = cfg
	return nil
}
