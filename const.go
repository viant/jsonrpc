package jsonrpc

// Version is the JSON-RPC protocol version.
const Version = "2.0"

const (
	ParseError     = -32700
	InvalidRequest = -32600
	MethodNotFound = -32601
	InvalidParams  = -32602
	InternalError  = -32603
)

type sessionKey string

// SessionKey is the key used to store the session ID in the context.
const SessionKey = sessionKey("jsonrpc-session")
