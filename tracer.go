package optelsdk

import (
	"context"
	"fmt"
	"runtime"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// TracerHelper 追踪辅助器，提供便捷的追踪功能
// 用于创建和管理分布式追踪span
type TracerHelper struct {
	tracer trace.Tracer
}

// NewTracerHelper 创建一个新的追踪辅助器实例
//
// 参数:
//   - serviceName: 服务名称，用于标识追踪器
//
// 返回:
//   - *TracerHelper: 追踪辅助器实例
func NewTracerHelper(serviceName string) *TracerHelper {
	return &TracerHelper{
		tracer: otel.Tracer(serviceName),
	}
}

// Start 开始一个新的span
// 这是最基础的追踪方法，用于标记一个操作的开始
//
// 参数:
//   - ctx: 上下文对象，用于传递追踪信息
//   - spanName: span的名称，描述这个操作
//   - opts: 可选的span配置选项
//
// 返回:
//   - context.Context: 包含新span的上下文
//   - trace.Span: 创建的span对象，使用完毕后需要调用End()
//
// 示例:
//
//	ctx, span := tracer.Start(ctx, "处理用户请求")
//	defer span.End()
//	// 执行业务逻辑
func (t *TracerHelper) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, spanName, opts...)
}

// StartWithAttributes 开始一个带有属性的新span
// 属性用于为span添加额外的元数据信息
//
// 参数:
//   - ctx: 上下文对象
//   - spanName: span的名称
//   - attrs: 键值对属性，用于描述span的详细信息
//
// 返回:
//   - context.Context: 包含新span的上下文
//   - trace.Span: 创建的span对象
//
// 示例:
//
//	ctx, span := tracer.StartWithAttributes(ctx, "查询数据库",
//	    "db.system", "mysql",
//	    "db.name", "users",
//	    "db.operation", "SELECT",
//	)
//	defer span.End()
func (t *TracerHelper) StartWithAttributes(ctx context.Context, spanName string, attrs ...interface{}) (context.Context, trace.Span) {
	attributes := t.convertToAttributes(attrs...)
	return t.tracer.Start(ctx, spanName, trace.WithAttributes(attributes...))
}

// RecordError 记录span中的错误
// 当操作失败时，使用此方法记录错误信息
//
// 参数:
//   - span: 要记录错误的span
//   - err: 错误对象
//   - description: 错误描述（可选）
//
// 示例:
//
//	if err != nil {
//	    tracer.RecordError(span, err, "数据库查询失败")
//	    span.End()
//	    return err
//	}
func (t *TracerHelper) RecordError(span trace.Span, err error, description string) {
	if err == nil {
		return
	}

	// 设置span状态为错误
	span.SetStatus(codes.Error, description)

	// 记录错误事件
	if description != "" {
		span.RecordError(err, trace.WithAttributes(
			attribute.String("error.description", description),
		))
	} else {
		span.RecordError(err)
	}
}

// AddEvent 向span添加事件
// 事件用于标记span生命周期中的重要时刻
//
// 参数:
//   - span: 要添加事件的span
//   - eventName: 事件名称
//   - attrs: 事件属性（可选）
//
// 示例:
//
//	tracer.AddEvent(span, "缓存命中", "cache.key", "user:123")
func (t *TracerHelper) AddEvent(span trace.Span, eventName string, attrs ...interface{}) {
	attributes := t.convertToAttributes(attrs...)
	span.AddEvent(eventName, trace.WithAttributes(attributes...))
}

// SetAttributes 为span设置属性
// 用于在span创建后添加或更新属性
//
// 参数:
//   - span: 要设置属性的span
//   - attrs: 键值对属性
//
// 示例:
//
//	tracer.SetAttributes(span,
//	    "user.id", "123",
//	    "user.role", "admin",
//	)
func (t *TracerHelper) SetAttributes(span trace.Span, attrs ...interface{}) {
	attributes := t.convertToAttributes(attrs...)
	span.SetAttributes(attributes...)
}

// TraceFunction 追踪一个函数的执行
// 这是一个便捷方法，自动创建span并处理错误
//
// 参数:
//   - ctx: 上下文对象
//   - functionName: 函数名称（如果为空，自动获取调用者函数名）
//   - fn: 要执行的函数
//
// 返回:
//   - error: 函数执行过程中的错误
//
// 示例:
//
//	err := tracer.TraceFunction(ctx, "处理订单", func(ctx context.Context, span trace.Span) error {
//	    // 业务逻辑
//	    tracer.SetAttributes(span, "order.id", "12345")
//	    return processOrder(ctx)
//	})
func (t *TracerHelper) TraceFunction(ctx context.Context, functionName string, fn func(context.Context, trace.Span) error) error {
	// 如果没有提供函数名，自动获取调用者的函数名
	if functionName == "" {
		pc, _, _, ok := runtime.Caller(1)
		if ok {
			functionName = runtime.FuncForPC(pc).Name()
		} else {
			functionName = "unknown"
		}
	}

	ctx, span := t.Start(ctx, functionName)
	defer span.End()

	err := fn(ctx, span)
	if err != nil {
		t.RecordError(span, err, "函数执行失败")
	}

	return err
}

