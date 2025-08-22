package middlewares

import (
	"context"
	"errors"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"strings"
	"testing"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/mcptest"
)

func TestMetricsMiddleware_Conformance(t *testing.T) {
	mw := NewMetricsMiddleware()
	mcptest.CheckMiddlewareFunc(t, mw)
}

func TestMetricsMiddleware(t *testing.T) {
	mockReq := &mcp.JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      "1",
		Request: mcp.Request{Method: "tools/call"},
	}

	// 成功请求
	t.Run("success", func(t *testing.T) {
		rec := NewInMemoryMetricsRecorder()
		mw := NewMetricsMiddleware(WithRecorder(rec))

		var inFlightDuringNext int
		mockFinalHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, s mcp.Session) (mcp.JSONRPCMessage, error) {
			rec.mu.Lock()
			inFlightDuringNext = rec.InFlight[req.Method]
			rec.mu.Unlock()
			return &mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: "ok"}, nil
		}

		_, err := mcptest.RunMiddlewareTest(t, mw, mockReq, mockFinalHandler)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if inFlightDuringNext != 1 {
			t.Fatalf("expected in-flight == 1 during next, got %d", inFlightDuringNext)
		}

		rec.mu.Lock()
		defer rec.mu.Unlock()
		if rec.InFlight[mockReq.Method] != 0 {
			t.Fatalf("expected in-flight == 0 after request, got %d", rec.InFlight[mockReq.Method])
		}
		if rec.Requests[mockReq.Method] != 1 {
			t.Fatalf("expected requests == 1, got %d", rec.Requests[mockReq.Method])
		}
		if len(rec.LatencyMs[mockReq.Method]) != 1 || len(rec.LatencySuccess[mockReq.Method]) != 1 || !rec.LatencySuccess[mockReq.Method][0] {
			t.Fatal("expected one latency with success=true")
		}
	})

	// 系统错误
	t.Run("ErrorFromNext", func(t *testing.T) {
		rec := NewInMemoryMetricsRecorder()
		mw := NewMetricsMiddleware(WithRecorder(rec))

		mockFinalHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
			return nil, errors.New("boom")
		}

		_, _ = mcptest.RunMiddlewareTest(t, mw, mockReq, mockFinalHandler)

		rec.mu.Lock()
		defer rec.mu.Unlock()
		if rec.Errors[mockReq.Method][mcp.ErrCodeInternal] != 1 {
			t.Fatalf("expected errors[internal] == 1, got %d", rec.Errors[mockReq.Method][mcp.ErrCodeInternal])
		}
		if len(rec.LatencySuccess[mockReq.Method]) != 1 || rec.LatencySuccess[mockReq.Method][0] {
			t.Fatalf("expected latency success=false when next returns error")
		}
	})

	// 运行时错误
	t.Run("JSONRPCErrorResponse", func(t *testing.T) {
		rec := NewInMemoryMetricsRecorder()
		mw := NewMetricsMiddleware(WithRecorder(rec))

		mockFinalHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
			e := &mcp.JSONRPCError{JSONRPC: "2.0", ID: req.ID}
			e.Error.Code = mcp.ErrCodeMethodNotFound
			e.Error.Message = "not found"
			return e, nil
		}

		_, _ = mcptest.RunMiddlewareTest(t, mw, mockReq, mockFinalHandler)

		rec.mu.Lock()
		defer rec.mu.Unlock()
		if rec.Errors[mockReq.Method][mcp.ErrCodeMethodNotFound] != 1 {
			t.Fatalf("expected errors[method_not_found] == 1, got %d", rec.Errors[mockReq.Method][mcp.ErrCodeMethodNotFound])
		}
		if len(rec.LatencySuccess[mockReq.Method]) != 1 || rec.LatencySuccess[mockReq.Method][0] {
			t.Fatalf("expected latency success=false when JSONRPCError returned")
		}
	})

	// 过滤
	t.Run("Filter", func(t *testing.T) {
		rec := NewInMemoryMetricsRecorder()
		mw := NewMetricsMiddleware(WithRecorder(rec), WithFilter(func(method string) bool { return method == "tools/call" }))

		mockReq2 := &mcp.JSONRPCRequest{
			JSONRPC: "2.0",
			ID:      "2",
			Request: mcp.Request{Method: "resources/list"},
		}

		mockFinalHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
			return &mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: "ok"}, nil
		}

		_, _ = mcptest.RunMiddlewareTest(t, mw, mockReq2, mockFinalHandler)

		rec.mu.Lock()
		defer rec.mu.Unlock()
		if rec.Requests[mockReq.Method] != 0 || len(rec.LatencyMs[mockReq.Method]) != 0 || rec.InFlight[mockReq.Method] != 0 {
			t.Fatalf("expected no metrics for filtered-out method; got requests=%d latency=%d inflight=%d",
				rec.Requests[mockReq.Method], len(rec.LatencyMs[mockReq.Method]), rec.InFlight[mockReq.Method])
		}
	})

	t.Run("PrometheusRecorder", func(t *testing.T) {
		rec, err := NewPrometheusMetricsRecorder()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		mw := NewMetricsMiddleware(WithRecorder(rec))

		// 成功
		_, err = mcptest.RunMiddlewareTest(t, mw, mockReq, func(ctx context.Context, req *mcp.JSONRPCRequest, s mcp.Session) (mcp.JSONRPCMessage, error) {
			return &mcp.JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: "ok"}, nil
		})
		if err != nil {
			t.Fatal(err)
		}

		// 业务错误
		_, _ = mcptest.RunMiddlewareTest(t, mw, mockReq, func(ctx context.Context, req *mcp.JSONRPCRequest, s mcp.Session) (mcp.JSONRPCMessage, error) {
			e := &mcp.JSONRPCError{JSONRPC: "2.0", ID: req.ID}
			e.Error.Code = mcp.ErrCodeMethodNotFound
			e.Error.Message = "nf"
			return e, nil
		})

		// 运行期错误
		_, _ = mcptest.RunMiddlewareTest(t, mw, mockReq, func(ctx context.Context, req *mcp.JSONRPCRequest, s mcp.Session) (mcp.JSONRPCMessage, error) {
			return nil, errors.New("boom")
		})

		// requests_total 至少有一条
		gotRequests := testutil.CollectAndCount(rec.RequestsCollector())
		if gotRequests == 0 {
			t.Fatalf("expected requests metrics, got 0")
		}

		// errors_total 精确比对
		expected := `
# HELP mcp_server_errors_total Total MCP errors
# TYPE mcp_server_errors_total counter
mcp_server_errors_total{code="-32601",method="tools/call"} 1
mcp_server_errors_total{code="-32603",method="tools/call"} 1
`
		if err := testutil.CollectAndCompare(
			rec.ErrorsCollector(),
			strings.NewReader(expected),
			"trpcmcp_server_errors_total",
		); err != nil {
			t.Fatalf("mismatch: %v", err)
		}
	})

}
