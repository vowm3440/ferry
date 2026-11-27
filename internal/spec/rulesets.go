package spec

// RuleSetID maps the public rule grammar to fixed internal catalogue IDs.
// It deliberately does not accept URLs, paths or arbitrary remote set names.
func RuleSetID(reference string) (string, bool) {
	switch reference {
	case "geosite:cn":
		return "geosite-cn", true
	case "geosite:geolocation-!cn":
		return "geosite-non-cn", true
	case "geoip:cn":
		return "geoip-cn", true
	case "geoip:private":
		return "geoip-private", true
	default:
		return "", false
	}
}

func ValidRuleSetID(id string) bool {
	switch id {
	case "geosite-cn", "geosite-non-cn", "geoip-cn", "geoip-private":
		return true
	default:
		return false
	}
}

func RuleSetCatalogIDs() []string {
	return []string{"geosite-cn", "geosite-non-cn", "geoip-cn", "geoip-private"}
}

func (s RunSpec) ReferencedRuleSets() []string {
	result := []string{}
	seen := map[string]bool{}
	for _, rule := range s.Rules {
		if !rule.Enabled {
			continue
		}
		for _, host := range rule.Hosts {
			if id, ok := RuleSetID(host); ok && !seen[id] {
				seen[id] = true
				result = append(result, id)
			}
		}
	}
	return result
}
