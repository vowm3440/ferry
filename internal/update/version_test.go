package update_test

import "testing"

func TestVerifyManifestUsesSemanticVersionPrecedence(t *testing.T) {
	cases := []struct {
		current, next string
		ok            bool
	}{
		{"1.0.9", "1.0.10", true},
		{"1.9.9", "1.10.0", true},
		{"1.2.3-alpha.9", "1.2.3-alpha.10", true},
		{"1.2.3-alpha", "1.2.3-alpha.1", true},
		{"1.2.3-alpha.1", "1.2.3-alpha.beta", true},
		{"1.2.3-beta.2", "1.2.3-beta.11", true},
		{"1.2.3-rc.1", "1.2.3", true},
		{"0.1.0-dev", "0.1.0", true},
		{"1.2.3", "1.2.3-rc.1", false},
		{"1.2.3", "1.2.2", false},
		{"1.2.3", "1.2.3", false},
		{"1.2.3+old", "1.2.3+new", false},
		{"999999999999999999999.0.0", "1000000000000000000000.0.0", true},
		{"1.2.3-alpha.999999999999999999999", "1.2.3-alpha.1000000000000000000000", true},
	}
	for _, tc := range cases {
		t.Run(tc.current+"_to_"+tc.next, func(t *testing.T) {
			raw := manifestJSON(tc.next, "stable", "https://example.com/ferry.zip", 5)
			_, err := verifyFixture(t, raw, tc.current, "stable")
			if (err == nil) != tc.ok {
				t.Fatalf("acceptance = %v, want %v: %v", err == nil, tc.ok, err)
			}
		})
	}
}

func TestVerifyManifestRejectsMalformedVersions(t *testing.T) {
	versions := []string{"", "v1.2.3", "1.2", "1.2.3.4", "01.2.3", "1.02.3", "1.2.03",
		"1.2.3-", "1.2.3+", "1.2.3-01", "1.2.3-a..b", "1.2.3+a..b", "1.2.3+a+b", "1.2.3-你好", " 1.2.3"}
	for _, version := range versions {
		t.Run(version, func(t *testing.T) {
			raw := manifestJSON(version, "stable", "https://example.com/ferry.zip", 5)
			if _, err := verifyFixture(t, raw, "0.0.0", "stable"); err == nil {
				t.Fatal("invalid next version accepted")
			}
			raw = manifestJSON("2.0.0", "stable", "https://example.com/ferry.zip", 5)
			if _, err := verifyFixture(t, raw, version, "stable"); err == nil {
				t.Fatal("invalid current version accepted")
			}
		})
	}
}
