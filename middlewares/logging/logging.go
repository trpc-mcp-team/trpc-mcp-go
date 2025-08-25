package logging

import (
	"context"
	"fmt"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

type Logger interface {
	Log(ctx context.Context, level Level, msg string, fields ...any)
}

type LoggerFunc func(ctx context.Context, level Level, msg string, fields ...any)

func (f LoggerFunc) Log(ctx context.Context, level Level, msg string, fields ...any) {
	f(ctx, level, msg, fields...)
}

// provide flexible implements for interface

type Fields []interface{}

// options structure preserve all configurable options
type options struct {
	shouldLog func(level Level, duration time.Duration, err error) bool

	logPayload bool

	fieldsFromCtx func(ctx context.Context) Fields
}

// Option is a func to change options struct
type Option func(*options)

type Level int

const (
	LevelDebug Level = -4
	LevelInfo  Level = 0
	LevelWarn  Level = 4
	LevelError Level = 8
	LevelFatal Level = 12
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	case LevelFatal:
		return "FATAL"
	default:
		return "UNKNOWN"

	}
}

// Enabled returns true if the level is enabled for logging.
// This allows for level-based filtering similar to slog.
func (l Level) Enabled(level Level) bool {
	return l >= level
}

// WithShouldLog 设置一个自定义的日志记录条件。
func WithShouldLog(f func(level Level, duration time.Duration, err error) bool) Option {
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
var defaultShouldLog = func(level Level, duration time.Duration, err error) bool {
	return level >= LevelError
}

func NewLoggingMiddleware(logger Logger, opts ...Option) mcp.MiddlewareFunc {
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
		//stage1 start log
		startFields := Fields{
			"system", "mcp",
			"span.kind", "server",
			"method", req.Method,
			"start_time", start.Format(time.RFC3339),
		}

		if o.fieldsFromCtx != nil {
			startFields = append(startFields, o.fieldsFromCtx(ctx)...)
		}

		if o.logPayload {
			startFields = append(startFields, "trpc.request.content", req.Params)
		}
		if o.shouldLog(LevelInfo, 0, nil) {
			logger.Log(ctx, LevelInfo, "Request started", startFields...)
		}
		resp, err := next(ctx, req, session)
		duration := time.Since(start)

		if !o.shouldLog(LevelError, duration, err) {
			return resp, err
		}

		resultFields := []any{
			"method", req.Method,
			"duration_ms", duration.Milliseconds(),
		}
		// stage2 : request reuslt log

		if o.logPayload && resp != nil {
			if jsonResp, ok := resp.(*mcp.JSONRPCResponse); ok {
				resultFields = append(resultFields, "trpc.response.content", jsonResp.Result)
			}
		}

		if err != nil {
			// stage3: error log
			errorFields := append(resultFields,
				"error", err.Error(),
				"error_type", fmt.Sprintf("%T", err),
			)
			logger.Log(ctx, LevelError, "Request failed", errorFields...)
		} else if o.shouldLog(LevelInfo, duration, err) {
			// stage4: finish log
			logger.Log(ctx, LevelInfo, "Request completed", resultFields...)
		}

		return resp, err
	}

}
