package ipc

import (
	"context"
	"net/http"

	"ferry/internal/telemetry"
)

type Inspector interface {
	LatencyTest(context.Context, []string) (map[string]int, error)
	Connections(context.Context) ([]telemetry.Connection, error)
	CloseConnection(context.Context, string) error
	Logs(context.Context, int64, int) ([]telemetry.LogEntry, error)
	SetLogLevel(context.Context, string) error
}

type latencyRequest struct {
	NodeIDs []string `json:"nodeIds"`
}
type closeRequest struct {
	ID string `json:"id"`
}
type levelRequest struct {
	Level string `json:"level"`
}

func registerInspection(mux *http.ServeMux, backend Backend) {
	b, ok := backend.(Inspector)
	if !ok {
		return
	}
	mux.HandleFunc("POST /v1/latency", func(w http.ResponseWriter, r *http.Request) {
		var input latencyRequest
		if !readJSON(w, r, &input) {
			return
		}
		value, err := b.LatencyTest(r.Context(), input.NodeIDs)
		if err != nil {
			failure(w)
			return
		}
		writeJSON(w, 200, value)
	})
	mux.HandleFunc("GET /v1/connections", func(w http.ResponseWriter, r *http.Request) {
		value, err := b.Connections(r.Context())
		if err != nil {
			failure(w)
			return
		}
		writeJSON(w, 200, value)
	})
	mux.HandleFunc("POST /v1/connections/close", func(w http.ResponseWriter, r *http.Request) {
		var input closeRequest
		if readJSON(w, r, &input) {
			complete(w, b.CloseConnection(r.Context(), input.ID))
		}
	})
	mux.HandleFunc("POST /v1/logs", func(w http.ResponseWriter, r *http.Request) {
		var input telemetry.LogRequest
		if !readJSON(w, r, &input) {
			return
		}
		value, err := b.Logs(r.Context(), input.After, input.Limit)
		if err != nil {
			failure(w)
			return
		}
		writeJSON(w, 200, value)
	})
	mux.HandleFunc("POST /v1/log-level", func(w http.ResponseWriter, r *http.Request) {
		var input levelRequest
		if readJSON(w, r, &input) {
			complete(w, b.SetLogLevel(r.Context(), input.Level))
		}
	})
}

func (c *Client) LatencyTest(ctx context.Context, ids []string) (map[string]int, error) {
	var result map[string]int
	err := c.call(ctx, "POST", "/v1/latency", latencyRequest{ids}, &result)
	return result, err
}
func (c *Client) Connections(ctx context.Context) ([]telemetry.Connection, error) {
	result := []telemetry.Connection{}
	err := c.call(ctx, "GET", "/v1/connections", nil, &result)
	return result, err
}
func (c *Client) CloseConnection(ctx context.Context, id string) error {
	return c.call(ctx, "POST", "/v1/connections/close", closeRequest{id}, nil)
}
func (c *Client) Logs(ctx context.Context, after int64, limit int) ([]telemetry.LogEntry, error) {
	result := []telemetry.LogEntry{}
	err := c.call(ctx, "POST", "/v1/logs", telemetry.LogRequest{After: after, Limit: limit}, &result)
	return result, err
}
func (c *Client) SetLogLevel(ctx context.Context, level string) error {
	return c.call(ctx, "POST", "/v1/log-level", levelRequest{level}, nil)
}
