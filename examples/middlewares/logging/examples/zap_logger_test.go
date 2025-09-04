package examples

import (
	"context"
	"errors"
	"testing"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/examples/middlewares/logging"
	"trpc.group/trpc-go/trpc-mcp-go/internal/log"
	mcptest "trpc.group/trpc-go/trpc-mcp-go/mcptest"
)

// TestZapAdapterWithMiddleware
func TestZapAdapterWithMiddleware(t *testing.T) {

	zapLogger := log.NewZapLogger()

	// create the Zap adapter
	adapter := NewZapAdapter(zapLogger)

	mockReq := &mcp.JSONRPCRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
		Params: map[string]interface{}{"user": "alice"},
	}

	successHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
		return &mcp.JSONRPCResponse{Result: "ok"}, nil
	}

	errorHandler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
		return nil, errors.New("test error")
	}

	t.Run("Default behavior - only logs errors", func(t *testing.T) {
		// default configuration: only log errors
		middleware := logging.NewLoggingMiddleware(adapter)

		// request without error - should not log
		mcptest.RunMiddlewareTest(t, middleware, mockReq, successHandler)
		// request with error - should log
		mcptest.RunMiddlewareTest(t, middleware, mockReq, errorHandler)

	})

	t.Run("Custom behavior - log all requests", func(t *testing.T) {
		// custom configuration: log all requests
		middleware := logging.NewLoggingMiddleware(adapter,
			logging.WithShouldLog(func(level logging.Level, duration time.Duration, err error) bool {
				return true
			}),
			logging.WithPayloadLogging(true),
		)

		mcptest.RunMiddlewareTest(t, middleware, mockReq, successHandler)

		mcptest.RunMiddlewareTest(t, middleware, mockReq, errorHandler)
		// should log with or without error
	})

	t.Run("With context fields", func(t *testing.T) {
		// get custom fields from context
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

		// create a context with a custom field
		ctxWithRequestID := context.WithValue(context.Background(), "request_id", "test-123")

		handler := func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
			return &mcp.JSONRPCResponse{Result: "success"}, nil
		}

		middleware(ctxWithRequestID, mockReq, nil, handler)
	})
}

// TestZapAdapterErrorHandling
func TestZapAdapterErrorHandling(t *testing.T) {
	zapLogger := log.NewZapLogger()
	adapter := NewZapAdapter(zapLogger)
	middleware := logging.NewLoggingMiddleware(adapter)

	mockReq := &mcp.JSONRPCRequest{
		Request: mcp.Request{
			Method: "tools/call",
		},
	}

	// test various error types
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

			// make sure the middleware handles the error without panicking
			mcptest.RunMiddlewareTest(t, middleware, mockReq, handler)
		})
	}
}

// customError
type customError struct {
	message string
}

func (e *customError) Error() string {
	return e.message
}
