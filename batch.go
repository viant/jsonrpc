package jsonrpc

import (
	"encoding/json"
	"errors"
)

// BatchRequest represents a JSON-RPC 2.0 batch request as per specs
type BatchRequest []*Request

// BatchResponse represents a JSON-RPC 2.0 batch response as per specs
type BatchResponse []interface{}

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

// MarshalJSON is a custom JSON marshaler for BatchResponse
func (b BatchResponse) MarshalJSON() ([]byte, error) {
	if len(b) == 0 {
		return []byte("[]"), nil
	}
	return json.Marshal([]interface{}(b))
}

// NewBatchResponseFromResponses creates a new BatchResponse from a slice of Response objects
func NewBatchResponseFromResponses(responses []*Response) BatchResponse {
	result := make(BatchResponse, len(responses))
	for i, resp := range responses {
		result[i] = resp
	}
	return result
}

// NewBatchResponseFromErrors creates a new BatchResponse from a slice of Error objects
func NewBatchResponseFromErrors(errors []*Response) BatchResponse {
	result := make(BatchResponse, len(errors))
	for i, err := range errors {
		result[i] = err
	}
	return result
}

// NewBatchResponseMixed creates a new BatchResponse from a mix of Response and Error objects
func NewBatchResponseMixed(responses []*Response, errors []*Response) BatchResponse {
	result := make(BatchResponse, 0, len(responses)+len(errors))

	for _, resp := range responses {
		result = append(result, resp)
	}

	for _, err := range errors {
		result = append(result, err)
	}

	return result
}
