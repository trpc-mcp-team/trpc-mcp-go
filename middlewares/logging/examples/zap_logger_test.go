package examples

import (
	"context"
	"errors"
	"testing"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/internal/log"
	mcptest "trpc.group/trpc-go/trpc-mcp-go/mcptest"
	"trpc.group/trpc-go/trpc-mcp-go/middlewares/logging"
)

// TestZapAdapterWithMiddleware 测试 ZapAdapter 与 logging 中间件的集成
func TestZapAdapterWithMiddleware(t *testing.T) {
	// 创建真实的 ZapLogger
	zapLogger := log.NewZapLogger()

	// 创建适配器
	adapter := NewZapAdapter(zapLogger)

	// 准备测试请求
	mockReq := &mcp.JSONRPCRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: map[string]interface{}{"user": "alice"},
	}

	// 成功的 handler
	successHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
		return &mcp.JSONRPCResponse{Result: "ok"}, nil
	}

	// 失败的 handler
	errorHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
		return nil, errors.New("test error")
	}

	t.Run("Default behavior - only logs errors", func(t *testing.T) {
		// 使用默认配置（只记录错误）
		middleware := logging.NewLoggingMiddleware(adapter)

		// 测试成功的请求 - 不应该有日志
		mcptest.RunMiddlewareTest(t, middleware, mockReq, successHandler)
		//fmt.Printf("zapLogger: %v\n", zapLogger)

		// 测试失败的请求 - 应该有错误日志
		mcptest.RunMiddlewareTest(t, middleware, mockReq, errorHandler)
		// fmt.Printf("zapLogger_with_error: %v\n", zapLogger)
	})

	t.Run("Custom behavior - log all requests", func(t *testing.T) {
		// 配置记录所有请求
		middleware := logging.NewLoggingMiddleware(adapter,
			logging.WithShouldLog(func(level logging.Level, duration time.Duration, err error) bool {
				return true
			}),
			logging.WithPayloadLogging(true),
		)

		// 测试成功的请求 - 应该有日志
		mcptest.RunMiddlewareTest(t, middleware, mockReq, successHandler)

		// 测试失败的请求 - 应该有日志
		mcptest.RunMiddlewareTest(t, middleware, mockReq, errorHandler)
	})

	t.Run("With context fields", func(t *testing.T) {
		// 配置从 context 提取字段
		middleware := logging.NewLoggingMiddleware(adapter,
			logging.WithShouldLog(func(level logging.Level, duration time.Duration, err error) bool {
				return true
			}),
			logging.WithFieldsFromContext(func(ctx context.Context) logging.Fields {
				if requestID, ok := ctx.Value("request_id").(string); ok {
					return logging.Fields{"request_id", requestID}
				}
				return nil
			}),
		)

		// 创建带有 request_id 的 context
		ctxWithRequestID := context.WithValue(context.Background(), "request_id", "test-123")

		handler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
			return &mcp.JSONRPCResponse{Result: "success"}, nil
		}

		middleware(ctxWithRequestID, mockReq, nil, handler)
	})
}

// TestZapAdapterErrorHandling 测试错误处理
func TestZapAdapterErrorHandling(t *testing.T) {
	zapLogger := log.NewZapLogger()
	adapter := NewZapAdapter(zapLogger)
	middleware := logging.NewLoggingMiddleware(adapter)

	mockReq := &mcp.JSONRPCRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
	}

	// 测试各种错误类型
	testErrors := []struct {
		name string
		err  error
	}{
		{"Standard error", errors.New("standard error")},
		{"Nil error", nil},
		{"Custom error", &customError{message: "custom error"}},
	}

	for _, tt := range testErrors {
		t.Run(tt.name, func(t *testing.T) {
			handler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
				return nil, tt.err
			}

			// 确保中间件能够处理各种错误类型而不 panic
			mcptest.RunMiddlewareTest(t, middleware, mockReq, handler)
		})
	}
}

// customError 自定义错误类型用于测试
type customError struct {
	message string
}

func (e *customError) Error() string {
	return e.message
}
