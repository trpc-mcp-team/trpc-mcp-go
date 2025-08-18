package middlewares

import (
	"context"
	"log"
	"runtime/debug"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

type RecoveryConfig struct {
	// Logger 日志记录器，如果为nil则使用默认logger
	Logger mcp.Logger
	// EnableStack 是否记录堆栈信息
	EnableStack bool
	// StackSkip 跳过的堆栈帧数
	StackSkip int
	// MaxStackSize 最大堆栈大小（字节）
	MaxStackSize int
	// EnableMetrics 是否启用恢复指标统计（预留）
	EnableMetrics bool
	// EnableAlert 是否启用告警（预留）
	EnableAlert bool
	// CustomErrorResponse 自定义错误响应生成器
	CustomErrorResponse func(ctx context.Context, req *mcp.JSONRPCRequest, panicErr interface{}) mcp.JSONRPCMessage
	// PanicFilter 过滤特定类型的panic（返回true表示需要记录和处理）
	PanicFilter func(panicErr interface{}) bool
}

// DefaultRecoveryConfig 返回默认的 RecoveryConfig 配置
func DefaultRecoveryConfig() *RecoveryConfig {
	return &RecoveryConfig{
		Logger:              nil, // 将使用默认logger
		EnableStack:         true,
		StackSkip:           3,
		MaxStackSize:        8192, // 8KB
		EnableMetrics:       false,
		EnableAlert:         false,
		CustomErrorResponse: nil,
		PanicFilter:         nil, // nil表示处理所有panic
	}
}

// RecoveryMiddleware 捕获处理过程中的 panic 并返回标准 JSON-RPC 错误响应
func RecoveryMiddleware(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session, next mcp.HandleFunc) (errResp mcp.JSONRPCMessage, err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Recovery] panic caught: %v\nStack:\n%s", r, debug.Stack())

			errResp = mcp.NewJSONRPCErrorResponse(
				req.ID,
				mcp.ErrCodeInternal,
				"internal server error",
				map[string]interface{}{
					"panic": r,
				},
			)
			err = nil
		}
	}()

	resp, err := next(ctx, req, session)
	return resp, err
}
