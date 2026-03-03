package optelsdk

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config 定义了OpenTelemetry SDK的配置选项
// 用于初始化追踪和日志功能
type Config struct {
	// ServiceName 服务名称，用于标识应用程序
	ServiceName string

	// ServiceVersion 服务版本号
	ServiceVersion string

	// Environment 运行环境（例如：dev, staging, production）
	Environment string

	// CollectorEndpoint OpenTelemetry Collector的gRPC端点地址
	// 例如：localhost:4317
	CollectorEndpoint string

	// EnableStdout 是否同时输出到标准输出（用于调试）
	EnableStdout bool

	// SampleRate 采样率，范围 0.0 到 1.0
	// 1.0 表示采样所有trace，0.5表示采样50%
	SampleRate float64
}

// SDK 是OpenTelemetry SDK的主要结构体
// 管理追踪器和日志器的生命周期
type SDK struct {
	config         *Config
	tracerProvider *sdktrace.TracerProvider
	logger         *Logger
	tracer         *TracerHelper
}

// NewSDK 创建并初始化一个新的OpenTelemetry SDK实例
//
// 参数:
//   - config: SDK配置选项
//
// 返回:
//   - *SDK: 初始化后的SDK实例
//   - error: 如果初始化失败，返回错误信息
//
// 示例:
//
//	sdk, err := optelsdk.NewSDK(&optelsdk.Config{
//	    ServiceName: "my-service",
//	    ServiceVersion: "1.0.0",
//	    Environment: "production",
//	    CollectorEndpoint: "localhost:4317",
//	    EnableStdout: false,
//	    SampleRate: 1.0,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer sdk.Shutdown(context.Background())
func NewSDK(config *Config) (*SDK, error) {
	// 验证配置
	if config.ServiceName == "" {
		return nil, fmt.Errorf("服务名称不能为空")
	}

	// 设置默认值
	if config.SampleRate == 0 {
		config.SampleRate = 1.0
	}
	if config.ServiceVersion == "" {
		config.ServiceVersion = "unknown"
	}
	if config.Environment == "" {
		config.Environment = "development"
	}

	sdk := &SDK{
		config: config,
	}

	// 初始化追踪器
	if err := sdk.initTracer(); err != nil {
		return nil, fmt.Errorf("初始化追踪器失败: %w", err)
	}

	// 初始化日志器
	sdk.logger = NewLogger(config.ServiceName)

	// 初始化追踪辅助器
	sdk.tracer = NewTracerHelper(config.ServiceName)

	return sdk, nil
}

