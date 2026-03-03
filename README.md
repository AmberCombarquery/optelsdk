# OpenTelemetry Go SDK

一个简单易用的 OpenTelemetry Go SDK，用于日志追踪和分布式追踪，可以替代传统的控制台输出，并能够接入 OpenTelemetry Collector。

## 功能特性

- ✅ **分布式追踪**: 完整的 OpenTelemetry 追踪功能，支持 span 创建、嵌套和属性管理
- ✅ **结构化日志**: 集成追踪信息的日志系统，替代传统的 `fmt.Println` 和 `log.Print`
- ✅ **Collector 集成**: 支持通过 gRPC 连接到 OpenTelemetry Collector
- ✅ **详细注释**: 所有公开 API 都有详细的中文注释和使用示例
- ✅ **便捷方法**: 提供 HTTP 请求、数据库操作等常见场景的追踪辅助方法
- ✅ **错误处理**: 自动记录错误到 span，便于问题排查

## 快速开始

### 1. 初始化 SDK

```go
package main

import (
    "context"
    "github.com/scwl/optelsdk"
)

func main() {
    // 创建并初始化 SDK
    sdk, err := optelsdk.NewSDK(&optelsdk.Config{
        ServiceName:       "my-service",        // 服务名称
        ServiceVersion:    "1.0.0",             // 服务版本
        Environment:       "production",        // 运行环境
        CollectorEndpoint: "localhost:4317",    // Collector 地址
        EnableStdout:      false,               // 是否同时输出到控制台
        SampleRate:        1.0,                 // 采样率 (0.0-1.0)
    })
    if err != nil {
        panic(err)
    }
    defer sdk.Shutdown(context.Background())

    // 获取日志器和追踪器
    logger := sdk.GetLogger()
    tracer := sdk.GetTracer()

    // 使用日志器替代 fmt.Println
    logger.InfoWithoutContext("应用程序启动成功")
}
```

### 2. 使用日志器

日志器可以完全替代传统的控制台输出，并且会自动关联追踪信息。

```go
// 不需要上下文的日志
logger.InfoWithoutContext("服务器启动，监听端口: %d", 8080)
logger.ErrorWithoutContext("配置加载失败: %v", err)

// 带上下文的日志（会自动包含 trace_id 和 span_id）
logger.Info(ctx, "处理用户请求，用户ID: %s", userID)
logger.Warn(ctx, "缓存未命中，将从数据库加载")
logger.Error(ctx, "数据库查询失败: %v", err)

// 设置日志级别
logger.SetMinLevel(optelsdk.DebugLevel)
logger.Debug(ctx, "调试信息: %+v", data)
```

### 3. 基本追踪

```go
// 创建一个 span
ctx, span := tracer.Start(ctx, "处理订单")
defer span.End()

// 添加属性
tracer.SetAttributes(span,
    "order.id", "12345",
    "user.id", "67890",
)

// 添加事件
tracer.AddEvent(span, "订单验证完成")

// 记录日志（会自动关联到 span）
logger.Info(ctx, "订单处理中...")
```

### 4. HTTP 请求追踪

```go
err := tracer.TraceHTTPRequest(ctx, "GET", "/api/users", func(ctx context.Context, span trace.Span) error {
    // 执行 HTTP 请求
    resp, err := http.Get("https://api.example.com/users")
    if err != nil {
        return err
    }
    defer resp.Body.Close()

    // 添加响应信息
    tracer.SetAttributes(span, "http.status_code", resp.StatusCode)
    logger.Info(ctx, "HTTP 请求成功，状态码: %d", resp.StatusCode)
  
    return nil
})
```

### 5. 数据库操作追踪

```go
err := tracer.TraceDBOperation(ctx, "mysql", "SELECT", "users", func(ctx context.Context, span trace.Span) error {
    // 执行数据库查询
    rows, err := db.QueryContext(ctx, "SELECT * FROM users WHERE id = ?", userID)
    if err != nil {
        return err
    }
    defer rows.Close()

    // 添加查询结果信息
    tracer.SetAttributes(span, "db.rows_affected", rowCount)
    logger.Info(ctx, "查询完成，返回 %d 条记录", rowCount)
  
    return nil
})
```

### 6. 函数追踪

```go
err := tracer.TraceFunction(ctx, "处理支付", func(ctx context.Context, span trace.Span) error {
    logger.Info(ctx, "开始处理支付")
  
    // 业务逻辑
    if err := processPayment(ctx); err != nil {
        return err // 错误会自动记录到 span
    }
  
    tracer.SetAttributes(span, "payment.amount", 100.00)
    return nil
})
```

### 7. 嵌套追踪

```go
// 父 span
ctx, parentSpan := tracer.Start(ctx, "处理订单")
defer parentSpan.End()

logger.Info(ctx, "开始处理订单")

// 子 span 1
ctx, span1 := tracer.Start(ctx, "验证订单")
logger.Info(ctx, "验证订单信息")
// ... 业务逻辑
span1.End()

// 子 span 2
ctx, span2 := tracer.Start(ctx, "扣减库存")
logger.Info(ctx, "扣减商品库存")
// ... 业务逻辑
span2.End()

logger.Info(ctx, "订单处理完成")
```

