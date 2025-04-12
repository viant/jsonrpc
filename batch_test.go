package jsonrpc

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBatchRequest_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		want    BatchRequest
		wantErr bool
	}{
		{
			name: "Valid batch request",
			data: `[
				{"jsonrpc": "2.0", "method": "sum", "params": [1,2,4], "id": 1},
				{"jsonrpc": "2.0", "method": "notify_hello", "params": [7]},
				{"jsonrpc": "2.0", "method": "subtract", "params": [42,23], "id": 2}
			]`,
			want: BatchRequest{
				&Request{Jsonrpc: "2.0", Method: "sum", Params: json.RawMessage(`[1,2,4]`), Id: float64(1)},
				&Request{Jsonrpc: "2.0", Method: "notify_hello", Params: json.RawMessage(`[7]`), Id: nil},
				&Request{Jsonrpc: "2.0", Method: "subtract", Params: json.RawMessage(`[42,23]`), Id: float64(2)},
			},
			wantErr: false,
		},
		{
			name:    "Empty array",
			data:    `[]`,
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid JSON",
			data:    `[{"jsonrpc": "2.0", "method": "sum", "params": [1,2,4], "id": 1},]`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var br BatchRequest
			err := json.Unmarshal([]byte(tt.data), &br)
			if (err != nil) != tt.wantErr {
				t.Errorf("BatchRequest.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				// Check only the structure, not exact values
				if len(br) != len(tt.want) {
					t.Errorf("BatchRequest.UnmarshalJSON() got length = %d, want %d", len(br), len(tt.want))
				}
			}
		})
	}
}

func TestBatchResponse_MarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		br      BatchResponse
		want    string
		wantErr bool
	}{
		{
			name: "Mixed responses and errors",
			br: BatchResponse{
				&Response{Id: float64(1), Jsonrpc: "2.0", Result: json.RawMessage(`{"result":3}`)},
				&Error{Id: float64(2), Jsonrpc: "2.0", Error: InnerError{Code: -32600, Message: "Invalid Request"}},
			},
			want:    `[{"id":1,"jsonrpc":"2.0","result":{"result":3}},{"error":{"code":-32600,"message":"Invalid Request"},"id":2,"jsonrpc":"2.0"}]`,
			wantErr: false,
		},
		{
			name:    "Empty batch response",
			br:      BatchResponse{},
			want:    `[]`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.br)
			if (err != nil) != tt.wantErr {
				t.Errorf("BatchResponse.MarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				var gotObj, wantObj interface{}
				json.Unmarshal(got, &gotObj)
				json.Unmarshal([]byte(tt.want), &wantObj)

				if !reflect.DeepEqual(gotObj, wantObj) {
					t.Errorf("BatchResponse.MarshalJSON() = %v, want %v", string(got), tt.want)
				}
			}
		})
	}
}

func TestNewBatchResponseHelpers(t *testing.T) {
	responses := []*Response{
		{Id: float64(1), Jsonrpc: "2.0", Result: json.RawMessage(`{"result":3}`)},
		{Id: float64(2), Jsonrpc: "2.0", Result: json.RawMessage(`{"result":5}`)},
	}
	
	errors := []*Error{
		{Id: float64(3), Jsonrpc: "2.0", Error: InnerError{Code: -32600, Message: "Invalid Request"}},
		{Id: float64(4), Jsonrpc: "2.0", Error: InnerError{Code: -32601, Message: "Method not found"}},
	}
	
	t.Run("NewBatchResponseFromResponses", func(t *testing.T) {
		br := NewBatchResponseFromResponses(responses)
		if len(br) != len(responses) {
			t.Errorf("NewBatchResponseFromResponses() length = %d, want %d", len(br), len(responses))
		}
	})
	
	t.Run("NewBatchResponseFromErrors", func(t *testing.T) {
		br := NewBatchResponseFromErrors(errors)
		if len(br) != len(errors) {
			t.Errorf("NewBatchResponseFromErrors() length = %d, want %d", len(br), len(errors))
		}
	})
	
	t.Run("NewBatchResponseMixed", func(t *testing.T) {
		br := NewBatchResponseMixed(responses, errors)
		if len(br) != len(responses)+len(errors) {
			t.Errorf("NewBatchResponseMixed() length = %d, want %d", len(br), len(responses)+len(errors))
		}
	})
}
