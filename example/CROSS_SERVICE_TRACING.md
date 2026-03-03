# 跨服务追踪使用指南

## 🎯 核心概念

在微服务架构中，一个用户请求可能会经过多个服务。使用 OpenTelemetry 的 **Context Propagation（上下文传播）** 机制，可以让所有服务共享同一个 **Trace ID**，从而在 Jaeger UI 中看到完整的调用链。

## 📊 工作原理

```
服务A (TraceID: abc123)
  ↓ InjectContext() → HTTP Headers
  ↓ [traceparent: 00-abc123-span1-01]
服务B (提取 TraceID: abc123)
  ↓ InjectContext() → HTTP Headers  
  ↓ [traceparent: 00-abc123-span2-01]
服务C (提取 TraceID: abc123)

结果：三个服务共享同一个 TraceID: abc123
```

## 🔑 关键方法

### 1. `InjectContext()` - 注入追踪信息

**作用：** 将当前的 Trace Context 注入到传输载体（HTTP Headers、gRPC Metadata 等）

**使用场景：** 在调用下游服务之前

```go
// 注意：需要使用实现了 TextMapCarrier 接口的类型
// 示例中提供了 MapCarrier 类型

// 创建 MapCarrier（实现了 TextMapCarrier 接口）
type MapCarrier map[string]string

func (m MapCarrier) Get(key string) string { return m[key] }
func (m MapCarrier) Set(key, value string) { m[key] = value }
func (m MapCarrier) Keys() []string {
    keys := make([]string, 0, len(m))
    for k := range m { keys = append(keys, k) }
    return keys
}

// 使用 MapCarrier
headers := make(MapCarrier)

// 注入 Trace Context
tracer.InjectContext(ctx, headers)

// headers 现在包含：
// {
//   "traceparent": "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
// }
```

### 2. `ExtractContext()` - 提取追踪信息

**作用：** 从传输载体中提取 Trace Context

**使用场景：** 在接收上游服务请求时

```go
// 从 HTTP Headers 提取
ctx = tracer.ExtractContext(context.Background(), headers)

// 现在 ctx 包含了上游服务的 Trace 信息
ctx, span := tracer.Start(ctx, "处理请求")
// 这个 span 会自动成为上游 span 的子 span
```

## 💻 实际应用场景

### 场景 1：HTTP 服务间调用

#### 服务A（调用方）

```go
package main

import (
    "net/http"
    "github.com/scwl/optelsdk"
)

func callServiceB(ctx context.Context, tracer *optelsdk.TracerHelper) error {
    // 创建 Span
    ctx, span := tracer.Start(ctx, "调用服务B")
    defer span.End()
  
    // 创建 HTTP 请求
    req, err := http.NewRequest("GET", "http://service-b:8080/api/order", nil)
    if err != nil {
        return err
    }
  
    // 关键步骤：注入 Trace Context 到 HTTP Headers
    tracer.InjectContext(ctx, req.Header)
  
    // 发送请求
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
  
    return nil
}
```

#### 服务B（接收方）

```go
package main

import (
    "net/http"
    "github.com/scwl/optelsdk"
)

func handleOrder(w http.ResponseWriter, r *http.Request) {
    // 关键步骤：从 HTTP Headers 提取 Trace Context
    ctx := tracer.ExtractContext(r.Context(), r.Header)
  
    // 创建 Span（自动成为服务A的子 Span）
    ctx, span := tracer.Start(ctx, "处理订单")
    defer span.End()
  
    // 业务逻辑
    logger.Info(ctx, "处理订单请求")
  
    // 如果需要调用服务C，继续传递 Trace Context
    callServiceC(ctx)
  
    w.WriteHeader(http.StatusOK)
}

func main() {
    http.HandleFunc("/api/order", handleOrder)
    http.ListenAndServe(":8080", nil)
}
```

### 场景 2：gRPC 服务间调用

#### 客户端