### 8. 错误处理

```go
ctx, span := tracer.Start(ctx, "数据库操作")
defer span.End()

result, err := db.Query(ctx, "SELECT ...")
if err != nil {
    // 记录错误到 span
    tracer.RecordError(span, err, "数据库查询失败")
  
    // 同时记录错误日志
    logger.Error(ctx, "查询失败: %v", err)
    return err
}

// 或者使用 WithError 方法
if err != nil {
    return logger.WithError(ctx, err, "操作失败")
}
```

## 配置说明

### Config 结构体

```go
type Config struct {
    // ServiceName 服务名称（必填）
    ServiceName string

    // ServiceVersion 服务版本号（可选，默认 "unknown"）
    ServiceVersion string

    // Environment 运行环境（可选，默认 "development"）
    // 例如: "development", "staging", "production"
    Environment string

    // CollectorEndpoint OpenTelemetry Collector 的 gRPC 端点地址
    // 例如: "localhost:4317" 或 "otel-collector.example.com:4317"
    CollectorEndpoint string

    // EnableStdout 是否同时输出到标准输出（用于调试）
    // 设置为 true 时，trace 数据会同时输出到控制台
    EnableStdout bool

    // SampleRate 采样率，范围 0.0 到 1.0
    // 1.0 表示采样所有 trace，0.5 表示采样 50%
    // 默认值为 1.0
    SampleRate float64
}
```

## API 参考

### SDK 方法

- `NewSDK(config *Config) (*SDK, error)` - 创建 SDK 实例
- `GetLogger() *Logger` - 获取日志器
- `GetTracer() *TracerHelper` - 获取追踪器
- `Shutdown(ctx context.Context) error` - 关闭 SDK
- `ForceFlush(ctx context.Context) error` - 强制刷新数据

### Logger 方法

- `Debug(ctx, format, args...)` - 调试日志
- `Info(ctx, format, args...)` - 信息日志
- `Warn(ctx, format, args...)` - 警告日志
- `Error(ctx, format, args...)` - 错误日志
- `Fatal(ctx, format, args...)` - 致命错误日志
- `WithError(ctx, err, format, args...) error` - 记录错误并返回
- `SetMinLevel(level LogLevel)` - 设置最小日志级别

无上下文版本:

- `DebugWithoutContext(format, args...)`
- `InfoWithoutContext(format, args...)`
- `WarnWithoutContext(format, args...)`
- `ErrorWithoutContext(format, args...)`

### TracerHelper 方法

- `Start(ctx, spanName, opts...) (context.Context, trace.Span)` - 开始 span
- `StartWithAttributes(ctx, spanName, attrs...) (context.Context, trace.Span)` - 开始带属性的 span
- `SetAttributes(span, attrs...)` - 设置属性
- `AddEvent(span, eventName, attrs...)` - 添加事件
- `RecordError(span, err, description)` - 记录错误
- `TraceFunction(ctx, name, fn) error` - 追踪函数
- `TraceHTTPRequest(ctx, method, url, fn) error` - 追踪 HTTP 请求
- `TraceDBOperation(ctx, dbSystem, operation, table, fn) error` - 追踪数据库操作

## 最佳实践

### 1. 始终使用 defer 关闭资源

```go
sdk, err := optelsdk.NewSDK(config)
if err != nil {
    panic(err)
}
defer sdk.Shutdown(context.Background())
```

### 2. 为 span 添加有意义的属性

```go
tracer.SetAttributes(span,
    "user.id", userID,
    "order.id", orderID,
    "payment.method", "credit_card",
)
```

### 3. 使用日志器替代所有控制台输出

```go
// ❌ 不推荐
fmt.Println("处理请求")
log.Printf("用户ID: %s", userID)

// ✅ 推荐
logger.InfoWithoutContext("处理请求")
logger.Info(ctx, "用户ID: %s", userID)
```

### 4. 合理设置采样率

```go
// 开发环境：采样所有 trace
SampleRate: 1.0

// 生产环境：根据流量调整
SampleRate: 0.1  // 采样 10%
```

### 5. 在生产环境使用 TLS

```go
// 注意：当前实现使用 insecure 连接
// 生产环境应该配置 TLS 证书
```

## 示例项目

完整的示例代码请查看 `example/main.go` 文件。

运行示例:

```bash
cd example
go run main.go
```

## 故障排查

### 连接 Collector 失败

1. 确认 Collector 正在运行: `docker ps | grep otel-collector`
2. 检查端口是否正确: 默认 gRPC 端口为 4317
3. 查看 Collector 日志: `docker logs otel-collector`

### 看不到 trace 数据

1. 检查采样率设置
2. 确认调用了 `span.End()`
3. 在程序退出前调用 `sdk.Shutdown()` 或 `sdk.ForceFlush()`

### 日志没有 trace 信息

确保在有 span 的上下文中使用日志:

```go
ctx, span := tracer.Start(ctx, "operation")
defer span.End()

// ✅ 会包含 trace_id
logger.Info(ctx, "message")

// ❌ 不会包含 trace_id
logger.InfoWithoutContext("message")
```

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request！
