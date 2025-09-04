package logging

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m" // ERROR
	ColorYellow = "\033[33m" // WARN
	ColorGreen  = "\033[32m" // INFO
	ColorBlue   = "\033[34m" // DEBUG
	ColorCyan   = "\033[36m" // 时间戳
	ColorWhite  = "\033[37m" // 消息
	ColorGray   = "\033[90m" // 字段
)

// useColor 检查是否应该使用颜色输出
var useColor = shouldUseColor()

func shouldUseColor() bool {
	// 检查各种颜色环境变量
	if os.Getenv("CLICOLOR") == "0" {
		return false
	}

	if os.Getenv("CLICOLOR_FORCE") == "1" || os.Getenv("FORCE_COLOR") == "1" {
		return true
	}

	if os.Getenv("CLICOLOR") == "1" || os.Getenv("COLOR") == "1" || os.Getenv("COLOR") == "true" {
		return isTerminal()
	}

	// 检测 COLORTERM
	if colorterm := os.Getenv("COLORTERM"); colorterm != "" {
		return isTerminal()
	}

	return false
}

// isTerminal 检查是否在终端中运行
func isTerminal() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

type Logger interface {
	Log(ctx context.Context, level Level, msg string, fields ...any)
}

type LoggerFunc func(ctx context.Context, level Level, msg string, fields ...any)

func (f LoggerFunc) Log(ctx context.Context, level Level, msg string, fields ...any) {
	f(ctx, level, msg, fields...)
}

// provide flexible implements for interface

type Fields []interface{}

// formatFields 格式化字段为更易读的字符串
func formatFields(levelColor string, fields ...any) string {
	if len(fields) == 0 {
		return ""
	}

	var result strings.Builder
	result.WriteString("\n")

	for i := 0; i < len(fields); i += 2 {
		if i+1 >= len(fields) {
			break
		}

		key := fmt.Sprintf("%v", fields[i])
		value := fields[i+1]

		if useColor {
			result.WriteString(fmt.Sprintf("  %s%s: %s", ColorGray, key, ColorReset))
		} else {
			result.WriteString(fmt.Sprintf("  %s: ", key))
		}

		switch v := value.(type) {
		case map[string]interface{}:
			result.WriteString("{\n")
			for k, val := range v {
				if useColor {
					result.WriteString(fmt.Sprintf("    %s%s: %s%v%s\n", ColorGray, k, levelColor, val, ColorReset))
				} else {
					result.WriteString(fmt.Sprintf("    %s: %v\n", k, val))
				}
			}
			result.WriteString("  }")
		default:
			if useColor {
				result.WriteString(fmt.Sprintf("%s%v%s", levelColor, v, ColorReset))
			} else {
				result.WriteString(fmt.Sprintf("%v", v))
			}
		}

		result.WriteString("\n")
	}

	return result.String()
}

// getLevelColor 根据日志级别返回对应的颜色
func getLevelColor(level Level) string {
	if !useColor {
		return ""
	}
	switch level {
	case LevelError:
		return ColorRed
	case LevelWarn:
		return ColorYellow
	case LevelInfo:
		return ColorGreen
	case LevelDebug:
		return ColorBlue
	default:
		return ColorWhite
	}
}

// logWithFormat 使用格式化输出日志
func logWithFormat(logger Logger, ctx context.Context, level Level, msg string, fields ...any) {
	levelColor := getLevelColor(level)
	var formattedMsg string
	if useColor {
		formattedMsg = fmt.Sprintf(" %s%s%s", levelColor, msg, ColorReset)
	} else {
		formattedMsg = fmt.Sprintf(" %s", msg)
	}
	formattedMsg += formatFields(levelColor, fields...)
	logger.Log(ctx, level, formattedMsg)
}

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
			"event", "request_started",
			"system", "mcp",
			"span.kind", "server",
			"method", req.Method,
			"start_time", start.Format(time.RFC3339),
		}

		// Add session_id if session is not nil
		if session != nil {
			startFields = append(startFields, "session_id", session.GetID())
		}

		if o.fieldsFromCtx != nil {
			startFields = append(startFields, o.fieldsFromCtx(ctx)...)
		}

		if o.logPayload {
			startFields = append(startFields, "request", map[string]interface{}{
				"params": req.Params,
			})
		}
		if o.shouldLog(LevelInfo, 0, nil) {
			logWithFormat(logger, ctx, LevelInfo, "Request started", startFields...)
		}
		resp, err := next(ctx, req, session)
		duration := time.Since(start)

		// 检查是否是错误响应（处理工具执行错误被转换为响应的情况）
		var hasError bool
		var errorMessage string
		var errorType string

		if err != nil {
			// 传统错误
			hasError = true
			errorMessage = err.Error()
			errorType = fmt.Sprintf("%T", err)
		} else if resp != nil {
			// 检查是否是错误响应
			if errorResp, ok := resp.(*mcp.JSONRPCError); ok {
				hasError = true
				errorMessage = errorResp.Error.Message
				errorType = "JSONRPCErrorResponse"
				if errorResp.Error.Data != nil {
					errorMessage += fmt.Sprintf(" (Data: %v)", errorResp.Error.Data)
				}
			}
		}

		// 决定是否记录日志
		shouldLogError := hasError && o.shouldLog(LevelError, duration, err)
		shouldLogSuccess := !hasError && o.shouldLog(LevelInfo, duration, err)

		if !shouldLogError && !shouldLogSuccess {
			return resp, err
		}

		// stage2 : request reuslt log
		resultFields := []any{
			"event", "request_completed",
			"method", req.Method,
			"duration_ms", duration.Milliseconds(),
		}

		if o.logPayload && resp != nil {
			if jsonResp, ok := resp.(*mcp.JSONRPCResponse); ok {
				resultFields = append(resultFields, "response", map[string]interface{}{
					"result": jsonResp.Result,
				})
			} else if errorResp, ok := resp.(*mcp.JSONRPCError); ok {
				resultFields = append(resultFields, "response", map[string]interface{}{
					"error": errorResp.Error,
				})
			}
		}

		if hasError {
			// stage3: error log
			errorFields := append(resultFields,
				"event", "request_failed",
				"error", map[string]interface{}{
					"message": errorMessage,
					"type":    errorType,
				},
			)
			logWithFormat(logger, ctx, LevelError, "Request failed", errorFields...)
		} else {
			// stage4: finish log
			logWithFormat(logger, ctx, LevelInfo, "Request completed", resultFields...)
		}

		return resp, err
	}

}
