package http

import (
	"context"
	"net/http"
)

// Server represents an HTTP server with a handler and address
type Server struct {
	server  http.Server // Embedding the http.Server struct to leverage its fields and methods
	handler http.Handler
	addr    string // Optional address to start the server on
}

func (s *Server) Start() error {
	s.server.Addr = s.addr       // Set the address for the server
	s.server.Handler = s.handler // Set the handler for the server
	return s.server.ListenAndServe()
}

// Shutdown gracefully stops the SSE server, closing all active sessions
// and shutting down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func NewServer(addr string, handler http.Handler) *Server {
	// Create a new instance of the Server struct
	return &Server{
		addr:    addr,
		handler: handler,
	}
}
