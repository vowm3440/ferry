package host

import (
	"net"
	"net/netip"
	"strings"

	"ferry/internal/spec"
)

// effectiveSpec avoids creating a blackhole IPv6 default route (::/0) on TUN
// when the host has no public IPv6 connectivity. Without this, split DNS rules
// that return real AAAA records for domestic direct domains cause browsers to
// connect to unreachable IPv6 addresses and fail with ERR_CONNECTION_RESET.
func (h *Host) effectiveSpec(s spec.RunSpec) spec.RunSpec {
	if !s.Inbound.Tun.Enabled || s.DNS.Strategy == "ipv6_only" {
		return s
	}
	check := h.hasIPv6
	if check == nil {
		check = detectGlobalIPv6
	}
	if !check() {
		s.DNS.Strategy = "ipv4_only"
	}
	return s
}

func detectGlobalIPv6() bool {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if strings.EqualFold(iface.Name, "FerryTun") {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		if hasPublicIPv6Addr(addrs) {
			return true
		}
	}
	return false
}

func hasPublicIPv6Addr(addrs []net.Addr) bool {
	for _, raw := range addrs {
		ipNet, ok := raw.(*net.IPNet)
		if !ok {
			continue
		}
		addr, ok := netip.AddrFromSlice(ipNet.IP)
		if !ok {
			continue
		}
		addr = addr.Unmap()
		if addr.Is6() && addr.IsGlobalUnicast() && !addr.IsPrivate() {
			return true
		}
	}
	return false
}
