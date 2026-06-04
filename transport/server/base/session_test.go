package base

import "testing"

func TestSession_WriteBuffered_NilWriter(t *testing.T) {
	session := &Session{}
	if session.WriteBuffered([]byte("x")) {
		t.Fatalf("WriteBuffered() = true, want false for detached session")
	}
	if session.WriteKeepAlive([]byte(": keepalive\n\n")) {
		t.Fatalf("WriteKeepAlive() = true, want false for detached session")
	}
}
