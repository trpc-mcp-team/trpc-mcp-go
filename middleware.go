// Tencent is pleased to support the open source community by making trpc-mcp-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-mcp-go is licensed under the Apache License Version 2.0.

package mcp

import "context"

// HandleFunc defines the function signature for a request handler, which is the
// final destination in a middleware chain.
type HandleFunc func(ctx context.Context, req *JSONRPCRequest, session Session) (JSONRPCMessage, error)

// MiddlewareFunc defines the function signature for a middleware. It processes a
// request and passes control to the next handler in the chain.
type MiddlewareFunc func(ctx context.Context, req *JSONRPCRequest, session Session, next HandleFunc) (JSONRPCMessage, error)
