package routing

import (
	"ferry/internal/spec"
	"net/netip"
	"regexp"
	"strings"
)

// BuildDNSDomains builds a single DNS matcher map (with OR semantics when multiple condition types exist).
func BuildDNSDomains(s spec.RunSpec, domains []string) map[string]any {
	return logical("or", hostConditions(s, domains))
}

func hostConditions(s spec.RunSpec, hosts []string) []map[string]any {
	var domains, suffixes, expressions, prefixes, sets []string
	for _, host := range hosts {
		if id, ok := s.RuleSetID(host); ok {
			sets = append(sets, id)
			continue
		}
		if values := ipPrefixes(host); len(values) > 0 {
			prefixes = append(prefixes, values...)
			continue
		}
		host = strings.ToLower(strings.TrimSuffix(host, "."))
		switch {
		case strings.HasPrefix(host, "*.") && !strings.ContainsAny(host[2:], "*?"):
			suffixes = append(suffixes, host[1:])
		case strings.ContainsAny(host, "*?"):
			expressions = append(expressions, "^"+globPattern(host)+"$")
		default:
			domains = append(domains, host)
		}
	}
	result := make([]map[string]any, 0, 4)
	if len(sets) > 0 {
		result = append(result, map[string]any{"rule_set": sets})
	}
	if len(domains) > 0 {
		result = append(result, map[string]any{"domain": domains})
	}
	if len(suffixes) > 0 {
		result = append(result, map[string]any{"domain_suffix": suffixes})
	}
	if len(expressions) > 0 {
		result = append(result, map[string]any{"domain_regex": expressions})
	}
	if len(prefixes) > 0 {
		result = append(result, map[string]any{"ip_cidr": prefixes})
	}
	return result
}

func processPattern(app string) string {
	app = strings.ReplaceAll(app, "/", `\`)
	start := "(?i)^"
	if !strings.Contains(app, `\`) {
		start = `(?i)(?:^|\\)`
	}
	return start + globPattern(app) + "$"
}

func globPattern(value string) string {
	var result strings.Builder
	for _, c := range value {
		switch c {
		case '*':
			result.WriteString(".*")
		case '?':
			result.WriteByte('.')
		default:
			result.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	return result.String()
}

func ipPrefixes(value string) []string {
	if address, err := netip.ParseAddr(value); err == nil {
		return []string{netip.PrefixFrom(address, address.BitLen()).String()}
	}
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return []string{prefix.Masked().String()}
	}
	start, end, found := strings.Cut(value, "-")
	if !found {
		return nil
	}
	first, errFirst := netip.ParseAddr(start)
	last, errLast := netip.ParseAddr(end)
	if errFirst != nil || errLast != nil || first.BitLen() != last.BitLen() || first.Compare(last) > 0 {
		return nil
	}
	return rangePrefixes(first, last)
}

func rangePrefixes(first, last netip.Addr) []string {
	result := []string{}
	for first.IsValid() && first.Compare(last) <= 0 {
		prefix := netip.PrefixFrom(first, first.BitLen())
		for bits := first.BitLen() - 1; bits >= 0; bits-- {
			candidate := netip.PrefixFrom(first, bits).Masked()
			if candidate.Addr() != first || prefixEnd(candidate).Compare(last) > 0 {
				break
			}
			prefix = candidate
		}
		result = append(result, prefix.String())
		first = prefixEnd(prefix).Next()
	}
	return result
}

func prefixEnd(prefix netip.Prefix) netip.Addr {
	bytes := prefix.Addr().As16()
	start := prefix.Bits()
	if prefix.Addr().Is4() {
		start += 96
	}
	for bit := start; bit < 128; bit++ {
		bytes[bit/8] |= 1 << (7 - bit%8)
	}
	result := netip.AddrFrom16(bytes)
	if prefix.Addr().Is4() {
		return result.Unmap()
	}
	return result
}
