package update

import (
	"regexp"
	"strings"
)

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)

type semanticVersion struct {
	core []string
	pre  []string
}

func parseVersion(raw string) (semanticVersion, bool) {
	if len(raw) > 128 {
		return semanticVersion{}, false
	}
	parts := versionPattern.FindStringSubmatch(raw)
	if parts == nil {
		return semanticVersion{}, false
	}
	v := semanticVersion{core: parts[1:4]}
	if parts[4] != "" {
		v.pre = strings.Split(parts[4], ".")
	}
	for _, part := range v.pre {
		if isNumeric(part) && len(part) > 1 && part[0] == '0' {
			return semanticVersion{}, false
		}
	}
	return v, true
}

func newerVersion(next, current string) bool {
	a, validA := parseVersion(next)
	b, validB := parseVersion(current)
	if !validA || !validB {
		return false
	}
	for i := range a.core {
		if comparison := compareIdentifier(a.core[i], b.core[i]); comparison != 0 {
			return comparison > 0
		}
	}
	if len(a.pre) == 0 || len(b.pre) == 0 {
		return len(a.pre) == 0 && len(b.pre) > 0
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		if comparison := compareIdentifier(a.pre[i], b.pre[i]); comparison != 0 {
			return comparison > 0
		}
	}
	return len(a.pre) > len(b.pre)
}

func compareIdentifier(a, b string) int {
	aNumeric, bNumeric := isNumeric(a), isNumeric(b)
	if aNumeric != bNumeric {
		if aNumeric {
			return -1
		}
		return 1
	}
	if aNumeric && len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}

func isNumeric(value string) bool {
	for _, c := range value {
		if c < '0' || c > '9' {
			return false
		}
	}
	return value != ""
}
