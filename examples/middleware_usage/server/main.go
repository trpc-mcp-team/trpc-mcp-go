package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/getkin/kin-openapi/openapi3"
	mcp "trpc.group/trpc-go/trpc-mcp-go"
)

// loggingMiddleware is a simple middleware that logs the entry and exit of a request.
func loggingMiddleware(ctx context.Context, req *mcp.JSONRPCRequest, session mcp.Session, next mcp.HandleFunc) (mcp.JSONRPCMessage, error) {
	log.Printf("[Logging Middleware] --> Enter request method: %s", req.Method)
	resp, err := next(ctx, req, session)
	log.Printf("[Logging Middleware] <-- Exit request method: %s, err: %v", req.Method, err)
	return resp, err
}

// GreeterTool is a simple tool handler that returns a greeting.
func GreeterTool(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	log.Printf("Tool '%s' called with arguments: %v", req.Params.Name, req.Params.Arguments)

	name, ok := req.Params.Arguments["name"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid 'name' argument")
	}

	greeting := fmt.Sprintf("Hello, %s!", name)
	result := mcp.NewTextResult(greeting)
	return result, nil
}

func main() {
	// Define the Greeter tool.
	greeterToolDef := &mcp.Tool{
		Name:        "greeter",
		Description: "A simple tool that returns a greeting.",
		InputSchema: &openapi3.Schema{
			// FINAL FIX: The 'Type' field requires a pointer to a slice of strings (*openapi3.Types).
			Type: &openapi3.Types{"object"},
			Properties: map[string]*openapi3.SchemaRef{
				"name": {
					Value: &openapi3.Schema{
						// FINAL FIX: This also requires the same pointer-to-slice-of-strings type.
						Type:        &openapi3.Types{"string"},
						Description: "The name to greet.",
					},
				},
			},
			Required: []string{"name"},
		},
	}

	// Create a new server.
	s := mcp.NewServer(
		"mcp-server",
		"1.0.0",
		mcp.WithStatelessMode(true),
	)

	// Use the logging middleware.
	s.Use(loggingMiddleware)

	// Register the Greeter tool.
	s.RegisterTool(greeterToolDef, GreeterTool)

	// Start the server.
	log.Println("Server is listening on :8080")
	if err := http.ListenAndServe(":8080", s.Handler()); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
