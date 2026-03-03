package main

import (
	"context"
	"errors"
	"time"

	"github.com/AmberCombarquery/optelsdk"

	"go.opentelemetry.io/otel/trace"
)

// MapCarrier 实现 propagation.TextMapCarrier 接口
// 用于在 HTTP Headers 或其他 map 中传递 Trace Context
type MapCarrier map[string]string

// Get 获取指定 key 的值
func (m MapCarrier) Get(key string) string {
	return m[key]
}

// Set 设置指定 key 的值
func (m MapCarrier) Set(key, value string) {
	m[key] = value
}

// Keys 返回所有的 key
func (m MapCarrier) Keys() []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func main() {
	// 1. 创建并初始化SDK
	// 配置OpenTelemetry SDK，连接到Collector
	// 注意：日志和追踪数据只会发送到Collector，不会输出到控制台
	sdk, err := optelsdk.NewSDK(&optelsdk.Config{
		ServiceName:       "example-service", // 服务名称
		ServiceVersion:    "1.0.0",           // 服务版本
		Environment:       "development",     // 运行环境
		CollectorEndpoint: "localhost:4317",  // Collector的gRPC端点
		SampleRate:        1.0,               // 采样率100%
	})
	if err != nil {
		panic(err)
	}
	// 确保程序退出前关闭SDK，刷新所有待处理的数据
	defer sdk.Shutdown(context.Background())

	// 2. 获取日志器和追踪器
	logger := sdk.GetLogger()
	tracer := sdk.GetTracer()

	// 可选：如果需要调试，可以启用控制台输出
	// logger.EnableConsoleOutput(true)

	// 3. 使用日志器替代传统的控制台输出
	// 日志会作为Span Event发送到Collector，不会输出到控制台
	logger.InfoWithoutContext("应用程序启动成功")

	// 4. 创建根上下文
	ctx := context.Background()

	// 5. 示例1: 基本的追踪使用
	demonstrateBasicTracing(ctx, logger, tracer)

	// 6. 示例2: HTTP请求追踪
	demonstrateHTTPTracing(ctx, logger, tracer)

	// 7. 示例3: 数据库操作追踪
	demonstrateDBTracing(ctx, logger, tracer)

	// 8. 示例4: 嵌套追踪
	demonstrateNestedTracing(ctx, logger, tracer)

	// 9. 示例5: 错误处理
	demonstrateErrorHandling(ctx, logger, tracer)

	// 10. 示例6: 跨服务追踪（模拟多机多服务间的 Trace 传递）
	demonstrateCrossServiceTracing(ctx, logger, tracer)

	// 11. 强制刷新所有数据到collector
	if err := sdk.ForceFlush(context.Background()); err != nil {
		logger.ErrorWithoutContext("刷新数据失败: %v", err)
	}

	logger.InfoWithoutContext("所有示例执行完成")
}

// demonstrateBasicTracing 演示基本的追踪功能
func demonstrateBasicTracing(ctx context.Context, logger *optelsdk.Logger, tracer *optelsdk.TracerHelper) {
	// 开始一个新的span
	ctx, span := tracer.Start(ctx, "基本追踪示例")
	defer span.End()

	// 在span中记录日志
	logger.Info(ctx, "这是一条带有trace信息的日志")

	// 为span添加属性
	tracer.SetAttributes(span,
		"user.id", "12345",
		"user.name", "张三",
		"operation", "查询用户信息",
	)

	// 添加事件
	tracer.AddEvent(span, "开始处理用户请求")

	// 模拟业务逻辑
	time.Sleep(100 * time.Millisecond)

	tracer.AddEvent(span, "用户请求处理完成")
	logger.Info(ctx, "基本追踪示例执行完成")
}

// demonstrateHTTPTracing 演示HTTP请求追踪
func demonstrateHTTPTracing(ctx context.Context, logger *optelsdk.Logger, tracer *optelsdk.TracerHelper) {
	// 使用TraceHTTPRequest方法追踪HTTP请求
	err := tracer.TraceHTTPRequest(ctx, "GET", "/api/users/12345", func(ctx context.Context, span trace.Span) error {
		logger.Info(ctx, "处理HTTP GET请求")

		// 模拟HTTP请求处理
		time.Sleep(50 * time.Millisecond)

		// 添加HTTP响应信息
		tracer.SetAttributes(span,
			"http.status_code", 200,
			"http.response_size", 1024,
		)

		logger.Info(ctx, "HTTP请求处理成功")
		return nil
	})

	if err != nil {
		logger.Error(ctx, "HTTP请求追踪失败: %v", err)
	}
}

