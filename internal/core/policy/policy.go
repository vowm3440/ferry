// Package policy exposes the core's strict outbound configuration boundary.
// The implementation lives beneath spec so GUI validation imports no core code.
package policy

import (
	"encoding/json"
	inputpolicy "ferry/internal/spec/policy"
)

func SanitizeOutbound(data json.RawMessage) (json.RawMessage, error) {
	return inputpolicy.SanitizeOutbound(data)
}

func CheckJSON(data []byte, maxDepth int) error {
	return inputpolicy.CheckJSON(data, maxDepth)
}
