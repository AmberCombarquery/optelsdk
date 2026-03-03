package optelsdk

import (
	"context"
	"fmt"
	"log"
	"time"

	"go.opentelemetry.io/otel/trace"
)

// LogLevel 定义日志级别
type LogLevel int

const (
	// DebugLevel 调试级别，用于详细的调试信息
	DebugLevel LogLevel = iota
	// InfoLevel 信息级别，用于一般的信息性消息
	InfoLevel
	// WarnLevel 警告级别，用于警告信息
	WarnLevel
	// ErrorLevel 错误级别，用于错误信息
	ErrorLevel
	// FatalLevel 致命错误级别，记录后程序将退出
	FatalLevel
)

// String 返回日志级别的字符串表示
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	case FatalLevel:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// Logger 日志器，用于替代传统的控制台输出
// 集成了OpenTelemetry追踪功能，可以将日志与trace关联
// 日志只会发送到OpenTelemetry Collector，不会输出到控制台
type Logger struct {
	serviceName   string
	minLevel      LogLevel
	enableConsole bool // 是否启用控制台输出（默认false）
}

// NewLogger 创建一个新的日志器实例
//
// 参数:
//   - serviceName: 服务名称，用于标识日志来源
//
// 返回:
//   - *Logger: 日志器实例
func NewLogger(serviceName string) *Logger {
	return &Logger{
		serviceName:   serviceName,
		minLevel:      InfoLevel, // 默认最小日志级别为Info
		enableConsole: false,     // 默认不输出到控制台，只发送到Collector
	}
}

// SetMinLevel 设置最小日志级别
// 只有大于或等于此级别的日志才会被输出
//
// 参数:
//   - level: 最小日志级别
//
// 示例:
//
//	logger.SetMinLevel(optelsdk.DebugLevel) // 输出所有级别的日志
func (l *Logger) SetMinLevel(level LogLevel) {
	l.minLevel = level
}

// EnableConsoleOutput 启用控制台输出（默认禁用）
// 启用后日志会同时发送到Collector和控制台
//
// 参数:
//   - enable: true启用控制台输出，false禁用
//
// 示例:
//
//	logger.EnableConsoleOutput(true) // 调试时启用控制台输出
func (l *Logger) EnableConsoleOutput(enable bool) {
	l.enableConsole = enable
}

// log 内部日志方法，处理实际的日志输出
//
// 参数:
//   - ctx: 上下文对象，用于提取trace信息
//   - level: 日志级别
//   - format: 格式化字符串
//   - args: 格式化参数
func (l *Logger) log(ctx context.Context, level LogLevel, format string, args ...interface{}) {
	// 检查日志级别
	if level < l.minLevel {
		return
	}

	// 格式化消息
	message := fmt.Sprintf(format, args...)

	// 获取当前时间
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")

	// 提取trace信息
	traceID := ""
	spanID := ""
	if ctx != nil {
		span := trace.SpanFromContext(ctx)
		if span.SpanContext().IsValid() {
			traceID = span.SpanContext().TraceID().String()
			spanID = span.SpanContext().SpanID().String()
		}
	}

	// 如果有span，将日志作为事件添加到span中（发送到Collector）
	if ctx != nil {
		span := trace.SpanFromContext(ctx)
		if span.SpanContext().IsValid() {
			// 将日志作为Span Event发送到Collector
			span.AddEvent(message)
		}
	}

	// 只在启用控制台输出时才打印到控制台
	if l.enableConsole {
		// 构建日志输出
		logMessage := fmt.Sprintf("[%s] [%s] [%s]", timestamp, level.String(), l.serviceName)

		// 如果有trace信息，添加到日志中
		if traceID != "" {
			logMessage += fmt.Sprintf(" [trace_id=%s] [span_id=%s]", traceID, spanID)
		}

		logMessage += fmt.Sprintf(" %s", message)

		// 输出到控制台
		log.Println(logMessage)
	}
}

// Debug 输出调试级别的日志
// 用于详细的调试信息，通常在开发环境使用
//
// 参数:
//   - ctx: 上下文对象
//   - format: 格式化字符串
//   - args: 格式化参数
//
// 示例:
//
//	logger.Debug(ctx, "用户ID: %d, 操作: %s", userID, operation)
func (l *Logger) Debug(ctx context.Context, format string, args ...interface{}) {
	l.log(ctx, DebugLevel, format, args...)
}

