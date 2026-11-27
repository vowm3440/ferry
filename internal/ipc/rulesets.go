package ipc

import (
	"context"
	"ferry/internal/telemetry"
	"net/http"
)

type RuleSetManager interface {
	RuleSetList(context.Context) ([]telemetry.RuleSetInfo, error)
	RuleSetUpdate(context.Context, string) error
}

func registerRuleSets(mux *http.ServeMux, b Backend) {
	manager, ok := b.(RuleSetManager)
	if !ok {
		return
	}
	mux.HandleFunc("GET /v1/rulesets", func(w http.ResponseWriter, r *http.Request) {
		values, err := manager.RuleSetList(r.Context())
		if err != nil {
			failure(w)
			return
		}
		writeJSON(w, 200, values)
	})
	mux.HandleFunc("POST /v1/rulesets/update", func(w http.ResponseWriter, r *http.Request) {
		var value struct {
			ID string `json:"id"`
		}
		if !readJSON(w, r, &value) {
			return
		}
		complete(w, manager.RuleSetUpdate(r.Context(), value.ID))
	})
}

func (c *Client) RuleSetList(ctx context.Context) ([]telemetry.RuleSetInfo, error) {
	values := []telemetry.RuleSetInfo{}
	err := c.call(ctx, "GET", "/v1/rulesets", nil, &values)
	return values, err
}
func (c *Client) RuleSetUpdate(ctx context.Context, id string) error {
	return c.call(ctx, "POST", "/v1/rulesets/update", struct {
		ID string `json:"id"`
	}{id}, nil)
}
