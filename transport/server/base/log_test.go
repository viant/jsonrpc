package base

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/viant/jsonrpc"
	"github.com/viant/jsonrpc/transport"
)

type payloadLogCapture struct{ entries, arguments []string }

func (l *payloadLogCapture) Errorf(format string, args ...interface{}) {
	l.entries = append(l.entries, fmt.Sprintf(format, args...))
	l.arguments = append(l.arguments, fmt.Sprint(args...))
}

type payloadHandler struct{ requests, notifications int }

func (h *payloadHandler) Serve(_ context.Context, request *jsonrpc.Request, response *jsonrpc.Response) {
	h.requests++
	response.Result = []byte(`"ok"`)
}
func (h *payloadHandler) OnNotification(context.Context, *jsonrpc.Notification) { h.notifications++ }

func TestServerParserLogsExcludePayload(t *testing.T) {
	const marker = "AAECA/8=PRIVATE_ARTIFACT_PAYLOAD"
	const secretNumber = "1234567890123456789012345678901234567890"
	for _, tc := range []struct{ kind, data string }{
		{"notification", `{"method":"tool","jsonrpc":false,"params":"` + marker + `"}`},
		{"response", `{"id":1,"jsonrpc":false,"result":"` + marker + `"}`},
		{"response", `{"id":1,"jsonrpc":"2.0","error":{"code":` + secretNumber + `,"message":"` + marker + `"}}`},
		{"notification", `{"method":"tool","jsonrpc":"2.0","params":"` + marker + `"` + "\x00"},
	} {
		t.Run(tc.kind+fmt.Sprint(len(tc.data)), func(t *testing.T) {
			logger, handler := &payloadLogCapture{}, &payloadHandler{}
			endpoint := &Handler{Logger: logger}
			session := &Session{Handler: handler, RoundTrips: transport.NewRoundTrips(10)}
			endpoint.HandleMessage(context.Background(), session, []byte(tc.data), nil)
			if len(logger.entries) != 1 {
				t.Fatalf("got %d entries, want 1", len(logger.entries))
			}
			for _, value := range append(logger.entries, logger.arguments...) {
				if strings.Contains(value, marker) || strings.Contains(value, secretNumber) || strings.Contains(value, tc.data) {
					t.Fatal("parser logger received payload")
				}
			}
			if handler.notifications != 0 || handler.requests != 0 {
				t.Fatal("malformed message dispatched")
			}
		})
	}
}

func TestServerValidMessagesStillDispatch(t *testing.T) {
	logger, handler := &payloadLogCapture{}, &payloadHandler{}
	endpoint := &Handler{Logger: logger}
	session := &Session{Handler: handler, RoundTrips: transport.NewRoundTrips(10)}
	var output bytes.Buffer
	endpoint.HandleMessage(context.Background(), session, []byte(`{"id":1,"method":"tool","jsonrpc":"2.0","params":{}}`), &output)
	endpoint.HandleMessage(context.Background(), session, []byte(`{"method":"event","jsonrpc":"2.0"}`), nil)
	trip, err := session.RoundTrips.Add(&jsonrpc.Request{Id: 2})
	if err != nil {
		t.Fatal(err)
	}
	endpoint.HandleMessage(context.Background(), session, []byte(`{"id":2,"jsonrpc":"2.0","result":"ok"}`), nil)
	if handler.requests != 1 || handler.notifications != 1 || trip.Response == nil || string(trip.Response.Result) != `"ok"` || len(logger.entries) != 0 || !strings.Contains(output.String(), `"result":"ok"`) {
		t.Fatalf("valid dispatch changed: handler=%#v response=%#v output=%s logs=%v", handler, trip.Response, output.String(), logger.entries)
	}
}
