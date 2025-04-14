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
