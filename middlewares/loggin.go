package middlewares

import (
	"context"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

type Logger interface {
	Log(fileds ...interface{})
}

type Fields []interface{}

// options 结构体保存了所有可配置项。
type options struct {
	shouldLog func(duration time.Duration, err error) bool

	logPayload bool

	fieldsFromCtx func(ctx context.Context) Fields
}

// Option 是一个用于修改 options 结构体的函数类型。
type Option func(*options)

// WithShouldLog 设置一个自定义的日志记录条件。
func WithShouldLog(f func(duration time.Duration, err error) bool) Option {
	return func(o *options) {
		o.shouldLog = f
	}
}

// WithPayloadLogging 启用或禁用对请求/响应体的日志记录。
func WithPayloadLogging(enabled bool) Option {
	return func(o *options) {
		o.logPayload = enabled
	}
}

// WithFieldsFromContext 设置一个从 context 中提取字段的函数。
func WithFieldsFromContext(f func(ctx context.Context) Fields) Option {
	return func(o *options) {
		o.fieldsFromCtx = f
	}
}

// 默认只记录出现错误的请求
var defaultShouldLog = func(duration time.Duration, err error) bool {
	return err != nil
}

func newLoggingMiddleware(logger Logger, opts ...Option) mcp.MiddlewareFunc {
	// 初始化默认配置
	o := &options{
		shouldLog:  defaultShouldLog,
		logPayload: false,
	}
	// 2. 应用所有用户传入的配置选项
	for _, opt := range opts {
		opt(o)
	}

	return func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session, next mcp.HandleFunc) (mcp.JSONRPCMessage, error) {
		start := time.Now()
		fields := Fields{
			"system", "mcp",
			"span.kind", "server",
			"method", req.Method,
			//"request_id", req.ID,
			//"session_id", session.GetID(),
			"start_time", start.Format(time.RFC3339),
		}

		if o.fieldsFromCtx != nil {
			fields = append(fields, o.fieldsFromCtx(ctx)...)
		}

		if o.logPayload {
			fields = append(fields, "trpc.request.content", req.Params)
		}

		resp, err := next(ctx, req, session)
		duration := time.Since(start)

		if !o.shouldLog(duration, err) {
			return resp, err
		}

		fields = append(fields, "duration", duration.Milliseconds(),
			"error", err,
		)

		if o.logPayload && resp != nil {
			if jsonResp, ok := resp.(*mcp.JSONRPCResponse); ok {
				fields = append(fields, "trpc.response.content", jsonResp.Result)
			}
		}

		logger.Log(fields...)

		return resp, err
	}

}
