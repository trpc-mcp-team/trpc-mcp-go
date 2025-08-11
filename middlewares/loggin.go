package middlewares

import (
	"context"
	"log"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

func LogginMiddleware(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session, next mcp.HandleFunc) (mcp.JSONRPCMessage, error) {
	log.Printf("-->进入请求：%s", req.Method)
	resp, err := next(ctx, req, session)
	log.Printf("-->退出请求：%s, 错误：%v", req.Method, err)
	return resp, err
}
