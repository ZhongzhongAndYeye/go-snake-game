package config

// AppConfig 顶层总配置结构体
type AppConfig struct {
	Log     LogConfig     `mapstructure:"log"`     // 日志配置
	Gateway GatewayConfig `mapstructure:"gateway"` // 网关配置
	Login   LoginConfig   `mapstructure:"login"`   // 登录服配置
	Game    GameConfig    `mapstructure:"game"`    // 游戏逻辑服配置
	Mysql   MysqlConfig   `mapstructure:"mysql"`   // MySQL 数据库配置
	Redis   RedisConfig   `mapstructure:"redis"`   // Redis 缓存配置
}

// LogConfig 日志配置
type LogConfig struct {
	Level   string `mapstructure:"level"`   // 日志级别: debug | info | warn | error
	Console bool   `mapstructure:"console"` // 是否输出到控制台
	File    string `mapstructure:"file"`    // 日志文件路径，为空表示不输出到文件
}

// GatewayConfig 网关服务 — WebSocket 接入层
type GatewayConfig struct {
	ListenAddr       string `mapstructure:"listen_addr"`       // WebSocket 监听地址
	LoginRpcAddr     string `mapstructure:"login_rpc_addr"`    // 登录服 gRPC 地址
	GameRpcAddr      string `mapstructure:"game_rpc_addr"`     // 游戏服 gRPC 地址
	HeartbeatTimeout int    `mapstructure:"heartbeat_timeout"` // 心跳超时（秒），超时断开连接
}

// LoginConfig 登录服 — 账号认证、登录流程
type LoginConfig struct {
	GrpcAddr string `mapstructure:"grpc_addr"` // gRPC 服务监听地址
}

// GameConfig 游戏逻辑服 — 对战逻辑
type GameConfig struct {
	GrpcAddr string `mapstructure:"grpc_addr"` // gRPC 服务监听地址
}

// MysqlConfig MySQL 数据库配置
type MysqlConfig struct {
	DSN            string `mapstructure:"dsn"`              // 数据库连接串
	MaxOpen        int    `mapstructure:"max_open"`         // 最大连接数
	MaxIdle        int    `mapstructure:"max_idle"`         // 最大空闲连接数
	MaxLifeMinutes int    `mapstructure:"max_life_minutes"` // 连接最大存活时间（分钟）
}

// RedisConfig Redis 缓存配置
type RedisConfig struct {
	Addr         string `mapstructure:"addr"`           // Redis 地址
	DB           int    `mapstructure:"db"`             // 使用的数据库编号
	Password     string `mapstructure:"password"`       // 密码，为空表示无密码
	PoolSize     int    `mapstructure:"pool_size"`      // 连接池最大连接数
	MinIdleConns int    `mapstructure:"min_idle_conns"` // 最小空闲连接数
	MaxRetries   int    `mapstructure:"max_retries"`    // 操作失败最大重试次数
	DialTimeout  int    `mapstructure:"dial_timeout"`   // 连接超时（秒）
	ReadTimeout  int    `mapstructure:"read_timeout"`   // 读超时（秒）
	WriteTimeout int    `mapstructure:"write_timeout"`  // 写超时（秒）
	PoolTimeout  int    `mapstructure:"pool_timeout"`   // 从连接池获取连接的超时时间（秒）
}
