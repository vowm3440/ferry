package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testBackend struct {
	started bool
	mode    string
	fail    bool
}

func (b *testBackend) Status() State                                    { return State{Status: "stopped", Mode: b.mode} }
func (b *testBackend) Start(_ context.Context, _ json.RawMessage) error { b.started = true; return nil }
func (b *testBackend) Stop(context.Context) error                       { return nil }
func (b *testBackend) SetMode(_ context.Context, mode string) error {
	if b.fail {
		return errors.New("C:\\secret\\password.json token=secret")
	}
	b.mode = mode
	return nil
}
func (b *testBackend) SelectNode(context.Context, string, string) error { return nil }
func (b *testBackend) Snapshot(context.Context) (Traffic, error) {
	return Traffic{UpHistory: make([]int64, 60), DownHistory: make([]int64, 60)}, nil
}

func request(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestHandlerRejectsUnauthenticatedMutation(t *testing.T) {
	b := &testBackend{}
	h := NewHandler(strings.Repeat("a", 64), b)
	w := request(h, "POST", "/v1/start", "wrong", `{}`)
	if w.Code != http.StatusUnauthorized || b.started {
		t.Fatalf("unauthorized request mutated backend: %d", w.Code)
	}
}

func TestHandlerStrictBodiesAndLimits(t *testing.T) {
	token := strings.Repeat("a", 64)
	b := &testBackend{}
	h := NewHandler(token, b)
	for _, body := range []string{`{"mode":"global","unknown":true}`, `{"mode":"global"} {}`, `{"mode":5}`, `{"mode":"nonsense"}`, `{"mode":"rule","mode":"global"}`} {
		w := request(h, "POST", "/v1/mode", token, body)
		if w.Code != 400 || b.mode != "" {
			t.Fatalf("accepted invalid mutation %q (%d)", body, w.Code)
		}
	}
	w := request(h, "POST", "/v1/start", token, strings.Repeat(" ", MaxBodySize+1))
	if w.Code != 413 || b.started {
		t.Fatalf("oversized request accepted: %d", w.Code)
	}
}

func TestHandlerRedactsBackendErrors(t *testing.T) {
	token := strings.Repeat("b", 64)
	w := request(NewHandler(token, &testBackend{fail: true}), "POST", "/v1/mode", token, `{"mode":"global"}`)
	if w.Code != 500 || strings.Contains(w.Body.String(), "secret") || !strings.Contains(w.Body.String(), "失败") {
		t.Fatalf("unsafe error: %s", w.Body.String())
	}
}

func TestHandlerRoutesAndJSON(t *testing.T) {
	token := strings.Repeat("c", 64)
	b := &testBackend{}
	h := NewHandler(token, b)
	for _, tc := range []struct {
		method, path, body string
		code               int
	}{
		{"GET", "/v1/status", "", 200}, {"GET", "/v1/traffic", "", 200},
		{"POST", "/v1/start", `{}`, 200}, {"POST", "/v1/mode", `{"mode":"global"}`, 200},
		{"POST", "/v1/select", `{"groupId":"g","nodeId":"n"}`, 200},
		{"POST", "/v1/stop", `{}`, 200}, {"GET", "/arbitrary", "", 404},
		{"GET", "/v1/start", "", 405},
	} {
		w := request(h, tc.method, tc.path, token, tc.body)
		if w.Code != tc.code {
			t.Errorf("%s %s: %d %s", tc.method, tc.path, w.Code, w.Body.String())
		}
		if tc.code == 200 && !json.Valid(w.Body.Bytes()) {
			t.Errorf("non-JSON response: %s", w.Body.String())
		}
	}
	if !b.started || b.mode != "global" {
		t.Fatal("valid mutations were not applied")
	}
}