```go
import (
    "google.golang.org/grpc/metadata"
)

func callGRPCService(ctx context.Context, tracer *optelsdk.TracerHelper) error {
    ctx, span := tracer.Start(ctx, "调用gRPC服务")
    defer span.End()
  
    // 创建 metadata
    md := metadata.New(nil)
  
    // 注入 Trace Context
    tracer.InjectContext(ctx, md)
  
    // 将 metadata 添加到 context
    ctx = metadata.NewOutgoingContext(ctx, md)
  
    // 发起 gRPC 调用
    resp, err := client.GetUser(ctx, &pb.GetUserRequest{ID: "123"})
  
    return err
}
```

#### 服务端

```go
import (
    "google.golang.org/grpc/metadata"
)

func (s *server) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    // 从 incoming metadata 提取 Trace Context
    md, ok := metadata.FromIncomingContext(ctx)
    if ok {
        ctx = tracer.ExtractContext(ctx, md)
    }
  
    // 创建 Span
    ctx, span := tracer.Start(ctx, "GetUser")
    defer span.End()
  
    // 业务逻辑
    logger.Info(ctx, "查询用户: %s", req.ID)
  
    return &pb.User{ID: req.ID, Name: "张三"}, nil
}
```

### 场景 3：消息队列（Kafka/RabbitMQ）

#### 生产者

```go
func publishMessage(ctx context.Context, tracer *optelsdk.TracerHelper) error {
    ctx, span := tracer.Start(ctx, "发布消息")
    defer span.End()
  
    // 创建消息头
    headers := make(map[string]string)
  
    // 注入 Trace Context
    tracer.InjectContext(ctx, headers)
  
    // 发送消息（将 headers 作为消息的元数据）
    msg := &kafka.Message{
        Topic: "orders",
        Value: []byte("order data"),
        Headers: convertToKafkaHeaders(headers),
    }
  
    return producer.WriteMessages(ctx, msg)
}

func convertToKafkaHeaders(headers map[string]string) []kafka.Header {
    var kafkaHeaders []kafka.Header
    for k, v := range headers {
        kafkaHeaders = append(kafkaHeaders, kafka.Header{
            Key:   k,
            Value: []byte(v),
        })
    }
    return kafkaHeaders
}
```

#### 消费者

```go
func consumeMessage(msg *kafka.Message, tracer *optelsdk.TracerHelper) error {
    // 从消息头提取 Trace Context
    headers := convertFromKafkaHeaders(msg.Headers)
    ctx := tracer.ExtractContext(context.Background(), headers)
  
    // 创建 Span
    ctx, span := tracer.Start(ctx, "处理消息")
    defer span.End()
  
    // 业务逻辑
    logger.Info(ctx, "处理订单消息")
  
    return nil
}

func convertFromKafkaHeaders(kafkaHeaders []kafka.Header) map[string]string {
    headers := make(map[string]string)
    for _, h := range kafkaHeaders {
        headers[h.Key] = string(h.Value)
    }
    return headers
}
```

## 🔍 在 Jaeger UI 中查看

运行示例代码后，访问 Jaeger UI：`http://localhost:16686`

你会看到类似这样的调用链：

```
Trace: 4bf92f3577b34da6a3ce929d0e0e4736 (总耗时: 75ms)
│
├─ 服务A-处理用户请求 (75ms)
│  │
│  ├─ 服务B-查询订单信息 (55ms)
│  │  │
│  │  └─ 服务C-查询库存 (25ms)
│  │
│  └─ 日志事件：
│     • 【服务A】收到用户请求
│     • 【服务A】生成 TraceID
│     • 【服务A】调用服务B
│     • 【服务A】收到服务B响应
```

## 📝 HTTP Headers 格式

OpenTelemetry 使用 W3C Trace Context 标准，注入的 Header 格式：

```
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
             │  │                                │                  │
             │  └─ Trace ID (32位十六进制)        └─ Span ID (16位)  └─ Flags
             └─ Version
```

**示例：**

```
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
```

- **Trace ID**: `4bf92f3577b34da6a3ce929d0e0e4736` - 全局唯一，标识整个请求链路
- **Span ID**: `00f067aa0ba902b7` - 当前操作的 ID
- **Flags**: `01` - 采样标志（01 表示采样）

## 🎯 最佳实践

### 1. 始终传递 Context

