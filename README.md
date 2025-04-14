# JSON-RPC

[![GoDoc](https://godoc.org/github.com/viant/jsonrpc?status.svg)](https://godoc.org/github.com/viant/jsonrpc)
[![Go Report Card](https://goreportcard.com/badge/github.com/viant/jsonrpc)](https://goreportcard.com/report/github.com/viant/jsonrpc)
[![GoReportCard](https://goreportcard.com/badge/github.com/viant/jsonrpc)](https://goreportcard.com/report/github.com/viant/jsonrpc)

This package implements the [JSON-RPC 2.0](https://www.jsonrpc.org/specification) protocol in Go, providing a lightweight and efficient way to create JSON-RPC clients and servers.

## Features

* Full implementation of the JSON-RPC 2.0 specification
* Support for notifications, requests, and responses
* Customizable error handling
* Efficient JSON marshaling and unmarshaling
* Thread-safe implementation
* Comprehensive logging capabilities

## Installation

```bash
go get github.com/viant/jsonrpc
```

## Usage

This package provides multiple transport implementations for JSON-RPC 2.0 communication:

### Standard I/O (stdio) Transport

The stdio transport allows JSON-RPC communication with external processes through standard input/output.

#### Client Usage

The stdio client executes a command and communicates with it via stdin/stdout:

```go
package main

import (
    "context"
    "fmt"
    "github.com/viant/jsonrpc"
    "github.com/viant/jsonrpc/transport/client/stdio"
)

func main() {
    // Create a new stdio client that runs the "my_service" command
    client, err := stdio.New("my_service", 
        stdio.WithArguments("--config", "config.json"),
        stdio.WithEnvironment("DEBUG", "true"),
    )
    if err != nil {
        panic(err)
    }

    // Create a JSON-RPC request
    request := &jsonrpc.Request{
        Jsonrpc: "2.0",
        Method:  "add",
        Params:  []byte(`{"x": 10, "y": 20}`),
        ID:      1,
    }

    // Send the request and get the response
    ctx := context.Background()
    response, err := client.Send(ctx, request)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Result: %s\n", response.Result)

    // Send a notification (no response expected)
    notification := &jsonrpc.Notification{
        Jsonrpc: "2.0",
        Method:  "log",
        Params:  []byte(`{"message": "Hello, world!"}`),
    }

    err = client.Notify(ctx, notification)
    if err != nil {
        panic(err)
    }
}
```

#### Server Usage

The stdio server reads JSON-RPC messages from stdin and writes responses to stdout:

```go
package main

import (
    "context"
    "github.com/viant/jsonrpc"
    "github.com/viant/jsonrpc/transport"
    "github.com/viant/jsonrpc/transport/server/stdio"
    "os"
)

// Define a handler for JSON-RPC methods
type Handler struct{}

func (h *Handler) Handle(ctx context.Context, method string, params []byte) (interface{}, error) {
    switch method {
    case "add":
        // Parse parameters and perform addition
        var args struct {
            X int `json:"x"`
            Y int `json:"y"`
        }
        if err := jsonrpc.Unmarshal(params, &args); err != nil {
            return nil, err
        }
        return map[string]int{"result": args.X + args.Y}, nil
    default:
        return nil, jsonrpc.NewError(jsonrpc.MethodNotFoundError, "Method not found", nil)
    }
}

func main() {
    // Create a new handler factory
    newHandler := func(ctx context.Context) transport.Handler {
        return &Handler{}
    }

    // Create a new stdio server
    ctx := context.Background()
    server := stdio.New(ctx, newHandler,
        stdio.WithErrorWriter(os.Stderr),
    )

    // Start listening for JSON-RPC messages
    if err := server.ListenAndServe(); err != nil {
        panic(err)
    }
}
```

### HTTP Server-Sent Events (SSE) Transport

The HTTP SSE transport allows JSON-RPC communication over HTTP using Server-Sent Events for real-time updates.

#### Client Usage

The SSE client connects to an SSE endpoint and sends messages to the server:

```go
package main

import (
    "context"
    "fmt"
    "github.com/viant/jsonrpc"
    "github.com/viant/jsonrpc/transport/client/http/sse"
    "time"
)

func main() {
    // Create a new SSE client
    ctx := context.Background()
    client, err := sse.New(ctx, "http://localhost:8080/sse",
        sse.WithHandshakeTimeout(time.Second * 10),
    )
    if err != nil {
        panic(err)
    }

    // Create a JSON-RPC request
    request := &jsonrpc.Request{
        Jsonrpc: "2.0",
        Method:  "getData",
        Params:  []byte(`{"id": 123}`),
        ID:      1,
    }

    // Send the request and get the response
    response, err := client.Send(ctx, request)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Result: %s\n", response.Result)

    // Send a notification
    notification := &jsonrpc.Notification{
        Jsonrpc: "2.0",
        Method:  "logEvent",
        Params:  []byte(`{"event": "user_login"}`),
    }

    err = client.Notify(ctx, notification)
    if err != nil {
        panic(err)
    }
}
```

#### Server Usage

The SSE server handles HTTP requests and maintains SSE connections:

```go
package main

import (
    "context"
    "fmt"
	"encoding/json"
    "github.com/viant/jsonrpc"
    "github.com/viant/jsonrpc/transport"
    "github.com/viant/jsonrpc/transport/server/http/sse"
    "net/http"
)

// Define a handler for JSON-RPC methods
type Handler struct{}

func (h *Handler) Handle(ctx context.Context, method string, params []byte) (interface{}, error) {
    switch method {
    case "getData":
        var args struct {
            ID int `json:"id"`
        }
        if err := json.Unmarshal(params, &args); err != nil {
            return nil, err
        }
        return map[string]string{"data": fmt.Sprintf("Data for ID %d", args.ID)}, nil
    default:
        return nil, jsonrpc.NewError(jsonrpc.MethodNotFoundError, "Method not found", nil)
    }
}

func main() {
    // Create a new handler factory
    newHandler := func(ctx context.Context) transport.Handler {
        return &Handler{}
    }

    // Create a new SSE handler
    handler := sse.New(newHandler,
        sse.WithSSEURI("/events"),
        sse.WithMessageURI("/rpc"),
    )

    // Register the handler with an HTTP server
    http.Handle("/events", handler)
    http.Handle("/rpc", handler)

    // Start the HTTP server
    if err := http.ListenAndServe(":8080", nil); err != nil {
        panic(err)
    }
}
```

## Message Types

The package provides the following message types:

* `Request` - Represents a JSON-RPC request containing method name, parameters, and an ID
* `Notification` - Similar to a request but without an ID, indicating no response is expected
* `Response` - Contains the result of a request or an error if the request failed
* `Error` - Provides detailed error information with error code, message, and optional data

## Error Codes

As per the JSON-RPC 2.0 specification, the following error codes are defined:

| Code     | Constant          | Description                                                                       |
|----------|--------------------|-----------------------------------------------------------------------------------|
| -32700   | ParseError         | Invalid JSON received                                                             |
| -32600   | InvalidRequest     | The JSON sent is not a valid Request object                                       |
| -32601   | MethodNotFound     | The method does not exist or is not available                                     |
| -32602   | InvalidParams      | Invalid method parameters                                                         |
| -32603   | InternalError      | Internal JSON-RPC error                                                           |


## License

The source code is made available under the [LICENSE](LICENSE) file.

## Contribution

Feel free to submit issues, fork the repository and send pull requests!

## Credits

Author: Adrian Witas

This project is maintained by [Viant](https://github.com/viant).
