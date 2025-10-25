# JSON-RPC

[![GoDoc](https://godoc.org/github.com/viant/jsonrpc?status.svg)](https://godoc.org/github.com/viant/jsonrpc)
[![Go Report Card](https://goreportcard.com/badge/github.com/viant/jsonrpc)](https://goreportcard.com/report/github.com/viant/jsonrpc)
[![GoReportCard](https://goreportcard.com/badge/github.com/viant/jsonrpc)](https://goreportcard.com/report/github.com/viant/jsonrpc)

This package implements the [JSON-RPC 2.0](https://www.jsonrpc.org/specification) protocol in Go, providing a lightweight and efficient way to create JSON-RPC clients and servers.

## Features

* Full implementation of the JSON-RPC 2.0 specification
* Support for notifications, requests, responses, and batch processing
* Customizable error handling
* Efficient JSON marshaling and unmarshaling
* Thread-safe implementation
* Comprehensive logging capabilities

## Installation

```bash
go get github.com/viant/jsonrpc
```

### HTTP Streamable (NDJSON) Transport

The streaming transport enables JSON-RPC over HTTP with newline-delimited JSON for server push and resumability.

Key points:

* Single endpoint (configurable, e.g., `/rpc`).
* Handshake: `POST <URI>` → returns a session id header (name is configurable, e.g., `X-Session-Id`).
* Exchange: `POST <URI>` (with session header) carries JSON-RPC messages; synchronous JSON response returned.
* Streaming: `GET <URI>` with headers `Accept: application/x-ndjson` **and** the session header opens a newline-delimited JSON stream.
* Each streamed line is an envelope `{"id":<seq>,"data":<jsonrpc>}`. Clients can resume after disconnect using `Last-Event-ID`.

Packages:

```go
// Server
import streamsrv "github.com/viant/jsonrpc/transport/server/http/streamable"

// Client
import streamcli "github.com/viant/jsonrpc/transport/client/http/streamable"
```

Minimal server example:

```go
package main

import (
    "context"
    "net/http"
    "github.com/viant/jsonrpc"
    "github.com/viant/jsonrpc/transport"
    streamsrv "github.com/viant/jsonrpc/transport/server/http/streamable"
    ssnsession "github.com/viant/jsonrpc/transport/server/http/session"
)

type handler struct{}

func (h *handler) Serve(ctx context.Context, req *jsonrpc.Request, resp *jsonrpc.Response) {
    resp.Result = []byte(`"pong"`)
}

func (h *handler) OnNotification(ctx context.Context, n *jsonrpc.Notification) {}

func main() {
    newH := func(ctx context.Context) transport.Handler { return &handler{} }
    // configure a custom header name for the session id
    http.Handle("/rpc", streamsrv.New(newH,
        streamsrv.WithSessionLocation(ssnsession.NewHeaderLocation("X-Session-Id")),
    ))
    _ = http.ListenAndServe(":8080", nil)
}
```

Minimal client example:

```go
package main

import (
    "context"
    "fmt"
    "github.com/viant/jsonrpc"
    streamcli "github.com/viant/jsonrpc/transport/client/http/streamable"
)

func main() {
    ctx := context.Background()
    // Use the same custom header name as the server
    client, _ := streamcli.New(ctx, "http://localhost:8080/rpc",
        streamcli.WithSessionHeaderName("X-Session-Id"),
    )

    req := &jsonrpc.Request{Jsonrpc: "2.0", Method: "ping"}
    resp, _ := client.Send(ctx, req)
    fmt.Println(string(resp.Result)) // pong
}
```

Both SSE and Streamable transports share a common flush helper located at
`transport/server/http/common`.

### Session Lifecycle and Reconnect

Both Streamable and SSE transports support graceful reconnects and eventual session cleanup.

- States: Active (stream attached), Detached (stream closed; pending reconnect), Closed (removed).
- Reconnect: When a stream disconnects, the session moves to Detached. If the client reconnects within a grace period, the server reattaches the stream and replays missed events using `Last-Event-ID`.
- Cleanup: A background sweeper removes sessions based on configured policies and timeouts.

Config options (server):

- WithReconnectGrace(duration): keep Detached sessions for quick reconnects (default: 30s).
- WithIdleTTL(duration): remove sessions idle for longer than TTL (default: 5m).
- WithMaxLifetime(duration): hard cap on any session’s lifetime (default: 1h).
- WithCleanupInterval(duration): sweeper cadence (default: 30s).
- WithMaxEventBuffer(int): number of events kept for replay (default: 1024).
- WithRemovalPolicy(policy): RemovalOnDisconnect | RemovalAfterGrace (default) | RemovalAfterIdle | RemovalManual.
- WithOverflowPolicy(policy): OverflowDropOldest (default) | OverflowMark.
- WithOnSessionClose(func): hook invoked before a session is finally removed.

BFF cookie (optional, dev/prod modes):
- WithBFFCookieSession(BFFCookie{Name, Secure, HttpOnly, SameSite, Path, Domain, MaxAge})
- WithCORSAllowedOrigins([]string{"http://localhost:3000", "https://app.example.com"})
- WithCORSAllowCredentials(true)
Notes: In dev, set `Secure: false` to allow HTTP; in prod, keep `Secure: true` and avoid wildcard origins when credentials are used.

Example (Streamable):

```go
http.Handle("/rpc", streamsrv.New(newH,
    streamsrv.WithReconnectGrace(30*time.Second),
    streamsrv.WithIdleTTL(5*time.Minute),
    streamsrv.WithMaxLifetime(1*time.Hour),
    streamsrv.WithCleanupInterval(30*time.Second),
    streamsrv.WithMaxEventBuffer(1024),
    streamsrv.WithRemovalPolicy(base.RemovalAfterGrace),
))
```

Migration note: To restore legacy behavior that removed sessions immediately on disconnect, set `WithReconnectGrace(0)` and `WithRemovalPolicy(base.RemovalOnDisconnect)`.

### BFF Auth Session (httpOnly cookie)

For browser-based flows where the server (BFF) holds authentication, use a single httpOnly cookie to carry an opaque BFF auth session id (default name suggestion: `BFF-Auth-Session`). This id maps to durable server-side auth state in an `AuthStore` (e.g., Redis). No access or refresh tokens are exposed to the client.

- Transport session header remains header-only and short-lived per client/tab.
- Auth session: `BFF-Auth-Session` cookie persists longer and can rehydrate a new transport session on handshake when no session id is present.
- Rehydrate: enable with `WithRehydrateOnHandshake(true)` and inject an `AuthStore` and cookie settings.
- Logout current vs all: `DELETE` kills only the current transport session; configure a `LogoutAllPath` to revoke the BFF auth session and clear the cookie.

Server options (Streamable):
- `WithAuthStore(store)`: durable auth store (use `auth.NewRedisStore(...)` in prod or `auth.NewMemoryStore(...)` in dev/tests)
- `WithBFFAuthCookie(&BFFAuthCookie{Name: "BFF-Auth-Session", HttpOnly: true, Secure: true, SameSite: http.SameSiteLaxMode})`
- `WithBFFAuthCookieUseTopDomain(true)`: auto Domain=eTLD+1 (prod); omit Domain for localhost/dev
- `WithRehydrateOnHandshake(true)`: mint new transport session using auth cookie when session is missing
- `WithLogoutAllPath("/logout-all")`: optional path to revoke auth and clear cookie

Security notes:
- Use `Access-Control-Allow-Credentials` + exact origins (no wildcard) when sending cookies cross-site.
- Keep cookie `Secure: true` in production; allow `Secure: false` only in dev over HTTP.
- Consider binding the grant to device hints (UA hash, optional IP range) in your AuthStore.

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

    // Send a batch request
    batchRequest := jsonrpc.BatchRequest{
        &jsonrpc.Request{
            Jsonrpc: "2.0",
            Method:  "subtract",
            Params:  []byte(`[42, 23]`),
            Id:      1,
        },
        &jsonrpc.Request{
            Jsonrpc: "2.0",
            Method:  "subtract",
            Params:  []byte(`[23, 42]`),
            Id:      2,
        },
        // A notification (no response expected)
        &jsonrpc.Request{
            Jsonrpc: "2.0",
            Method:  "update",
            Params:  []byte(`[1,2,3,4,5]`),
        },
    }

    responses, err := client.SendBatch(ctx, batchRequest)
    if err != nil {
        panic(err)
    }

    // Process batch responses
    for i, response := range responses {
        fmt.Printf("Response %d: %s\n", i+1, response.Result)
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

#
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
