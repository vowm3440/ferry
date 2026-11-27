package ipc

import (
	"context"
	"net/http"

	"ferry/internal/telemetry"
)

type RuleTester interface {
	TestRule(context.Context, telemetry.RuleTestRequest) (telemetry.RuleTestResult, error)
}

func registerRules(mux *http.ServeMux, backend Backend) {
	tester, ok := backend.(RuleTester)
	if !ok {
		return
	}
	mux.HandleFunc("POST /v1/rules/test", func(w http.ResponseWriter, r *http.Request) {
		var input telemetry.RuleTestRequest
		if !readJSON(w, r, &input) {
			return
		}
		value, err := tester.TestRule(r.Context(), input)
		if err != nil {
			failure(w)
			return
		}
		writeJSON(w, 200, value)
	})
}

func (c *Client) TestRule(ctx context.Context, input telemetry.RuleTestRequest) (telemetry.RuleTestResult, error) {
	var result telemetry.RuleTestResult
	err := c.call(ctx, "POST", "/v1/rules/test", input, &result)
	return result, err
}