// demonstrateDBTracing 演示数据库操作追踪
func demonstrateDBTracing(ctx context.Context, logger *optelsdk.Logger, tracer *optelsdk.TracerHelper) {
	// 使用TraceDBOperation方法追踪数据库操作
	err := tracer.TraceDBOperation(ctx, "mysql", "SELECT", "users", func(ctx context.Context, span trace.Span) error {
		logger.Info(ctx, "执行数据库查询")

		// 模拟数据库查询
		time.Sleep(80 * time.Millisecond)

		// 添加查询结果信息
		tracer.SetAttributes(span,
			"db.rows_affected", 10,
			"db.query_time_ms", 80,
		)

		logger.Info(ctx, "数据库查询完成，返回10条记录")
		return nil
	})

	if err != nil {
		logger.Error(ctx, "数据库操作追踪失败: %v", err)
	}
}

// demonstrateNestedTracing 演示嵌套追踪
func demonstrateNestedTracing(ctx context.Context, logger *optelsdk.Logger, tracer *optelsdk.TracerHelper) {
	// 父span
	ctx, parentSpan := tracer.Start(ctx, "处理订单")
	defer parentSpan.End()

	logger.Info(ctx, "开始处理订单")

	// 子span 1: 验证订单
	ctx, validateSpan := tracer.StartWithAttributes(ctx, "验证订单",
		"order.id", "ORD-12345",
	)
	logger.Info(ctx, "验证订单信息")
	time.Sleep(30 * time.Millisecond)
	validateSpan.End()

	// 子span 2: 扣减库存
	ctx, inventorySpan := tracer.Start(ctx, "扣减库存")
	logger.Info(ctx, "扣减商品库存")
	time.Sleep(40 * time.Millisecond)
	inventorySpan.End()

	// 子span 3: 创建支付订单
	ctx, paymentSpan := tracer.Start(ctx, "创建支付订单")
	logger.Info(ctx, "创建支付订单")
	time.Sleep(50 * time.Millisecond)
	paymentSpan.End()

	logger.Info(ctx, "订单处理完成")
}

// demonstrateErrorHandling 演示错误处理
func demonstrateErrorHandling(ctx context.Context, logger *optelsdk.Logger, tracer *optelsdk.TracerHelper) {
	// 使用TraceFunction追踪函数执行
	err := tracer.TraceFunction(ctx, "处理支付", func(ctx context.Context, span trace.Span) error {
		logger.Info(ctx, "开始处理支付")

		// 模拟业务逻辑
		time.Sleep(60 * time.Millisecond)

		// 模拟一个错误
		err := errors.New("支付网关超时")
		if err != nil {
			// 使用logger的WithError方法记录错误
			return logger.WithError(ctx, err, "支付处理失败")
		}

		return nil
	})

	if err != nil {
		logger.Error(ctx, "支付流程出错: %v", err)
	}

	// 演示不同级别的日志
	logger.Debug(ctx, "这是一条调试日志")
	logger.Info(ctx, "这是一条信息日志")
	logger.Warn(ctx, "这是一条警告日志")
	logger.Error(ctx, "这是一条错误日志")
}

