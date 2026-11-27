package rulesets

import "testing"

func TestEmptyDecodedSetsCannotReplaceUsableCache(t *testing.T) {
	for _, payload := range [][]byte{{0}, {1, 0, 3, 0, 255, 0}} {
		if _, err := validateSRS(compressedSRS(t, payload)); err == nil {
			t.Fatal("runtime-invalid empty set accepted")
		}
	}
}