// Info 输出信息级别的日志
// 用于一般的信息性消息，记录应用程序的正常运行状态
//
// 参数:
//   - ctx: 上下文对象
//   - format: 格式化字符串
//   - args: 格式化参数
//
// 示例:
//
//	logger.Info(ctx, "服务启动成功，监听端口: %d", port)
func (l *Logger) Info(ctx context.Context, format string, args ...interface{}) {
	l.log(ctx, InfoLevel, format, args...)
}

// Warn 输出警告级别的日志
// 用于警告信息，表示可能存在问题但不影响程序运行
//
// 参数:
//   - ctx: 上下文对象
//   - format: 格式化字符串
//   - args: 格式化参数
//
// 示例:
//
//	logger.Warn(ctx, "缓存未命中，将从数据库加载数据")
func (l *Logger) Warn(ctx context.Context, format string, args ...interface{}) {
	l.log(ctx, WarnLevel, format, args...)
}

// Error 输出错误级别的日志
// 用于错误信息，表示发生了错误但程序可以继续运行
//
// 参数:
//   - ctx: 上下文对象
//   - format: 格式化字符串
//   - args: 格式化参数
//
// 示例:
//
//	logger.Error(ctx, "数据库连接失败: %v", err)
func (l *Logger) Error(ctx context.Context, format string, args ...interface{}) {
	l.log(ctx, ErrorLevel, format, args...)
}

// Fatal 输出致命错误级别的日志
// 用于致命错误，记录后程序将退出
//
// 参数:
//   - ctx: 上下文对象
//   - format: 格式化字符串
//   - args: 格式化参数
//
// 示例:
//
//	logger.Fatal(ctx, "无法加载配置文件: %v", err)
func (l *Logger) Fatal(ctx context.Context, format string, args ...interface{}) {
	l.log(ctx, FatalLevel, format, args...)
	// 致命错误后退出程序
	log.Fatal("程序因致命错误退出")
}

// WithError 输出错误日志并返回错误对象
// 这是一个便捷方法，用于同时记录错误和返回错误
//
// 参数:
//   - ctx: 上下文对象
//   - err: 错误对象
//   - format: 格式化字符串
//   - args: 格式化参数
//
// 返回:
//   - error: 原始错误对象
//
// 示例:
//
//	if err != nil {
//	    return logger.WithError(ctx, err, "处理请求失败")
//	}
func (l *Logger) WithError(ctx context.Context, err error, format string, args ...interface{}) error {
	if err != nil {
		message := fmt.Sprintf(format, args...)
		l.Error(ctx, "%s: %v", message, err)
	}
	return err
}

// InfoWithoutContext 输出信息级别的日志（不需要上下文）
// 用于在没有上下文的情况下输出日志
//
// 参数:
//   - format: 格式化字符串
//   - args: 格式化参数
//
// 示例:
//
//	logger.InfoWithoutContext("应用程序启动中...")
func (l *Logger) InfoWithoutContext(format string, args ...interface{}) {
	l.log(nil, InfoLevel, format, args...)
}

// ErrorWithoutContext 输出错误级别的日志（不需要上下文）
// 用于在没有上下文的情况下输出错误日志
//
// 参数:
//   - format: 格式化字符串
//   - args: 格式化参数
//
// 示例:
//
//	logger.ErrorWithoutContext("初始化失败: %v", err)
func (l *Logger) ErrorWithoutContext(format string, args ...interface{}) {
	l.log(nil, ErrorLevel, format, args...)
}

// WarnWithoutContext 输出警告级别的日志（不需要上下文）
// 用于在没有上下文的情况下输出警告日志
//
// 参数:
//   - format: 格式化字符串
//   - args: 格式化参数
//
// 示例:
//
//	logger.WarnWithoutContext("配置项缺失，使用默认值")
func (l *Logger) WarnWithoutContext(format string, args ...interface{}) {
	l.log(nil, WarnLevel, format, args...)
}

// DebugWithoutContext 输出调试级别的日志（不需要上下文）
// 用于在没有上下文的情况下输出调试日志
//
// 参数:
//   - format: 格式化字符串
//   - args: 格式化参数
//
// 示例:
//
//	logger.DebugWithoutContext("加载配置: %+v", config)
func (l *Logger) DebugWithoutContext(format string, args ...interface{}) {
	l.log(nil, DebugLevel, format, args...)
}
