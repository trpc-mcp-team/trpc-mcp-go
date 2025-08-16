package middlewares

import (
	"context"
	"log/slog"
	"os"
	"testing"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/mcptest"
)

type SlogAdapter struct {
	logger *slog.Logger
}

func (s *SlogAdapter) Log(fields ...interface{}) {
	// 将 fields 转换为 slog 的字段格式
	s.logger.Info("MCP Request Handled", fields...)
}

func NewSlogAdapter() *SlogAdapter {
	return &SlogAdapter{
		logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
	}
}

func TestLogginMiddleware_Conformance(t *testing.T) {
	mylogger := NewSlogAdapter()
	mcptest.CheckMiddlewareFunc(t, newLoggingMiddleware(mylogger))
}
func TestLogginMiddleware(t *testing.T) {
	//准备模拟请求，更多模拟请求样例可看jsonrpc_test.gp
	mockReq := &mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "request-1",
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: map[string]interface{}{
			"name":      "test-tool",
			"arguments": map[string]interface{}{"param1": "value1"},
		},
	}
	myLogger := NewSlogAdapter() // 使用 SlogAdapter 作为日志记录器
	//模拟最终业务逻辑
	finalHandlerCalled := false
	mockFinalHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
		t.Log("模拟的最终业务逻辑被调用")
		finalHandlerCalled = true
		return &mcp.JSONRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  "final handler result",
		}, nil
	}

	//调用测试辅助函数
	t.Log("开始执行中间件测试...")
	_, err := mcptest.RunMiddlewareTest(
		t,
		newLoggingMiddleware(myLogger), // 假设 LogginMiddleware 定义在同一个包中
		mockReq,
		mockFinalHandler,
	)
	t.Log("中间件测试执行完毕。")

	// 1. 检查中间件本身是否返回了不该有的错误
	if err != nil {
		t.Errorf("预期中间件本身不应返回错误, 但收到了: %v", err)
	}

	// 2. 检查最终的业务逻辑（next函数）是否真的被调用了
	//    这是验证“洋葱模型”是否被正确遵守的关键。
	if !finalHandlerCalled {
		t.Errorf("预期最终的 handler 应该被调用, 但它没有！这通常意味着中间件没有正确调用 next()。")
	}
}
