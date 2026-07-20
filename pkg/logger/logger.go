package logger

import (
	"os" // 标准库，用于操作文件和控制台输出

	"go-snake-game/pkg/config" // 项目配置包，提供 LogConfig 结构体

	"go.uber.org/zap"         // uber 高性能日志库
	"go.uber.org/zap/zapcore" // zap 底层核心组件，用于自定义 encoder、level、输出目标
)

// Log 全局日志实例，InitLogger 调用后赋值，全局可访问
var Log *zap.SugaredLogger

// InitLogger 根据配置初始化全局日志。
// 控制台模式（Console=true）使用带颜色的可读格式，否则使用 JSON 格式。
// 若 File 非空，同时输出到指定日志文件。
func InitLogger(cfg config.LogConfig) error {
	// 创建开发环境默认的 encoder 配置
	// 包含：时间戳、调用栈、级别等字段的默认格式
	encoderCfg := zap.NewDevelopmentEncoderConfig()

	// 将时间格式设为 ISO8601 标准格式（如 2026-07-05T16:30:00.000+0800）
	// 比默认的 epoch 时间戳更易读
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	// 声明 encoder 变量，后续根据 Console 模式选择不同实现
	var encoder zapcore.Encoder

	if cfg.Console {
		// 开发环境：日志级别带 ANSI 颜色（DEBUG=蓝色 INFO=绿色 WARN=黄色 ERROR=红色）
		encoderCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		// 控制台格式：人类可读的 key=value 形式，例如 "INFO  msg=服务启动  port=8080"
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	} else {
		// 生产环境：日志级别无颜色，纯大写字母
		encoderCfg.EncodeLevel = zapcore.CapitalLevelEncoder
		// JSON 格式：结构化日志，便于日志采集系统（如 ELK）解析
		// 例如 {"level":"INFO","msg":"服务启动","port":8080}
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	}

	// 将配置中的字符串级别（如 "info"）解析为 zapcore 内部级别常量
	// 低于此级别的日志将不会被输出（如设为 InfoLevel 则 Debug 日志被忽略）
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		// 解析失败（如配置写错成 "inf"），兜底使用 InfoLevel，避免程序崩溃
		level = zapcore.InfoLevel
	}

	// 收集所有日志输出目标（控制台 + 可选文件）
	var writers []zapcore.WriteSyncer

	// 始终输出到标准输出（控制台）
	// AddSync 将 io.Writer 包装为线程安全的 zapcore.WriteSyncer
	writers = append(writers, zapcore.AddSync(os.Stdout))

	// 如果配置文件输出路径（非空），追加文件输出
	if cfg.File != "" {
		// 打开日志文件：不存在则创建，只写模式，追加写入，权限 0644
		file, err := os.OpenFile(cfg.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// 文件打开失败直接返回错误，由调用方决定是否继续
			return err
		}
		// 将文件也包装为 WriteSyncer 加入输出列表
		writers = append(writers, zapcore.AddSync(file))
	}

	// 构建 zap 核心：encoder 决定格式，MultiWriteSyncer 同时写入多个目标，level 控制过滤
	core := zapcore.NewCore(encoder, zapcore.NewMultiWriteSyncer(writers...), level)

	// AddCaller() 开启调用方文件与行号记录
	// AddCallerSkip(1) 向上多跳一层堆栈，使得 Debug/Info 等全局函数
	// 输出的 caller 信息指向业务代码调用处，而非本包的 wrapper 函数
	// AddStacktrace(ErrorLevel) 使 Error 级别日志自动输出完整调用栈
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1), zap.AddStacktrace(zapcore.ErrorLevel))

	// Sugar() 将 Logger 转为 SugaredLogger，支持 printf 风格的格式化方法
	// 如 Debugw("msg", "key1", val1, "key2", val2) 键值对输出
	Log = logger.Sugar()

	return nil
}

// Debug 输出调试级别日志，常用于开发阶段排查问题
// 用法：logger.Debug("连接建立", "remote_addr", "127.0.0.1:8080")
func Debug(msg string, fields ...interface{}) {
	if Log == nil {
		return
	}
	// Debugw 为 SugaredLogger 的键值对方法，输出格式："msg"="连接建立" "remote_addr"="127.0.0.1:8080"
	Log.Debugw(msg, fields...)
}

// Info 输出信息级别日志，记录服务正常运行的关键节点
// 用法：logger.Info("服务启动", "port", 8080)
func Info(msg string, fields ...interface{}) {
	if Log == nil {
		return
	}
	Log.Infow(msg, fields...)
}

// Warn 输出警告级别日志，表示潜在问题但不影响当前流程
// 用法：logger.Warn("心跳超时", "timeout_sec", 60)
func Warn(msg string, fields ...interface{}) {
	if Log == nil {
		return
	}
	Log.Warnw(msg, fields...)
}

// Error 输出错误级别日志，记录影响业务执行的错误，自动携带调用栈。
// 用法：logger.Error("数据库连接失败", "err", err)
func Error(msg string, fields ...interface{}) {
	if Log == nil {
		return
	}
	// 追加调用栈信息，便于定位根因
	Log.Errorw(msg, fields...)
}

// WithTraceID 返回一个携带 trace_id 字段的子 Logger，用于链路追踪。
// 同一请求链路内共享该 Logger，所有日志自动附带 trace_id 便于聚合检索。
// 用法：traceLog := logger.WithTraceID("req-abc-123")
//
//	traceLog.Info("用户登录", "uid", 1001)
//	traceLog.Info("开始匹配", "mode", "rank")
func WithTraceID(traceID string) *zap.SugaredLogger {
	if Log == nil {
		return nil
	}
	// With 方法返回一个携带固定字段的新 SugaredLogger，不修改原实例
	return Log.With("trace_id", traceID)
}
