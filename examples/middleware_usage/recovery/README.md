# Recovery 错误恢复中间件

## Overview
The **Recovery** middleware is used to capture panics during request processing.  

It returns a standard JSON-RPC error response while also logging detailed error information and optional stack traces, improving the robustness and maintainability of services.


## Usage Examples

**Default Configuration**
```go
server.Use(Recovery())
```

**Custom Configuration**
```go
server.Use(RecoveryWithOptions(
    WithLogger(customLogger),
    WithStackTrace(true),
    WithMaxStackSize(4096),
    WithPanicFilter(OnlyHandleRuntimeErrors()),
    WithCustomErrorResponse(func(ctx context.Context, req *JSONRPCRequest, panicErr interface{}) JSONRPCMessage {
        return NewJSONRPCErrorResponse(
            req.ID,
            ErrCodeInternal,
            "Service temporarily unavailable",
            nil,
        )
    }),
))
```

## Configuration Details

### Options

* **Logger**: Logger instance. Defaults to `GetDefaultLogger()`.  
* **EnableStack**: Whether to log stack traces. Default is `true`.  
* **StackSkip**: Number of stack frames to skip. Default is `3`.  
* **MaxStackSize**: Maximum number of bytes to output for the stack trace. Default is `8192 (8KB)`.  
* **PanicFilter**: Panic filter function. If it returns `true`, the panic will be handled. By default, all panics are handled.  
* **CustomErrorResponse**: Custom error response generator. By default, a generic internal error response is returned.  

### Common Configuration Functions

* `WithLogger(logger Logger)`: Set a custom logger.  

* `WithStackTrace(enable bool)`: Enable or disable stack trace logging.  

* `WithStackSkip(skip int)`: Set the number of stack frames to skip.  

* `WithMaxStackSize(size int)`: Set the maximum output size of the stack trace.  

* `WithPanicFilter(filter func(interface{}) bool)`: Set a custom panic filter.  

* `WithCustomErrorResponse(handler func(context.Context, *JSONRPCRequest, interface{}) JSONRPCMessage)`: Set a custom error response generator.  

### Built-in Panic Filters

* `IgnoreStringPanics()`: Ignore panics of type `string`.  

* `OnlyHandleRuntimeErrors()`: Only handle runtime-related panics.  


## Design Notes
Automatically logs detailed information after capturing a panic (including timestamp, request ID, method name, error message, and stack trace).

Supports flexible configuration to meet different business requirements.

By default, internal error details are not exposed to clients, ensuring security.