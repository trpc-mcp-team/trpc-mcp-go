package logging

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/mcptest"
)

// MockLogger 是一个用于测试的 logger 实现。
type MockLogger struct {
	buf bytes.Buffer
	mu  sync.Mutex // 防止并发写入
}

func (m *MockLogger) Log(ctx context.Context, level Level, msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 记录级别和消息
	m.buf.WriteString(fmt.Sprintf("[%s] %s ", level, msg))

	// 记录字段
	for _, f := range fields {
		m.buf.WriteString(fmt.Sprintf("%v ", f))
	}
	m.buf.WriteString("\n")
}

func (m *MockLogger) String() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.String()
}

func (m *MockLogger) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buf.Reset()
}

func (m *MockLogger) Contains(sub string) bool {
	return strings.Contains(m.String(), sub)
}

func TestLoggingMiddleware_WithOptions(t *testing.T) {
	// 准备通用的 mock 请求和 handler
	mockReq := &mcp.JSONRPCRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
	}

	mockFinalHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
		return &mcp.JSONRPCResponse{Result: "ok"}, nil
	}

	mockErrorHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
		return nil, errors.New("something went wrong")
	}

	// === 子测试开始 ===

	t.Run("ShouldLog/Default_OnlyLogsOnError", func(t *testing.T) {
		mockLogger := &MockLogger{}
		middleware := NewLoggingMiddleware(mockLogger) // 使用默认选项
		// Case 1: 成功调用，不应该有日志
		mcptest.RunMiddlewareTest(t, middleware, mockReq, mockFinalHandler)
		if logOutput := mockLogger.String(); logOutput != "" {
			t.Errorf("默认配置下，成功的请求不应产生日志，但收到了: %s", logOutput)
		}

		mockLogger.Reset()
		fmt.Println(mockLogger)

		// Case 2: 失败调用，应该有日志
		mcptest.RunMiddlewareTest(t, middleware, mockReq, mockErrorHandler)
		if !mockLogger.Contains("error something went wrong") {
			t.Errorf("默认配置下，失败的请求应该产生错误日志，但没有找到相关内容。日志为: %s", mockLogger.String())
		}
		fmt.Println("----------------------------------------------------------")
		fmt.Println(mockLogger)
	})

	t.Run("ShouldLog/Custom_LogAllRequests", func(t *testing.T) {
		mockLogger := &MockLogger{}
		// 自定义选项：记录所有请求
		middleware := NewLoggingMiddleware(mockLogger,
			WithShouldLog(func(level Level, duration time.Duration, err error) bool {
				return true
			}),
		)
		// 成功的请求也应该有日志
		fmt.Println("function!")
		mcptest.RunMiddlewareTest(t, middleware, mockReq, mockFinalHandler)
		if !mockLogger.Contains("method tools/call") {
			t.Errorf("配置为记录所有请求时，成功的请求也应该产生日志，但没有。日志为: %s", mockLogger.String())
		}
	})

	t.Run("PayloadLogging/Enabled", func(t *testing.T) {
		mockLogger := &MockLogger{}
		middleware := NewLoggingMiddleware(mockLogger,
			WithShouldLog(func(level Level, duration time.Duration, err error) bool { return true }), // 确保会记录日志
			WithPayloadLogging(true),
		)

		// 准备一个带参数的请求
		reqWithParams := &mcp.JSONRPCRequest{
			Request: mcp.Request{
				Method: "tools/call",
			},
			Params: map[string]interface{}{"user": "alice"},
		}

		mcptest.RunMiddlewareTest(t, middleware, reqWithParams, mockFinalHandler)

		logOutput := mockLogger.String()
		// 检查日志中是否包含了请求和响应的内容
		fmt.Println(logOutput)
		if !strings.Contains(logOutput, "trpc.request.content") || !strings.Contains(logOutput, "user:alice") {
			t.Errorf("启用 PayloadLogging 后，日志应包含请求内容，但没有找到。日志为: %s", logOutput)
		}
		if !strings.Contains(logOutput, "trpc.response.content") || !strings.Contains(logOutput, "ok") {
			t.Errorf("启用 PayloadLogging 后，日志应包含响应内容，但没有找到。日志为: %s", logOutput)
		}
	})

	t.Run("PayloadLogging/Disabled", func(t *testing.T) {
		mockLogger := &MockLogger{}
		middleware := NewLoggingMiddleware(mockLogger,
			WithShouldLog(func(level Level, duration time.Duration, err error) bool { return true }),
			WithPayloadLogging(false), // 显式禁用 (或使用默认)
		)
		fmt.Println("function!")
		reqWithParams := &mcp.JSONRPCRequest{
			Request: mcp.Request{
				Method: "tools/call",
			},
			Params: map[string]interface{}{"user": "alice"},
		}

		mcptest.RunMiddlewareTest(t, middleware, reqWithParams, mockFinalHandler)

		logOutput := mockLogger.String()
		if strings.Contains(logOutput, "grpc.request.content") || strings.Contains(logOutput, "user:alice") {
			t.Errorf("禁用 PayloadLogging 后，日志不应包含请求内容，但却找到了。日志为: %s", logOutput)
		}
	})

	t.Run("FieldsFromContext", func(t *testing.T) {
		mockLogger := &MockLogger{}
		middleware := NewLoggingMiddleware(mockLogger,
			WithShouldLog(func(level Level, duration time.Duration, err error) bool { return true }),
			WithFieldsFromContext(func(ctx context.Context) Fields {
				if requestID, ok := ctx.Value("request_id").(string); ok {
					return Fields{"request_id", requestID}
					//测试中间件的自定义函数截取ctx内容的功能
				}
				return nil
			}),
		)

		// 准备一个带有自定义值的 context
		ctxWithField := context.WithValue(context.Background(), "request_id", "xyz-123")

		finalHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
			// 可以在这里检查 context 是否被正确传递
			if ctx.Value("request_id") != "xyz-123" {
				t.Error("context 没有被正确传递到 final handler")
			}
			return &mcp.JSONRPCResponse{Result: "ok"}, nil
		}

		// 手动执行中间件
		middleware(ctxWithField, mockReq, nil, finalHandler)

		if !mockLogger.Contains("request_id xyz-123") {
			t.Errorf("预期日志中包含从 context 提取的字段 'request_id'，但没有找到。日志为: %s", mockLogger.String())
		}
	})
}
