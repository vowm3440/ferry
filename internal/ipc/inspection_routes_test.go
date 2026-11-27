package ipc

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ferry/internal/telemetry"
)

type routeBackend struct {
	testBackend
	fail            bool
	closedID, level string
}

func (b *routeBackend) LatencyTest(context.Context, []string) (map[string]int, error) {
	if b.fail {
		return nil, errors.New("secret")
	}
	return map[string]int{"node": 12}, nil
}
func (b *routeBackend) Connections(context.Context) ([]telemetry.Connection, error) {
	if b.fail {
		return nil, errors.New("secret")
	}
	return []telemetry.Connection{{ID: "connection"}}, nil
}
func (b *routeBackend) CloseConnection(_ context.Context, id string) error {
	b.closedID = id
	return nil
}
func (b *routeBackend) Logs(context.Context, int64, int) ([]telemetry.LogEntry, error) {
	if b.fail {
		return nil, errors.New("secret")
	}
	return []telemetry.LogEntry{{Seq: 9, Message: "line"}}, nil
}
func (b *routeBackend) SetLogLevel(_ context.Context, level string) error {
	b.level = level
	return nil
}
func (b *routeBackend) TestRule(context.Context, telemetry.RuleTestRequest) (telemetry.RuleTestResult, error) {
	if b.fail {
		return telemetry.RuleTestResult{}, errors.New("secret")
	}
	return telemetry.RuleTestResult{MatchedRuleID: "test", Action: "block"}, nil
}

func TestInspectionHTTPRejectsMalformedAndRedactsErrors(t *testing.T) {
	token := strings.Repeat("a", 64)
	backend := &routeBackend{}
	handler := NewHandler(token, backend)
	for _, tc := range []struct{ path, body string }{
		{"/v1/latency", `{"nodeIds":["node"]}`}, {"/v1/logs", `{"after":0,"limit":20}`}, {"/v1/rules/test", `{"app":"test","host":"example.com","port":443,"network":"tcp","config":{}}`},
	} {
		if response := request(handler, "POST", tc.path, token, tc.body); response.Code != 200 {
			t.Fatalf("%s: %d", tc.path, response.Code)
		}
		if response := request(handler, "POST", tc.path, token, `{"unknown":true}`); response.Code != 400 {
			t.Fatalf("%s accepted unknown fields", tc.path)
		}
		backend.fail = true
		response := request(handler, "POST", tc.path, token, tc.body)
		backend.fail = false
		if response.Code != 500 || strings.Contains(response.Body.String(), "secret") {
			t.Fatal("backend error leaked")
		}
	}
	response := request(handler, "POST", "/v1/connections/close", token, `{"id":"connection"}`)
	if response.Code != 200 || backend.closedID != "connection" {
		t.Fatal("close route lost ID")
	}
	response = request(handler, "POST", "/v1/log-level", token, `{"level":"warn"}`)
	if response.Code != 200 || backend.level != "warn" {
		t.Fatal("level route lost value")
	}
}

func TestClientReadsInspectionPayloads(t *testing.T) {
	token := strings.Repeat("b", 64)
	server := httptest.NewServer(NewHandler(token, &routeBackend{}))
	defer server.Close()
	c := &Client{http: server.Client(), token: token}
	// Repoint only the test transport; production remains HTTP over named pipe.
	c.http = &http.Client{Transport: rewriteTransport{base: server.Client().Transport, url: server.URL}}
	ctx := context.Background()
	values, err := c.LatencyTest(ctx, []string{"node"})
	if err != nil || values["node"] != 12 {
		t.Fatal("latency payload missing")
	}
	items, err := c.Connections(ctx)
	if err != nil || len(items) != 1 {
		t.Fatal("connections payload missing")
	}
	logs, err := c.Logs(ctx, 0, 20)
	if err != nil || len(logs) != 1 || logs[0].Seq != 9 {
		t.Fatal("log payload missing")
	}
	if err := c.SetLogLevel(ctx, "warn"); err != nil {
		t.Fatal(err)
	}
	if err := c.CloseConnection(ctx, "connection"); err != nil {
		t.Fatal(err)
	}
}

type rewriteTransport struct {
	base http.RoundTripper
	url  string
}

func (t rewriteTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	req, err := http.NewRequestWithContext(r.Context(), r.Method, t.url+r.URL.Path, r.Body)
	if err != nil {
		return nil, err
	}
	req.Header = r.Header
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}