// demonstrateCrossServiceTracing 演示跨服务追踪
// 模拟服务A调用服务B，服务B再调用服务C的场景
func demonstrateCrossServiceTracing(ctx context.Context, logger *optelsdk.Logger, tracer *optelsdk.TracerHelper) {
	// ========== 服务A：发起请求 ==========
	ctx, spanA := tracer.Start(ctx, "服务A-处理用户请求")
	defer spanA.End()

	logger.Info(ctx, "【服务A】收到用户请求")

	// 获取当前的 Trace ID 和 Span ID
	traceID := spanA.SpanContext().TraceID().String()
	spanID := spanA.SpanContext().SpanID().String()
	logger.Info(ctx, "【服务A】生成 TraceID: %s, SpanID: %s", traceID, spanID)

	// 模拟准备调用服务B的HTTP请求
	// 创建一个 MapCarrier 来模拟 HTTP Headers
	headers := make(MapCarrier)

	// 将 Trace Context 注入到 HTTP Headers 中
	// 这是关键步骤：使用 InjectContext 将追踪信息注入到请求头
	tracer.InjectContext(ctx, headers)

	logger.Info(ctx, "【服务A】注入 Trace Context 到 HTTP Headers:")
	for key, value := range headers {
		logger.Info(ctx, "  %s: %s", key, value)
	}

	// 模拟发送 HTTP 请求到服务B
	time.Sleep(20 * time.Millisecond)
	logger.Info(ctx, "【服务A】调用服务B...")

	// ========== 服务B：接收请求 ==========
	// 模拟服务B接收到请求，从 HTTP Headers 中提取 Trace Context
	ctxB := context.Background()

	// 从 HTTP Headers 中提取 Trace Context
	// 这是关键步骤：使用 ExtractContext 从请求头中提取追踪信息
	ctxB = tracer.ExtractContext(ctxB, headers)

	// 创建服务B的 Span，它会自动成为服务A的子 Span
	ctxB, spanB := tracer.Start(ctxB, "服务B-查询订单信息")
	defer spanB.End()

	// 验证 Trace ID 是否一致（应该与服务A相同）
	traceBID := spanB.SpanContext().TraceID().String()
	spanBID := spanB.SpanContext().SpanID().String()
	logger.Info(ctxB, "【服务B】接收请求，TraceID: %s, SpanID: %s", traceBID, spanBID)

	if traceID == traceBID {
		logger.Info(ctxB, "【服务B】✓ Trace ID 匹配成功！与服务A在同一个 Trace 中")
	}

	tracer.SetAttributes(spanB, "service", "service-b", "operation", "query_order")

	// 模拟服务B的业务逻辑
	time.Sleep(30 * time.Millisecond)

	// 服务B准备调用服务C
	headersC := make(MapCarrier)
	tracer.InjectContext(ctxB, headersC)
	logger.Info(ctxB, "【服务B】调用服务C...")

	// ========== 服务C：接收请求 ==========
	ctxC := context.Background()
	ctxC = tracer.ExtractContext(ctxC, headersC)

	ctxC, spanC := tracer.Start(ctxC, "服务C-查询库存")
	defer spanC.End()

	traceCID := spanC.SpanContext().TraceID().String()
	spanCID := spanC.SpanContext().SpanID().String()
	logger.Info(ctxC, "【服务C】接收请求，TraceID: %s, SpanID: %s", traceCID, spanCID)

	if traceID == traceCID {
		logger.Info(ctxC, "【服务C】✓ Trace ID 匹配成功！与服务A、服务B在同一个 Trace 中")
	}

	tracer.SetAttributes(spanC, "service", "service-c", "operation", "query_inventory")

	// 模拟服务C的业务逻辑
	time.Sleep(25 * time.Millisecond)
	logger.Info(ctxC, "【服务C】库存查询完成")

	// ========== 服务B：处理服务C的响应 ==========
	logger.Info(ctxB, "【服务B】收到服务C响应，订单信息查询完成")

	// ========== 服务A：处理服务B的响应 ==========
	logger.Info(ctx, "【服务A】收到服务B响应，用户请求处理完成")

	// 总结
	logger.Info(ctx, "========== 跨服务追踪总结 ==========")
	logger.Info(ctx, "完整的调用链：服务A -> 服务B -> 服务C")
	logger.Info(ctx, "所有服务共享同一个 TraceID: %s", traceID)
	logger.Info(ctx, "在 Jaeger UI 中可以看到完整的调用链和时序关系")
}

// ========== 实际应用示例 ==========
//
// 在真实的微服务场景中，你需要：
//
// 1. 服务A（发送方）：
//    ```go
//    // 创建 HTTP 请求
//    req, _ := http.NewRequest("GET", "http://service-b:8080/api/order", nil)
//
//    // 注入 Trace Context 到 HTTP Headers
//    tracer.InjectContext(ctx, req.Header)
//
//    // 发送请求
//    resp, err := http.DefaultClient.Do(req)
//    ```
//
// 2. 服务B（接收方）：
//    ```go
//    func handleRequest(w http.ResponseWriter, r *http.Request) {
//        // 从 HTTP Headers 提取 Trace Context
//        ctx := tracer.ExtractContext(r.Context(), r.Header)
//
//        // 创建 Span（自动成为上游服务的子 Span）
//        ctx, span := tracer.Start(ctx, "处理订单请求")
//        defer span.End()
//
//        // 业务逻辑...
//        logger.Info(ctx, "处理订单")
//    }
//    ```
//
// 3. gRPC 场景：
//    ```go
//    // 客户端
//    md := metadata.New(nil)
//    tracer.InjectContext(ctx, md)
//    ctx = metadata.NewOutgoingContext(ctx, md)
//
//    // 服务端
//    md, _ := metadata.FromIncomingContext(ctx)
//    ctx = tracer.ExtractContext(ctx, md)
//    ```
