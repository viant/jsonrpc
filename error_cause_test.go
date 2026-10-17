package jsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"syscall"
	"testing"
)

func TestErrorLocalCausePreservesWireShape(t *testing.T) {
	for _, cause := range []error{context.DeadlineExceeded, &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}} {
		original := NewInternalError("transport failed", nil)
		before, err := json.Marshal(original)
		if err != nil {
			t.Fatal(err)
		}
		wrapped := original.WithCause(cause)
		if original.Unwrap() != nil || errors.Is(original, cause) {
			t.Fatal("WithCause mutated original")
		}
		if !errors.Is(wrapped, cause) {
			t.Fatal("missing local cause")
		}
		after, err := json.Marshal(wrapped)
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatalf("wire changed: %s vs %s", before, after)
		}
		var received Error
		if err = json.Unmarshal(after, &received); err != nil {
			t.Fatal(err)
		}
		if received.Unwrap() != nil {
			t.Fatal("wire must not manufacture a local transport cause")
		}
	}
}

func TestNilErrorCauseSafe(t *testing.T) {
	var value *Error
	if value.Unwrap() != nil || value.WithCause(context.Canceled) != nil {
		t.Fatal("nil error must remain nil")
	}
}

func TestWireDecodeClearsReusedLocalCause(t *testing.T) {
	original := NewInternalError("local transport", nil).WithCause(context.DeadlineExceeded)
	if err := json.Unmarshal([]byte(`{"code":-32001,"message":"access denied"}`), original); err != nil {
		t.Fatal(err)
	}
	if original.Unwrap() != nil || errors.Is(original, context.DeadlineExceeded) {
		t.Fatal("wire error retained a local transport cause")
	}
	if original.Code != -32001 || original.Message != "access denied" {
		t.Fatal("wire fields not decoded")
	}
}
