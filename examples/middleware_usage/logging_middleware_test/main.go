package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/examples/middlewares/logging"
)

// MockLogger 是一个用于测试的 logger 实现（来自 new_logging_test.go）
type MockLogger struct {
	buf bytes.Buffer
	mu  sync.Mutex // 防止并发写入
}

func (m *MockLogger) Log(ctx context.Context, level logging.Level, msg string, fields ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")

	// 记录级别和消息
	m.buf.WriteString(fmt.Sprintf("[%s] [%s] %s ", timestamp, level, msg))

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

func authMiddleware(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session, next mcp.HandleFunc) (mcp.JSONRPCMessage, error) {
	log.Printf("[Auth Middleware] Checking authentication for: %s", req.Method)

	// 模拟认证检查
	if req.Method == "tools/call" {
		log.Printf("[Auth Middleware] Tool call requires authentication")
	}

	resp, err := next(ctx, req, session)
	log.Printf("[Auth Middleware] Authentication check completed for: %s", req.Method)
	return resp, err
}

// 测试工具
func TestTool(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("TestTool called with: %v", req.Params.Arguments)

	message := fmt.Sprintf("TestTool executed at: %s", time.Now().Format(time.RFC3339))
	result := mcp.NewTextResult(message)
	return result, nil
}

// 错误测试工具
func ErrorTool(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("ErrorTool called with: %v", req.Params.Arguments)
	return nil, errors.New("something went wrong")
}

func main() {
	// 创建 MockLogger
	mockLogger := &MockLogger{}

	// 创建功能完整的 logging middleware
	advancedLoggingMiddleware := logging.NewLoggingMiddleware(
		mockLogger,
		logging.WithShouldLog(func(level logging.Level, duration time.Duration, err error) bool {
			// 记录所有级别的日志用于测试
			return true
		}),
		logging.WithPayloadLogging(true),
		logging.WithFieldsFromContext(func(ctx context.Context) logging.Fields {
			return logging.Fields{
				"test.source", "integration-test",
				"test.timestamp", time.Now().Unix(),
			}
		}),
	)

	// 创建服务器
	s := mcp.NewServer(
		"middleware-test-server",
		"1.0.0",
		mcp.WithStatelessMode(true),
	)

	s.Use(authMiddleware)            // 第2层：认证
	s.Use(advancedLoggingMiddleware) // 第3层：高级日志

	// 注册测试工具
	testToolDef := &mcp.Tool{
		Name:        "test_tool",
		Description: "A simple test tool",
		InputSchema: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: map[string]*openapi3.SchemaRef{
				"message": {
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "Test message",
					},
				},
			},
		},
	}

	errorToolDef := &mcp.Tool{
		Name:        "error_tool",
		Description: "A tool that returns an error",
		InputSchema: &openapi3.Schema{
			Type: &openapi3.Types{"object"},
			Properties: map[string]*openapi3.SchemaRef{
				"message": {
					Value: &openapi3.Schema{
						Type:        &openapi3.Types{"string"},
						Description: "Error message",
					},
				},
			},
		},
	}

	s.RegisterTool(testToolDef, TestTool)
	s.RegisterTool(errorToolDef, ErrorTool)

	// 启动服务器
	fmt.Println("=== Middleware Integration Test Server with MockLogger ===")
	fmt.Println("Server listening on :8080")
	fmt.Println("")
	fmt.Println("Test scenarios:")
	fmt.Println("1. All requests will pass through 3 middleware layers")
	fmt.Println("2. Simple middleware logs entry/exit")
	fmt.Println("3. Auth middleware checks authentication")
	fmt.Println("4. Advanced middleware logs detailed info using MockLogger")
	fmt.Println("5. Test both successful and error scenarios")
	fmt.Println("")
	fmt.Println("MockLogger features:")
	fmt.Println("- Thread-safe logging with mutex protection")
	fmt.Println("- Buffer-based log storage")
	fmt.Println("- Contains() method for log verification")
	fmt.Println("- String() method for complete log output")
	fmt.Println("- Reset() method for clearing logs")
	fmt.Println("")

	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				logOutput := mockLogger.String()
				if logOutput != "" {
					fmt.Printf("\n=== MockLogger Output ===\n%s\n", logOutput)
					// 可选：清空缓冲区
					mockLogger.Reset()
				}
			}
		}
	}()

	fmt.Println("Server listening on :8080")
	if err := http.ListenAndServe(":8080", s.Handler()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
