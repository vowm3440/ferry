package policy

import "strings"

func validTLS(value any) bool {
	o, ok := value.(map[string]any)
	if !ok || !allowed(o, "enabled server_name insecure alpn utls reality disable_sni") {
		return false
	}
	if o["enabled"] != true {
		for key := range o {
			if key != "enabled" {
				return false
			}
		}
	}
	for _, key := range []string{"enabled", "insecure", "disable_sni"} {
		if value, exists := o[key]; exists {
			if _, ok := value.(bool); !ok {
				return false
			}
		}
	}
	if value, exists := o["server_name"]; exists {
		name, ok := value.(string)
		if !ok || !Host(name) {
			return false
		}
	}
	if value, exists := o["alpn"]; exists {
		list, ok := value.([]any)
		if !ok || len(list) > 16 {
			return false
		}
		for _, item := range list {
			text, ok := item.(string)
			if !ok || text == "" || len(text) > 255 {
				return false
			}
		}
	}
	if value, exists := o["utls"]; exists && !validUTLS(value) {
		return false
	}
	if value, exists := o["reality"]; exists && !validReality(value) {
		return false
	}
	return true
}

func validTransport(value any) bool {
	o, ok := value.(map[string]any)
	if !ok {
		return false
	}
	kind, _ := o["type"].(string)
	if kind == "grpc" {
		return allowed(o, "type service_name") && optionalStrings(o, "service_name", 2048)
	}
	if kind != "ws" && kind != "http" {
		return false
	}
	if !allowed(o, "type path headers") || !optionalStrings(o, "path", 2048) {
		return false
	}
	if path, exists := o["path"]; exists {
		text := path.(string)
		if !strings.HasPrefix(text, "/") || strings.HasPrefix(text, "//") || strings.Contains(text, `\`) {
			return false
		}
	}
	if value, exists := o["headers"]; exists {
		return validHeaders(value)
	}
	return true
}

func validHeaders(value any) bool {
	o, ok := value.(map[string]any)
	if !ok || len(o) > 32 {
		return false
	}
	for key := range o {
		if len(key) == 0 || len(key) > 128 || strings.ContainsAny(key, " \r\n:\t") {
			return false
		}
		if !optionalStrings(o, key, 2048) {
			return false
		}
	}
	return true
}
