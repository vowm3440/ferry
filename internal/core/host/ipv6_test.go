package host

import (
	"net"
	"testing"

	"ferry/internal/spec"
)

func TestEffectiveSpecGuardsIPv4OnlyTunHosts(t *testing.T) {
	base := spec.Default()
	base.Inbound.Tun.Enabled = true
	base.DNS.Strategy = "prefer_ipv4"

	hNoV6 := New()
	hNoV6.hasIPv6 = func() bool { return false }
	if got := hNoV6.effectiveSpec(base); got.DNS.Strategy != "ipv4_only" {
		t.Fatalf("expected ipv4_only on IPv4-only TUN host, got %q", got.DNS.Strategy)
	}

	explicitV6 := base
	explicitV6.DNS.Strategy = "ipv6_only"
	if got := hNoV6.effectiveSpec(explicitV6); got.DNS.Strategy != "ipv6_only" {
		t.Fatalf("expected explicit ipv6_only preserved, got %q", got.DNS.Strategy)
	}

	hWithV6 := New()
	hWithV6.hasIPv6 = func() bool { return true }
	if got := hWithV6.effectiveSpec(base); got.DNS.Strategy != "prefer_ipv4" {
		t.Fatalf("expected prefer_ipv4 preserved on dual-stack host, got %q", got.DNS.Strategy)
	}

	noTun := base
	noTun.Inbound.Tun.Enabled = false
	if got := hNoV6.effectiveSpec(noTun); got.DNS.Strategy != "prefer_ipv4" {
		t.Fatalf("expected non-TUN strategy unchanged, got %q", got.DNS.Strategy)
	}
}

func TestHasPublicIPv6AddrFiltersPrivateAndLinkLocal(t *testing.T) {
	parse := func(cidr string) net.Addr {
		ip, n, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatal(err)
		}
		return &net.IPNet{IP: ip, Mask: n.Mask}
	}
	nonPublic := []net.Addr{
		parse("192.168.137.194/24"),
		parse("fe80::1/64"),
		parse("fdfe:dcba:9876::1/126"),
		parse("::1/128"),
	}
	if hasPublicIPv6Addr(nonPublic) {
		t.Fatal("non-public IPv6 addresses falsely detected as global IPv6")
	}
	withPublic := append(nonPublic, parse("240e:e9:6002::1/64"))
	if !hasPublicIPv6Addr(withPublic) {
		t.Fatal("global IPv6 address not detected")
	}
}
