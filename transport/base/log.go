package base

import (
	"fmt"

	"github.com/viant/jsonrpc"
)

// LogParseError logs only the error type and message size. Parser errors may
// include input values, so neither their text nor the message is safe to log.
func LogParseError(logger jsonrpc.Logger, messageType jsonrpc.MessageType, err error, size int) {
	if logger != nil {
		logger.Errorf("failed to parse %s: error type %s (message bytes: %d)", messageType, fmt.Sprintf("%T", err), size)
	}
}
