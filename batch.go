package jsonrpc

import (
	"encoding/json"
	"errors"
)

// BatchRequest represents a JSON-RPC 2.0 batch request as per specs
type BatchRequest []*Request

// BatchResponse represents a JSON-RPC 2.0 batch response as per specs
type BatchResponse []*Response

// UnmarshalJSON is a custom JSON unmarshaler for the BatchRequest type
func (b *BatchRequest) UnmarshalJSON(data []byte) error {
	// First check if it's an empty array which is not allowed as per the specs
	if string(data) == "[]" {
		return errors.New("invalid batch request: empty array")
	}

	// Try to unmarshal as an array
	var requests []*Request
	err := json.Unmarshal(data, &requests)
	if err != nil {
		return err
	}

	if len(requests) == 0 {
		return errors.New("invalid batch request: empty array")
	}

	*b = requests
	return nil
}
