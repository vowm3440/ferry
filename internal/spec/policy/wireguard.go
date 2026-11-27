package policy

import (
	"encoding/base64"
	"net/netip"
)

func validWireGuard(o map[string]any) bool {
	if !validWGKey(o["private_key"], true) || !validWGKey(o["peer_public_key"], true) {
		return false
	}
	if !validWGKey(o["pre_shared_key"], false) || !optionalInteger(o, "mtu", 1280, 65535) {
		return false
	}
	if !validWGAddresses(o["local_address"]) {
		return false
	}
	return validWGReserved(o["reserved"])
}

func validWGKey(value any, required bool) bool {
	if value == nil {
		return !required
	}
	text, ok := value.(string)
	if !ok || text == "" {
		return false
	}
	decoded, err := base64.StdEncoding.DecodeString(text)
	return err == nil && len(decoded) == 32
}

func validWGAddresses(value any) bool {
	items, ok := value.([]any)
	if !ok || len(items) == 0 || len(items) > 4 {
		return false
	}
	for _, item := range items {
		text, ok := item.(string)
		if !ok {
			return false
		}
		prefix, err := netip.ParsePrefix(text)
		if err != nil || prefix.Addr().Zone() != "" {
			return false
		}
	}
	return true
}

func validWGReserved(value any) bool {
	if value == nil {
		return true
	}
	items, ok := value.([]any)
	if !ok || len(items) != 3 {
		return false
	}
	for _, item := range items {
		if !integer(item, 0, 255) {
			return false
		}
	}
	return true
}
