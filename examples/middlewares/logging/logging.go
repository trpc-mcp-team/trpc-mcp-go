// Package logging provides an example of a logging middleware.
// This is not part of the core library, but serves as a guide for users to implement their own.
package logging

import (
	"context"
	"log"
	"time"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// Middleware logs the beginning and end of each request, along with the time taken.
func Middleware(next mcp.HandleFunc) mcp.HandleFunc {
	return func(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session) (mcp.JSONRPCMessage, error) {
		startTime := time.Now()
		log.Printf("Request started: method[%s], id[%s]", req.Method, req.ID)

		resp, err := next(ctx, req, session)

		log.Printf(
			"Request finished: method[%s], id[%s], duration[%s], error[%v]",
			req.Method,
			req.ID,
			time.Since(startTime),
			err,
		)
		return resp, err
	}
}
