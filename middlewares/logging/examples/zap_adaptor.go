package examples

import (
	"context"

	"trpc.group/trpc-go/trpc-mcp-go/internal/log"
	"trpc.group/trpc-go/trpc-mcp-go/middlewares/logging"
)

func NewZapAdapter(zapLogger *log.ZapLogger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, level logging.Level, msg string, fields ...any) {
		args := make([]interface{}, 0, len(fields)+2)
		args = append(args, msg)
		for i := 0; i < len(fields); i += 2 {
			if i+1 < len(fields) {
				args = append(args, fields[i], fields[i+1])

			} else {
				// This is to ensure that the number of arguments is even
				args = append(args, fields[i], "missing value")
			}
		}

		switch level {
		case logging.LevelDebug:
			zapLogger.Debug(args...)
		case logging.LevelInfo:
			zapLogger.Info(args...)
		case logging.LevelWarn:
			zapLogger.Warn(args...)
		case logging.LevelError:
			zapLogger.Error(args...)
		case logging.LevelFatal:
			zapLogger.Fatal(args...)
		default:
			// 未知级别默认使用 Info
			zapLogger.Info(args...)
		}
	})
}
