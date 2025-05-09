# Streamable MCP: Model Context Protocol Implementation with Streaming Support

A Go implementation of the [Model Context Protocol (MCP)](https://github.com/model-context-protocol/model-context-protocol) with comprehensive streaming support via Server-Sent Events (SSE). This library enables efficient communication between client applications and tools/resources.

## Features

### Core Features

- **Full MCP Specification Support**: Implements the MCP 2024-11-05 specification
- **Streaming Support**: Real-time data streaming with Server-Sent Events (SSE)
- **Tool Framework**: Register and execute tools with structured parameter handling
- **Resource Management**: Serve text and binary resources with RESTful interfaces
- **Prompt Templates**: Create and manage prompt templates for LLM interactions
- **Progress Notifications**: Built-in support for progress updates on long-running operations
- **Logging System**: Integrated logging with configurable levels

### Transport Options

- **Server-Sent Events (SSE)**: Efficient one-way streaming from server to client
- **JSON-RPC**: Standard request-response communication

### Connection Modes

- **Stateless**: Simple request-response pattern without persistent sessions
- **Stateful**: Persistent connections with session management

## Installation

```bash
go get github.com/modelcontextprotocol/streamable-mcp
```

## Quick Start

### Server Example

```go
package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/streamable-mcp/log"
	"github.com/modelcontextprotocol/streamable-mcp/schema"
	"github.com/modelcontextprotocol/streamable-mcp/server"
)

func main() {
	// Configure logging
	log.SetLevel(log.InfoLevel)
	log.Info("Starting example server...")

	// Create server
	mcpServer := server.NewServer(":3000", schema.Implementation{
		Name:    "Example-Server",
		Version: "1.0.0",
	}, server.WithPathPrefix("/mcp"))

	// Register a tool (defined elsewhere)
	if err := mcpServer.RegisterTool(tools.NewGreetTool()); err != nil {
		log.Fatalf("Failed to register tool: %v", err)
	}
	
	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	
	// Start server in background
	go func() {
		log.Info("Server started on port 3000")
		if err := mcpServer.Start(); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	}()
	
	// Wait for termination signal
	<-stop
	log.Info("Shutting down server...")
}
```

### Tool Definition Example

```go
package tools

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/streamable-mcp/schema"
)

// NewGreetTool creates a simple greeting tool
func NewGreetTool() *schema.Tool {
	return schema.NewTool("greet",
		func(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
			// Check for context cancellation
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				// Continue execution
			}
			
			// Extract name parameter
			name := "World"
			if nameArg, ok := req.Params.Arguments["name"]; ok {
				if nameStr, ok := nameArg.(string); ok && nameStr != "" {
					name = nameStr
				}
			}
			
			// Create greeting message
			greeting := fmt.Sprintf("Hello, %s!", name)
			
			// Return result
			return schema.NewTextResult(greeting), nil
		},
		schema.WithDescription("A simple greeting tool"),
		schema.WithString("name", 
			schema.Description("Name to greet"),
		),
	)
}
```

### Client Example

```go
package main

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/streamable-mcp/client"
	"github.com/modelcontextprotocol/streamable-mcp/log"
	"github.com/modelcontextprotocol/streamable-mcp/schema"
)

func main() {
	// Initialize logging
	log.Info("Starting client...")
	
	// Create context
	ctx := context.Background()
	
	// Create client
	mcpClient, err := client.NewClient("http://localhost:3000/mcp", schema.Implementation{
		Name:    "MCP-Go-Client",
		Version: "1.0.0",
	}, client.WithProtocolVersion(schema.ProtocolVersion_2024_11_05))
	if err != nil {
		log.Errorf("Failed to create client: %v", err)
		return
	}
	defer mcpClient.Close()
	
	// Initialize client
	initResp, err := mcpClient.Initialize(ctx)
	if err != nil {
		log.Errorf("Initialization failed: %v", err)
		return
	}
	log.Infof("Server: %s %s, Protocol: %s", 
		initResp.ServerInfo.Name, 
		initResp.ServerInfo.Version,
		initResp.ProtocolVersion)
	
	// Get session ID
	sessionID := mcpClient.GetSessionID()
	if sessionID != "" {
		log.Infof("Session ID: %s", sessionID)
	}
	
	// List available tools
	tools, err := mcpClient.ListTools(ctx)
	if err != nil {
		log.Errorf("Failed to list tools: %v", err)
		return
	}
	
	// Call a tool if available
	if len(tools) > 0 {
		log.Infof("Calling tool: %s", tools[0].Name)
		content, err := mcpClient.CallTool(ctx, tools[0].Name, map[string]interface{}{
			"name": "MCP User",
		})
		if err != nil {
			log.Errorf("Tool call failed: %v", err)
			return
		}
		
		// Process results
		for _, item := range content {
			if textContent, ok := item.(schema.TextContent); ok {
				log.Infof("Result: %s", textContent.Text)
			}
		}
	}
	
	// Terminate session if active
	if sessionID != "" {
		if err := mcpClient.TerminateSession(ctx); err != nil {
			log.Errorf("Failed to terminate session: %v", err)
		}
	}
}
```

## Configuration

### Server Configuration

The server can be configured using option functions:

```go
server := server.NewServer(
    ":3000",                        // Listen address
    schema.Implementation{          // Server info
        Name:    "My-MCP-Server",
        Version: "1.0.0",
    },
    server.WithPathPrefix("/mcp"),            // API path prefix
    server.WithSSEEnabled(true),              // Enable SSE
    server.WithGetSSEEnabled(true),           // Allow GET for SSE
    server.WithDefaultResponseMode("sse"),    // Default to SSE mode
    server.WithStatelessMode(false),          // Use stateful mode
)
```

### Available Server Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithPathPrefix` | Set API path prefix | `/mcp` |
| `WithSessionManager` | Use custom session manager | Built-in manager |
| `WithoutSession` | Disable session management | Sessions enabled |
| `WithSSEEnabled` | Enable SSE responses | `true` |
| `WithGetSSEEnabled` | Allow GET for SSE connections | `true` |
| `WithDefaultResponseMode` | Default mode: "json" or "sse" | `"sse"` |
| `WithNotificationBufferSize` | Size of notification buffer | `10` |
| `WithStatelessMode` | Run in stateless mode | `false` |

### Client Configuration

The client can be configured using option functions:

```go
client, err := client.NewClient(
    "http://localhost:3000/mcp",             // Server URL
    schema.Implementation{                   // Client info
        Name:    "MCP-Client",
        Version: "1.0.0",
    },
    client.WithProtocolVersion(schema.ProtocolVersion_2024_11_05),  // Protocol version
    client.WithGetSSEEnabled(true),                                // Use GET for SSE
)
```

### Available Client Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithProtocolVersion` | Specify MCP protocol version | Latest version |
| `WithGetSSEEnabled` | Use GET for SSE instead of POST | `false` |

## Advanced Features

### Streaming Progress with SSE

Create tools that provide real-time progress updates:

```go
func NewStreamingTool() *schema.Tool {
    return schema.NewTool("sse-progress",
        func(ctx context.Context, req *schema.CallToolRequest) (*schema.CallToolResult, error) {
            // Extract parameters
            steps := 5 // Default
            if stepsArg, ok := req.Params.Arguments["steps"]; ok {
                if stepsFloat, ok := stepsArg.(float64); ok {
                    steps = int(stepsFloat)
                }
            }
            
            // Get notification sender from context
            notifier, ok := schema.GetNotificationSender(ctx)
            if !ok {
                return schema.NewTextResult("No notification sender available"), nil
            }
            
            // Send initial progress
            notifier.SendProgress(0.0, "Starting process")
            notifier.SendLogMessage("info", "Starting process")
            
            // Process steps and send progress updates
            for i := 1; i <= steps; i++ {
                select {
                case <-ctx.Done():
                    return nil, ctx.Err()
                default:
                    // Calculate progress
                    progress := float64(i) / float64(steps)
                    message := fmt.Sprintf("Step %d/%d", i, steps)
                    
                    // Send notifications
                    notifier.SendProgress(progress, message)
                    notifier.SendLogMessage("info", fmt.Sprintf("Completed %s", message))
                    
                    // Simulate work
                    time.Sleep(500 * time.Millisecond)
                }
            }
            
            // Final progress update
            notifier.SendProgress(1.0, "Complete")
            
            return schema.NewTextResult("Process completed successfully"), nil
        },
        schema.WithDescription("Tool with progress notifications"),
        schema.WithNumber("steps",
            schema.Description("Number of steps to process"),
            schema.Default(5),
        ),
    )
}
```

### Client-Side Progress Handling

```go
// Create notification collector
collector := &NotificationCollector{
    progressHandler: func(progress float64, message string) {
        fmt.Printf("Progress: %.0f%% - %s\n", progress*100, message)
    },
    logHandler: func(level string, message string) {
        fmt.Printf("[%s] %s\n", level, message)
    },
}

// Set up stream options
streamOpts := &transport.StreamOptions{
    NotificationHandlers: map[string]transport.NotificationHandler{
        "notifications/progress": collector.HandleProgressNotification,
        "notifications/message": collector.HandleLogNotification,
    },
}

// Call tool with streaming
content, err := client.CallToolWithStream(ctx, "sse-progress", map[string]interface{}{
    "steps": 5,
}, streamOpts)
```

### Resource Management

Register and serve resources:

```go
// Register a text resource
textResource := &schema.Resource{
    URI:         "resource://example/text",
    Name:        "example-text",
    Description: "Example text resource",
    MimeType:    "text/plain",
}
server.RegisterResource(textResource)

// Resource handler is automatically set up through the HTTP handler
```

### Prompt Templates

Register prompt templates:

```go
// Register a prompt template
promptTemplate := &schema.Prompt{
    Name:        "example-prompt",
    Description: "Example prompt template",
    Arguments: []schema.PromptArgument{
        {
            Name:        "topic",
            Description: "Topic to discuss",
            Required:    true,
        },
    },
}
server.RegisterPrompt(promptTemplate)
```

## Example Patterns

The project includes several example patterns:

| Pattern | Description |
|---------|-------------|
| `basic` | Simple tool registration and usage |
| `resource_prompt_example` | Resource and prompt template examples |
| `stateful_json` | Stateful connections with JSON-RPC |
| `stateful_sse` | Stateful connections with SSE |
| `stateful_json_getsse` | Stateful JSON with GET SSE support |
| `stateful_sse_yes_getsse` | Stateful SSE with GET SSE support |
| `stateless_json` | Stateless connections with JSON-RPC |
| `stateless_sse_no_getsse` | Stateless connections with SSE |

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request. 