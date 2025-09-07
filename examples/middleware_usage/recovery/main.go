// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	mcp "trpc.group/trpc-go/trpc-mcp-go"
	"trpc.group/trpc-go/trpc-mcp-go/examples/middlewares/recovery"
)

func main() {
	// Print startup message.
	log.Printf("Starting Recovery middleware example server...")

	// Create server with Recovery middleware.
	// - Recovery is used to catch panics in tool handlers
	// - It returns standard JSON-RPC error responses
	// - It also logs detailed error information with optional stack trace
	mcpServer := mcp.NewServer(
		"Recovery-Example-Server",
		"0.1.0",
		mcp.WithServerAddress(":3001"),
		mcp.WithServerPath("/mcp"),
		mcp.WithServerLogger(mcp.GetDefaultLogger()),
	)

	// Register a tool that may panic.
	unsafeTool := NewUnsafeTool()
	unsafeHandler := func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		// Simulate a panic when name == "panic"
		if nameArg, ok := req.Params.Arguments["name"]; ok {
			if nameStr, ok := nameArg.(string); ok && nameStr == "panic" {
				panic("simulated panic for testing Recovery middleware")
			}
		}

		// Otherwise return a normal result.
		return mcp.NewTextResult("Safe execution completed."), nil
	}

	// Apply Recovery middleware with custom options.
	mcpServer.Use(
		recovery.RecoveryWithOptions(
			recovery.WithLogger(mcp.GetDefaultLogger()),
			recovery.WithStackTrace(true),
			recovery.WithMaxStackSize(4096),
			recovery.WithCustomErrorResponse(func(ctx context.Context, req *mcp.JSONRPCRequest, panicErr interface{}) mcp.JSONRPCMessage {
				return mcp.NewJSONRPCErrorResponse(
					req.ID,
					mcp.ErrCodeInternal,
					"Service temporarily unavailable (panic handled by Recovery)",
					nil,
				)
			}),
		))

	// Register the unsafe tool with Recovery protection.
	mcpServer.RegisterTool(unsafeTool, unsafeHandler)
	log.Printf("Registered unsafe tool with Recovery middleware: unsafe-tool")

	// Set up graceful shutdown.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine.
	go func() {
		log.Printf("MCP server started, listening on port 3001, path /mcp")
		if err := mcpServer.Start(); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	// Wait for termination signal.
	<-stop
	log.Printf("Shutting down server...")
}

// NewUnsafeTool creates a tool that may trigger panic.
func NewUnsafeTool() *mcp.Tool {
	return mcp.NewTool("unsafe-tool",
		mcp.WithDescription("A demo tool that may panic to test the Recovery middleware."),
		mcp.WithString("name",
			mcp.Description("The name parameter; use 'panic' to trigger an error."),
		),
	)
}
