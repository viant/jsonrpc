package base

import (
	"fmt"
	"strings"
	"testing"

	"github.com/viant/jsonrpc"
)

type secretParseError struct{}

func (*secretParseError) Error() string { panic("parser error text must never be evaluated") }

type parseLogger struct{ entry, arguments string }

func (l *parseLogger) Errorf(format string, args ...interface{}) {
	l.entry = fmt.Sprintf(format, args...)
	l.arguments = fmt.Sprint(args...)
}

func TestLogParseErrorNeverPassesErrorToLogger(t *testing.T) {
	logger := &parseLogger{}
	LogParseError(logger, jsonrpc.MessageTypeRequest, &secretParseError{}, 123)
	if !strings.Contains(logger.entry, "*base.secretParseError") || !strings.Contains(logger.arguments, "123") {
		t.Fatalf("missing safe diagnostics: %s", logger.entry)
	}
	LogParseError(nil, jsonrpc.MessageTypeRequest, &secretParseError{}, 123)
}
