package jsonrpc

import (
	"encoding/json"
	"testing"
)

func TestNotificationUnmarshalPreservesParams(t *testing.T) {
	var n Notification
	if err := json.Unmarshal([]byte(`{"jsonrpc":"2.0","method":"notifications/resources/updated","params":{"uri":"window://studio/overview","_meta":{"io.modelcontextprotocol/subscriptionId":7}}}`), &n); err != nil {
		t.Fatal(err)
	}
	var params map[string]interface{}
	if err := json.Unmarshal(n.Params, &params); err != nil {
		t.Fatal(err)
	}
	if params["uri"] != "window://studio/overview" {
		t.Fatalf("parameters lost: %s", n.Params)
	}
	if err := json.Unmarshal([]byte(`{"jsonrpc":"2.0","method":"notifications/resources/list_changed"}`), &n); err != nil {
		t.Fatal(err)
	}
	if n.Params != nil {
		t.Fatalf("absent parameters retained old payload: %s", n.Params)
	}
	for _, invalid := range []string{`{"method":"change","params":{}}`, `{"jsonrpc":"2.0","params":{}}`, `{"jsonrpc":"2.0","method":"change","id":1}`} {
		if err := json.Unmarshal([]byte(invalid), &n); err == nil {
			t.Fatalf("accepted invalid notification: %s", invalid)
		}
	}
}
