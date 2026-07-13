package db

import (
	"context"
	"strings"
	"sync"
	"time"

	"go-snake-game/pkg/logger"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// GlobalDB 全局数据库实例，InitMySQL 调用后赋值，全局可访问。
var GlobalDB *gorm.DB

var (
	initOnce sync.Once
)

// MySQLConfig MySQL 数据库连接配置。
type MySQLConfig struct {
	DSN            string // DSN 数据库连接串，格式：user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
	MaxOpen        int    // 最大打开连接数，控制连接池的容量上限
	MaxIdle        int    // 最大空闲连接数，连接池中保留的空闲连接数量
	MaxLifeMinutes int    // 连接最大存活时间（分钟）。
}

// InitMySQL 初始化 MySQL 数据库连接，将实例赋值给全局变量 GlobalDB。
func InitMySQL(cfg *MySQLConfig) {
	initOnce.Do(func() {
		// 校验 DSN 配置是否包含`parseTime=True&loc=Local`，否则 time.Time 字段解析失败
		if err := validateDSN(cfg.DSN); err != nil {
			logger.Error("MySQL DSN 配置校验失败", "error", err.Error())
			panic("DSN 配置校验失败: " + err.Error())
		}

		db, err := gorm.Open(mysql.Open(cfg.DSN), &gorm.Config{
			// 配置 GORM 日志：使用项目统一的 logger 包输出
			Logger: &gormLogger{
				// LogMode 设 4 表示打印所有 SQL（包括查询、执行、慢查询）
				// 开发环境便于调试，生产环境可改为 gormlogger.Error 只打印错误
				LogLevel: gormlogger.Info,
			},
		})
		if err != nil {
			logger.Error("MySQL 数据库初始化失败", "error", err.Error())
			panic("数据库初始化失败: " + err.Error())
		}

		// 第二步：获取底层 sql.DB 并设置连接池参数
		sqlDB, err := db.DB()
		if err != nil {
			logger.Error("获取底层 sql.DB 失败", "error", err.Error())
			panic("获取数据库连接池失败: " + err.Error())
		}

		// 最大打开连接数：控制同时与 MySQL 建立的最大连接数
		// 超出此数量后新建连接会阻塞，直到有连接被释放回池中
		sqlDB.SetMaxOpenConns(cfg.MaxOpen)

		// 最大空闲连接数：连接池中保留的空闲连接数
		// 空闲连接超过此数量会被回收，低于此数量时连接不会被释放
		sqlDB.SetMaxIdleConns(cfg.MaxIdle)

		// 连接最大存活时间：超过此时间的连接会被关闭并重建
		// 避免 MySQL 服务端 wait_timeout 参数导致服务端主动断开长时间未使用的连接
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.MaxLifeMinutes) * time.Minute)

		// 第三步：Ping 验证数据库连通性
		// 确保配置的 DSN 正确、网络可达、数据库已启动
		if err := sqlDB.Ping(); err != nil {
			logger.Error("MySQL 数据库连通性验证失败", "error", err.Error())
			panic("数据库 Ping 失败: " + err.Error())
		}

		// 第四步：赋值全局变量
		GlobalDB = db

		logger.Info("MySQL 数据库初始化成功",
			"max_open", cfg.MaxOpen,
			"max_idle", cfg.MaxIdle,
			"max_life_minutes", cfg.MaxLifeMinutes,
		)
	})
}

// DSNValidationError DSN 校验错误类型，用于标识配置错误。
type DSNValidationError struct {
	msg string
}

// 实现 error 接口，DSNValidationError 即成为一个error错误类型
func (e *DSNValidationError) Error() string {
	return e.msg
}

// 预定义的 DSN 校验错误
var (
	ErrMissingParseTime = &DSNValidationError{msg: "DSN 必须包含 parseTime=True，否则 time.Time 字段解析失败"}
	ErrMissingLocLocal  = &DSNValidationError{msg: "DSN 必须包含 loc=Local，否则时区解析不正确"}
)

// validateDSN 校验 DSN 连接串是否包含必要的参数。
// 必须包含 parseTime=True 和 loc=Local，否则 time.Time 字段解析会失败或时区不正确。
func validateDSN(dsn string) error {
	// parseTime=True: 告诉 MySQL 驱动将时间字符串转换为 Go 的 time.Time 对象
	// 如果缺少此参数，time.Time 字段将解析为零值 (0001-01-01 00:00:00)
	if !strings.Contains(dsn, "parseTime=True") {
		return ErrMissingParseTime
	}
	// loc=Local: 使用服务器本地时区解析时间
	// 如果缺少此参数，MySQL 驱动默认使用 UTC 时区，会导致时间差 8 小时（东八区）
	if !strings.Contains(dsn, "loc=Local") {
		return ErrMissingLocLocal
	}
	return nil
}

//  ======================== gormLogger GORM 日志适配器，将 GORM 的日志输出重定向到项目统一的 logger 包。===============

// 实现 gormlogger.Interface 接口，GORM 在执行 SQL 时会自动调用其方法。
type gormLogger struct {
	LogLevel gormlogger.LogLevel
}

// LogMode 设置日志级别，返回新的 logger 实例。
func (l *gormLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

// Info 打印 Info 级别日志。
func (l *gormLogger) Info(_ context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Info {
		logger.Info("[GORM] "+msg, "data", data)
	}
}

// Warn 打印 Warn 级别日志。
func (l *gormLogger) Warn(_ context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Warn {
		logger.Warn("[GORM] "+msg, "data", data)
	}
}

// Error 打印 Error 级别日志。
func (l *gormLogger) Error(_ context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= gormlogger.Error {
		logger.Error("[GORM] "+msg, "data", data)
	}
}

// Trace 打印 SQL 执行日志，包含执行耗时、影响行数、SQL 语句。
// 这是 GORM 日志接口的核心方法，每条 SQL 执行后都会调用。
func (l *gormLogger) Trace(_ context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.LogLevel <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	switch {
	case err != nil && l.LogLevel >= gormlogger.Error:
		// SQL 执行出错，打印错误日志
		logger.Error("[GORM] SQL 执行失败",
			"elapsed_ms", elapsed.Milliseconds(),
			"rows", rows,
			"sql", sql,
			"error", err.Error(),
		)
	case elapsed > 200*time.Millisecond && l.LogLevel >= gormlogger.Warn:
		// 慢查询：超过 200ms 打印警告日志
		logger.Warn("[GORM] 慢查询",
			"elapsed_ms", elapsed.Milliseconds(),
			"rows", rows,
			"sql", sql,
		)
	case l.LogLevel >= gormlogger.Info:
		// 正常 SQL，打印 Info 日志
		logger.Debug("[GORM] SQL",
			"elapsed_ms", elapsed.Milliseconds(),
			"rows", rows,
			"sql", sql,
		)
	}
}

// ps     调用频率
// Trace  非常频繁 （每次操作都有）
// Info   较少    （管理操作）
// Warn   偶尔    （出小问题）
// Error  很少    （出大问题）