// TraceHTTPRequest 追踪HTTP请求
// 专门用于HTTP请求的追踪，自动添加HTTP相关属性
//
// 参数:
//   - ctx: 上下文对象
//   - method: HTTP方法（GET, POST等）
//   - url: 请求URL
//   - fn: 要执行的函数
//
// 返回:
//   - error: 函数执行过程中的错误
//
// 示例:
//
//	err := tracer.TraceHTTPRequest(ctx, "GET", "/api/users", func(ctx context.Context, span trace.Span) error {
//	    // 执行HTTP请求
//	    tracer.SetAttributes(span, "http.status_code", 200)
//	    return nil
//	})
func (t *TracerHelper) TraceHTTPRequest(ctx context.Context, method, url string, fn func(context.Context, trace.Span) error) error {
	spanName := fmt.Sprintf("%s %s", method, url)
	ctx, span := t.StartWithAttributes(ctx, spanName,
		"http.method", method,
		"http.url", url,
	)
	defer span.End()

	err := fn(ctx, span)
	if err != nil {
		t.RecordError(span, err, "HTTP请求失败")
	}

	return err
}

// TraceDBOperation 追踪数据库操作
// 专门用于数据库操作的追踪，自动添加数据库相关属性
//
// 参数:
//   - ctx: 上下文对象
//   - dbSystem: 数据库系统（mysql, postgresql等）
//   - operation: 操作类型（SELECT, INSERT等）
//   - table: 表名
//   - fn: 要执行的函数
//
// 返回:
//   - error: 函数执行过程中的错误
//
// 示例:
//
//	err := tracer.TraceDBOperation(ctx, "mysql", "SELECT", "users", func(ctx context.Context, span trace.Span) error {
//	    // 执行数据库查询
//	    tracer.SetAttributes(span, "db.rows_affected", 10)
//	    return nil
//	})
func (t *TracerHelper) TraceDBOperation(ctx context.Context, dbSystem, operation, table string, fn func(context.Context, trace.Span) error) error {
	spanName := fmt.Sprintf("%s %s.%s", operation, dbSystem, table)
	ctx, span := t.StartWithAttributes(ctx, spanName,
		"db.system", dbSystem,
		"db.operation", operation,
		"db.table", table,
	)
	defer span.End()

	err := fn(ctx, span)
	if err != nil {
		t.RecordError(span, err, "数据库操作失败")
	}

	return err
}

// convertToAttributes 将键值对转换为OpenTelemetry属性
// 内部辅助方法，用于处理可变参数
//
// 参数:
//   - attrs: 键值对，格式为 key1, value1, key2, value2, ...
//
// 返回:
//   - []attribute.KeyValue: OpenTelemetry属性数组
func (t *TracerHelper) convertToAttributes(attrs ...interface{}) []attribute.KeyValue {
	if len(attrs)%2 != 0 {
		// 如果参数数量不是偶数，忽略最后一个参数
		attrs = attrs[:len(attrs)-1]
	}

	attributes := make([]attribute.KeyValue, 0, len(attrs)/2)
	for i := 0; i < len(attrs); i += 2 {
		key, ok := attrs[i].(string)
		if !ok {
			continue
		}

		// 根据值的类型创建相应的属性
		switch v := attrs[i+1].(type) {
		case string:
			attributes = append(attributes, attribute.String(key, v))
		case int:
			attributes = append(attributes, attribute.Int(key, v))
		case int64:
			attributes = append(attributes, attribute.Int64(key, v))
		case float64:
			attributes = append(attributes, attribute.Float64(key, v))
		case bool:
			attributes = append(attributes, attribute.Bool(key, v))
		default:
			// 对于其他类型，转换为字符串
			attributes = append(attributes, attribute.String(key, fmt.Sprintf("%v", v)))
		}
	}

	return attributes
}

// GetSpanFromContext 从上下文中获取当前的span
// 用于在不同函数间传递span
//
// 参数:
//   - ctx: 上下文对象
//
// 返回:
//   - trace.Span: 当前的span，如果不存在则返回无操作span
//
// 示例:
//
//	span := tracer.GetSpanFromContext(ctx)
//	tracer.SetAttributes(span, "custom.field", "value")
func (t *TracerHelper) GetSpanFromContext(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// InjectContext 将追踪上下文注入到carrier中
// 用于跨服务传播追踪信息（例如HTTP头）
//
// 参数:
//   - ctx: 包含追踪信息的上下文
//   - carrier: 用于携带追踪信息的载体（例如http.Header）
//
// 示例:
//
//	carrier := propagation.MapCarrier{}
//	tracer.InjectContext(ctx, carrier)
//	// 将carrier中的信息添加到HTTP请求头
func (t *TracerHelper) InjectContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	otel.GetTextMapPropagator().Inject(ctx, carrier)
}

// ExtractContext 从carrier中提取追踪上下文
// 用于接收跨服务传播的追踪信息
//
// 参数:
//   - ctx: 基础上下文
//   - carrier: 携带追踪信息的载体（例如http.Header）
//
// 返回:
//   - context.Context: 包含提取的追踪信息的新上下文
//
// 示例:
//
//	carrier := propagation.HeaderCarrier(r.Header)
//	ctx = tracer.ExtractContext(ctx, carrier)
//	// 现在ctx包含了上游服务的追踪信息
func (t *TracerHelper) ExtractContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}
