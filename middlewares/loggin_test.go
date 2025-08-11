package middlewares

import (
	"context"
	"testing"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/mcptest"
)

func TestLogginMiddleware_Conformance(t *testing.T) {
	mcptest.CheckMiddlewareFunc(t, LogginMiddleware)
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
		LogginMiddleware, // 假设 LogginMiddleware 定义在同一个包中
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