// initTracer 初始化OpenTelemetry追踪器
// 配置导出器、采样器和资源信息
func (s *SDK) initTracer() error {
	ctx := context.Background()

	// 创建资源，包含服务的元数据信息
	res, err := resource.New(ctx,
		resource.WithAttributes(
			// 服务名称
			semconv.ServiceName(s.config.ServiceName),
			// 服务版本
			semconv.ServiceVersion(s.config.ServiceVersion),
			// 部署环境
			semconv.DeploymentEnvironment(s.config.Environment),
		),
	)
	if err != nil {
		return fmt.Errorf("创建资源失败: %w", err)
	}

	// 配置导出器列表
	var exporters []sdktrace.SpanExporter

	// 如果配置了Collector端点，创建OTLP导出器
	if s.config.CollectorEndpoint != "" {
		// 创建gRPC连接选项
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(s.config.CollectorEndpoint),
			otlptracegrpc.WithInsecure(), // 注意：生产环境应使用TLS
			otlptracegrpc.WithDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		}

		// 创建OTLP trace导出器
		otlpExporter, err := otlptracegrpc.New(ctx, opts...)
		if err != nil {
			return fmt.Errorf("创建OTLP导出器失败: %w", err)
		}
		exporters = append(exporters, otlpExporter)
	}

	// 如果启用了标准输出，创建stdout导出器（用于调试）
	if s.config.EnableStdout {
		stdoutExporter, err := stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
		if err != nil {
			return fmt.Errorf("创建标准输出导出器失败: %w", err)
		}
		exporters = append(exporters, stdoutExporter)
	}

	// 如果没有配置任何导出器，返回错误
	if len(exporters) == 0 {
		return fmt.Errorf("至少需要配置一个导出器（CollectorEndpoint或EnableStdout）")
	}

	// 创建批处理span处理器选项
	batchOptions := []sdktrace.BatchSpanProcessorOption{
		sdktrace.WithMaxQueueSize(2048),        // 最大队列大小
		sdktrace.WithMaxExportBatchSize(512),   // 最大批量导出大小
		sdktrace.WithBatchTimeout(5 * time.Second), // 批处理超时时间
	}

	// 为每个导出器创建批处理span处理器
	var spanProcessors []sdktrace.SpanProcessor
	for _, exporter := range exporters {
		spanProcessors = append(spanProcessors, sdktrace.NewBatchSpanProcessor(exporter, batchOptions...))
	}

	// 创建采样器
	var sampler sdktrace.Sampler
	if s.config.SampleRate >= 1.0 {
		// 采样所有trace
		sampler = sdktrace.AlwaysSample()
	} else if s.config.SampleRate <= 0.0 {
		// 不采样任何trace
		sampler = sdktrace.NeverSample()
	} else {
		// 按比例采样
		sampler = sdktrace.TraceIDRatioBased(s.config.SampleRate)
	}

	// 创建追踪器提供者选项
	tracerProviderOptions := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	}

	// 添加所有span处理器
	for _, processor := range spanProcessors {
		tracerProviderOptions = append(tracerProviderOptions, sdktrace.WithSpanProcessor(processor))
	}

	// 创建追踪器提供者
	s.tracerProvider = sdktrace.NewTracerProvider(tracerProviderOptions...)

	// 设置全局追踪器提供者
	otel.SetTracerProvider(s.tracerProvider)

	// 设置全局传播器，用于跨服务追踪
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return nil
}

// GetLogger 返回日志器实例
// 用于替代传统的控制台输出
//
// 返回:
//   - *Logger: 日志器实例
//
// 示例:
//
//	logger := sdk.GetLogger()
//	logger.Info("应用程序启动成功")
func (s *SDK) GetLogger() *Logger {
	return s.logger
}

// GetTracer 返回追踪辅助器实例
// 用于创建和管理分布式追踪
//
// 返回:
//   - *TracerHelper: 追踪辅助器实例
//
// 示例:
//
//	tracer := sdk.GetTracer()
//	ctx, span := tracer.Start(context.Background(), "operation-name")
//	defer span.End()
func (s *SDK) GetTracer() *TracerHelper {
	return s.tracer
}

// Shutdown 优雅地关闭SDK，确保所有数据都被导出
// 应该在应用程序退出前调用，通常使用defer
//
// 参数:
//   - ctx: 上下文，用于控制关闭超时
//
// 返回:
//   - error: 如果关闭过程中出现错误
//
// 示例:
//
//	defer sdk.Shutdown(context.Background())
func (s *SDK) Shutdown(ctx context.Context) error {
	if s.tracerProvider != nil {
		if err := s.tracerProvider.Shutdown(ctx); err != nil {
			return fmt.Errorf("关闭追踪器提供者失败: %w", err)
		}
	}
	return nil
}

// ForceFlush 强制刷新所有待处理的遥测数据
// 确保数据立即发送到collector
//
// 参数:
//   - ctx: 上下文，用于控制刷新超时
//
// 返回:
//   - error: 如果刷新过程中出现错误
//
// 示例:
//
//	if err := sdk.ForceFlush(context.Background()); err != nil {
//	    log.Printf("刷新失败: %v", err)
//	}
func (s *SDK) ForceFlush(ctx context.Context) error {
	if s.tracerProvider != nil {
		if err := s.tracerProvider.ForceFlush(ctx); err != nil {
			return fmt.Errorf("刷新追踪器提供者失败: %w", err)
		}
	}
	return nil
}
