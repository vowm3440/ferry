package rulesets

import (
	"context"
	"os"
	"testing"
)

// This opt-in compatibility check exercises current catalog artifacts without
// introducing public-network dependencies into the normal test suite.
func TestCurrentUpstreamArtifacts(t *testing.T) {
	if os.Getenv("FERRY_TEST_RULESET_UPSTREAM") != "1" {
		t.Skip("explicit upstream compatibility check only")
	}
	m := New(t.TempDir())
	for _, item := range Catalog() {
		t.Run(item.ID, func(t *testing.T) {
			if err := m.Update(context.Background(), item.ID); err != nil {
				t.Fatal(err)
			}
			rules, err := m.Load(item.ID)
			if err != nil || len(rules.Rules) == 0 {
				t.Fatalf("empty or invalid artifact: %v", err)
			}
		})
	}
}
