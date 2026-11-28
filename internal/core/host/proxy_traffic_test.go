package host

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type stubOutbound struct {
	typ string
	tag string
}

func (s stubOutbound) Type() string                                                           { return s.typ }
func (s stubOutbound) Tag() string                                                            { return s.tag }
func (s stubOutbound) Network() []string                                                      { return []string{N.NetworkTCP, N.NetworkUDP} }
func (s stubOutbound) Dependencies() []string                                                 { return nil }
func (s stubOutbound) DialContext(context.Context, string, M.Socksaddr) (net.Conn, error)     { return nil, nil }
func (s stubOutbound) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error)      { return nil, nil }

func TestIsProxyOutboundFiltersDirectBlockAndDNS(t *testing.T) {
	cases := []struct {
		name  string
		chain []adapter.Outbound
		match adapter.Outbound
		want  bool
	}{
		{name: "nil", want: false},
		{name: "direct type", match: stubOutbound{typ: C.TypeDirect, tag: "__direct"}, want: false},
		{name: "direct tag in chain", chain: []adapter.Outbound{stubOutbound{typ: C.TypeSelector, tag: "proxy"}, stubOutbound{typ: C.TypeDirect, tag: "__direct"}}, want: false},
		{name: "block type", match: stubOutbound{typ: C.TypeBlock, tag: "__block"}, want: false},
		{name: "dns type", match: stubOutbound{typ: C.TypeDNS, tag: "dns-out"}, want: false},
		{name: "reject tag", match: stubOutbound{typ: "custom", tag: "REJECT"}, want: false},
		{name: "proxy node", chain: []adapter.Outbound{stubOutbound{typ: C.TypeSelector, tag: "proxy"}, stubOutbound{typ: C.TypeSOCKS, tag: "n1"}}, want: true},
		{name: "vless node", match: stubOutbound{typ: C.TypeVLESS, tag: "hk-01"}, want: true},
	}
	for _, tc := range cases {
		if got := isProxyOutbound(tc.chain, tc.match); got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestProxyTrafficTrackerCountsOnlyProxyConnectionsAndFlows(t *testing.T) {
	tracker := &proxyTrafficTracker{}
	directOut := stubOutbound{typ: C.TypeDirect, tag: "__direct"}
	proxyOut := stubOutbound{typ: C.TypeSOCKS, tag: "n1"}
	groupOut := stubOutbound{typ: C.TypeSelector, tag: "proxy"}

	// Direct flow is not tracked at all.
	if flow := tracker.RoutedFlow(context.Background(), adapter.InboundContext{OutboundChain: []adapter.Outbound{directOut}}, nil, directOut); flow != nil {
		t.Fatal("direct flow should return nil tracker")
	}
	// Proxy flow counts forward (upload) and reverse (download) bytes.
	flow := tracker.RoutedFlow(context.Background(), adapter.InboundContext{OutboundChain: []adapter.Outbound{groupOut, proxyOut}}, nil, groupOut)
	if flow == nil {
		t.Fatal("proxy flow should return a tracker")
	}
	flow.CountForward(128)
	flow.CountReverse(4096)

	// Direct TCP connection is returned untouched and not counted.
	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()
	wrappedDirect := tracker.RoutedConnection(context.Background(), c1, adapter.InboundContext{OutboundChain: []adapter.Outbound{directOut}}, nil, directOut)
	if wrappedDirect != c1 {
		t.Fatal("direct connection should not be wrapped")
	}

	// Proxy TCP connection counts bytes written back to client as download.
	p1, p2 := net.Pipe()
	defer p1.Close()
	defer p2.Close()
	wrappedProxy := tracker.RoutedConnection(context.Background(), p1, adapter.InboundContext{OutboundChain: []adapter.Outbound{groupOut, proxyOut}}, nil, groupOut)
	done := make(chan struct{})
	go func() {
		defer close(done)
		buf := make([]byte, 32)
		_, _ = p2.Read(buf)
	}()
	if _, err := wrappedProxy.Write([]byte("proxy-payload")); err != nil {
		t.Fatal(err)
	}
	<-done

	if got := tracker.Download(); got != 4096+int64(len("proxy-payload")) {
		t.Fatalf("unexpected proxy download bytes: got %d, want %d", got, 4096+len("proxy-payload"))
	}
	if got := tracker.Upload(); got != 128 {
		t.Fatalf("unexpected proxy upload bytes: got %d, want 128", got)
	}
}

func TestUpdateTrafficUsesProxyDownloadTotalWhenProvided(t *testing.T) {
	h := New()
	h.sampledAt = time.Now().Add(-2 * time.Second)
	h.updateTraffic(connectionSnapshot{
		UploadTotal:        500,
		DownloadTotal:      10_000,
		ProxyDownloadTotal: 1_200,
		HasProxyDown:       true,
	})
	if h.traffic.TotalDown != 1_200 {
		t.Fatalf("TotalDown should reflect proxy-only download, got %d", h.traffic.TotalDown)
	}
	if h.traffic.TotalUp != 500 || h.traffic.Down == 0 {
		t.Fatalf("unexpected upload/rate values: %+v", h.traffic)
	}
	// Direct download spikes while proxy download stays flat.
	h.sampledAt = time.Now().Add(-2 * time.Second)
	h.updateTraffic(connectionSnapshot{
		UploadTotal:        700,
		DownloadTotal:      50_000,
		ProxyDownloadTotal: 1_200,
		HasProxyDown:       true,
	})
	if h.traffic.TotalDown != 1_200 {
		t.Fatalf("direct download should not inflate TotalDown, got %d", h.traffic.TotalDown)
	}
}