```go
// ✅ 正确：传递 context
func processOrder(ctx context.Context) error {
    ctx, span := tracer.Start(ctx, "processOrder")
    defer span.End()
  
    // 调用下游服务时传递 ctx
    return callPaymentService(ctx)
}

// ❌ 错误：不传递 context
func processOrder() error {
    ctx := context.Background()  // 新的 context，丢失了追踪信息
    ctx, span := tracer.Start(ctx, "processOrder")
    defer span.End()
  
    return callPaymentService(ctx)  // 追踪链断裂
}
```

### 2. HTTP 中间件自动注入/提取

```go
// HTTP 客户端中间件
func TracingMiddleware(next http.RoundTripper, tracer *optelsdk.TracerHelper) http.RoundTripper {
    return &tracingTransport{
        next:   next,
        tracer: tracer,
    }
}

type tracingTransport struct {
    next   http.RoundTripper
    tracer *optelsdk.TracerHelper
}

func (t *tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
    // 自动注入 Trace Context
    t.tracer.InjectContext(req.Context(), req.Header)
    return t.next.RoundTrip(req)
}

// HTTP 服务端中间件
func TracingHandler(next http.Handler, tracer *optelsdk.TracerHelper) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 自动提取 Trace Context
        ctx := tracer.ExtractContext(r.Context(), r.Header)
      
        ctx, span := tracer.Start(ctx, r.URL.Path)
        defer span.End()
      
        // 使用新的 context
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 3. 设置服务标识

```go
// 在每个服务中设置不同的服务名
sdk, err := optelsdk.NewSDK(&optelsdk.Config{
    ServiceName:       "order-service",  // 服务A
    // ServiceName:    "payment-service", // 服务B
    // ServiceName:    "inventory-service", // 服务C
    CollectorEndpoint: "localhost:4317",
})
```

### 4. 添加服务特定的属性

```go
ctx, span := tracer.Start(ctx, "处理订单")
defer span.End()

// 添加服务和业务相关的属性
tracer.SetAttributes(span,
    "service.name", "order-service",
    "service.version", "1.2.3",
    "order.id", orderID,
    "customer.id", customerID,
)
```

## 🚨 常见问题

### Q1: Trace ID 不一致怎么办？

**检查清单：**

1. 确保调用方使用了 `InjectContext()`
2. 确保接收方使用了 `ExtractContext()`
3. 检查 HTTP Headers 是否正确传递
4. 查看日志中的 `traceparent` header

```go
// 调试：打印 headers
headers := make(map[string]string)
tracer.InjectContext(ctx, headers)
fmt.Printf("Headers: %+v\n", headers)
```

### Q2: 如何在没有上游 Trace 的情况下开始追踪？

```go
// 创建新的根 Trace
ctx := context.Background()
ctx, span := tracer.Start(ctx, "新的请求")
defer span.End()

// 这会生成一个新的 Trace ID
```

### Q3: 如何手动指定 Trace ID？

OpenTelemetry 会自动生成 Trace ID，通常不需要手动指定。如果确实需要：

```go
import (
    "go.opentelemetry.io/otel/trace"
)

// 生成自定义 Trace ID（不推荐）
traceID, _ := trace.TraceIDFromHex("4bf92f3577b34da6a3ce929d0e0e4736")
spanID, _ := trace.SpanIDFromHex("00f067aa0ba902b7")

spanContext := trace.NewSpanContext(trace.SpanContextConfig{
    TraceID: traceID,
    SpanID:  spanID,
    TraceFlags: trace.FlagsSampled,
})

ctx = trace.ContextWithSpanContext(context.Background(), spanContext)
```

### Q4: gRPC metadata 如何实现 TextMapCarrier？

```go
import "google.golang.org/grpc/metadata"

// metadata.MD 已经实现了 TextMapCarrier 接口
md := metadata.New(nil)
tracer.InjectContext(ctx, md)  // 直接使用
```

## 📚 相关资源

- [W3C Trace Context 规范](https://www.w3.org/TR/trace-context/)
- [OpenTelemetry Context Propagation](https://opentelemetry.io/docs/instrumentation/go/manual/#propagators-and-context)
- [Jaeger 文档](https://www.jaegertracing.io/docs/)

## 🎓 运行示例

```bash
cd example
go run main.go
```

查看输出（如果启用了控制台）或在 Jaeger UI 中查看完整的调用链。
