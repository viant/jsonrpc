package jsonrpc

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestRequest_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *Request
		wantError bool
	}{
		{
			name:  "valid request",
			input: `{"jsonrpc":"2.0","method":"test","id":1,"params":{"name":"test"}}`,
			want: &Request{
				Jsonrpc: "2.0",
				Method:  "test",
				Id:      float64(1),
				Params:  json.RawMessage(`{"name":"test"}`),
			},
			wantError: false,
		},
		{
			name:      "missing jsonrpc version",
			input:     `{"method":"test","id":1,"params":{"name":"test"}}`,
			want:      nil,
			wantError: true,
		},
		{
			name:      "missing method",
			input:     `{"jsonrpc":"2.0","id":1,"params":{"name":"test"}}`,
			want:      nil,
			wantError: true,
		},
		{
			name:      "missing id",
			input:     `{"jsonrpc":"2.0","method":"test","params":{"name":"test"}}`,
			want:      nil,
			wantError: true,
		},
		{
			name:  "params optional",
			input: `{"jsonrpc":"2.0","method":"test","id":1}`,
			want: &Request{
				Jsonrpc: "2.0",
				Method:  "test",
				Id:      float64(1),
				Params:  json.RawMessage("null"),
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Request
			err := json.Unmarshal([]byte(tt.input), &got)
			
			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if got.Jsonrpc != tt.want.Jsonrpc {
				t.Errorf("Jsonrpc: got %v, want %v", got.Jsonrpc, tt.want.Jsonrpc)
			}
			
			if got.Method != tt.want.Method {
				t.Errorf("Method: got %v, want %v", got.Method, tt.want.Method)
			}
			
			if !reflect.DeepEqual(got.Id, tt.want.Id) {
				t.Errorf("Id: got %v (%T), want %v (%T)", got.Id, got.Id, tt.want.Id, tt.want.Id)
			}
			
			// Compare params as strings to avoid issues with marshaling differences
			gotParams := string(got.Params)
			wantParams := string(tt.want.Params)
			if gotParams != wantParams && gotParams != "null" && wantParams != "null" {
				t.Errorf("Params: got %v, want %v", gotParams, wantParams)
			}
		})
	}
}

func TestNotification_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *Notification
		wantError bool
	}{
		{
			name:  "valid notification",
			input: `{"jsonrpc":"2.0","method":"test","params":{"name":"test"}}`,
			want: &Notification{
				Jsonrpc: "2.0",
				Method:  "test",
				Params:  json.RawMessage(`{"name":"test"}`),
			},
			wantError: false,
		},
		{
			name:      "missing jsonrpc version",
			input:     `{"method":"test","params":{"name":"test"}}`,
			want:      nil,
			wantError: true,
		},
		{
			name:      "missing method",
			input:     `{"jsonrpc":"2.0","params":{"name":"test"}}`,
			want:      nil,
			wantError: true,
		},
		{
			name:      "with id field (not allowed)",
			input:     `{"jsonrpc":"2.0","method":"test","id":1,"params":{"name":"test"}}`,
			want:      nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Notification
			err := json.Unmarshal([]byte(tt.input), &got)
			
			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if got.Jsonrpc != tt.want.Jsonrpc {
				t.Errorf("Jsonrpc: got %v, want %v", got.Jsonrpc, tt.want.Jsonrpc)
			}
			
			if got.Method != tt.want.Method {
				t.Errorf("Method: got %v, want %v", got.Method, tt.want.Method)
			}
		})
	}
}

func TestResponse_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      *Response
		wantError bool
	}{
		{
			name:  "valid response",
			input: `{"jsonrpc":"2.0","id":1,"result":{"status":"ok"}}`,
			want: &Response{
				Jsonrpc: "2.0",
				Id:      float64(1),
				Result:  json.RawMessage(`{"status":"ok"}`),
			},
			wantError: false,
		},
		{
			name:      "missing jsonrpc version",
			input:     `{"id":1,"result":{"status":"ok"}}`,
			want:      nil,
			wantError: true,
		},
		{
			name:      "missing id",
			input:     `{"jsonrpc":"2.0","result":{"status":"ok"}}`,
			want:      nil,
			wantError: true,
		},
		{
			name:      "missing result",
			input:     `{"jsonrpc":"2.0","id":1}`,
			want:      nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got Response
			err := json.Unmarshal([]byte(tt.input), &got)
			
			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if got.Jsonrpc != tt.want.Jsonrpc {
				t.Errorf("Jsonrpc: got %v, want %v", got.Jsonrpc, tt.want.Jsonrpc)
			}
			
			if !reflect.DeepEqual(got.Id, tt.want.Id) {
				t.Errorf("Id: got %v (%T), want %v (%T)", got.Id, got.Id, tt.want.Id, tt.want.Id)
			}
			
			// Compare result as strings to avoid issues with marshaling differences
			gotResult := string(got.Result)
			wantResult := string(tt.want.Result)
			if gotResult != wantResult {
				t.Errorf("Result: got %v, want %v", gotResult, wantResult)
			}
		})
	}
}

func TestMessage_MarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		message  *Message
		expected string
	}{
		{
			name: "request message",
			message: NewRequestMessage(&Request{
				Jsonrpc: "2.0",
				Method:  "test",
				Id:      1,
				Params:  json.RawMessage(`{"name":"test"}`),
			}),
			expected: `{"jsonrpc":"2.0","id":1,"method":"test","params":{"name":"test"}}`,
		},
		{
			name: "notification message",
			message: NewNotificationMessage(&Notification{
				Jsonrpc: "2.0",
				Method:  "notify",
				Params:  json.RawMessage(`{"event":"update"}`),
			}),
			expected: `{"jsonrpc":"2.0","method":"notify","params":{"event":"update"}}`,
		},
		{
			name: "response message",
			message: NewResponseMessage(&Response{
				Jsonrpc: "2.0",
				Id:      2,
				Result:  json.RawMessage(`{"status":"ok"}`),
			}),
			expected: `{"jsonrpc":"2.0","id":2,"result":{"status":"ok"}}`,
		},
		{
			name: "error message",
			message: NewErrorMessage(&Error{
				Jsonrpc: "2.0",
				Id:      3,
				Error: InnerError{
					Code:    -32600,
					Message: "Invalid Request",
					Data:    "Details here",
				},
			}),
			expected: `{"error":{"code":-32600,"data":"Details here","message":"Invalid Request"},"id":3,"jsonrpc":"2.0"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.message)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			// Compare JSON objects
			var gotObj, expectedObj interface{}
			if err := json.Unmarshal(got, &gotObj); err != nil {
				t.Errorf("Failed to unmarshal result: %v", err)
			}
			if err := json.Unmarshal([]byte(tt.expected), &expectedObj); err != nil {
				t.Errorf("Failed to unmarshal expected: %v", err)
			}
			
			if !reflect.DeepEqual(gotObj, expectedObj) {
				t.Errorf("Message JSON\ngot:  %s\nwant: %s", got, tt.expected)
			}
		})
	}
}
