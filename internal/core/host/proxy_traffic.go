package host

import (
	"context"
	"net"
	"strings"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-tun"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
)

var (
	_ adapter.ConnectionTracker = (*proxyTrafficTracker)(nil)
	_ tun.FlowTracker           = (*proxyFlowTracker)(nil)
)

// proxyTrafficTracker counts bytes transferred exclusively through proxy
// outbounds, ignoring direct, block, and DNS connections.
type proxyTrafficTracker struct {
	upload   atomic.Int64
	download atomic.Int64
}

func (t *proxyTrafficTracker) Download() int64 {
	return t.download.Load()
}

func (t *proxyTrafficTracker) Upload() int64 {
	return t.upload.Load()
}

func (t *proxyTrafficTracker) RoutedConnection(_ context.Context, conn net.Conn, metadata adapter.InboundContext, _ adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	if !isProxyOutbound(metadata.OutboundChain, matchOutbound) {
		return conn
	}
	return bufio.NewInt64CounterConn(conn, []*atomic.Int64{&t.upload}, []*atomic.Int64{&t.download})
}

func (t *proxyTrafficTracker) RoutedPacketConnection(_ context.Context, conn N.PacketConn, metadata adapter.InboundContext, _ adapter.Rule, matchOutbound adapter.Outbound) N.PacketConn {
	if !isProxyOutbound(metadata.OutboundChain, matchOutbound) {
		return conn
	}
	return bufio.NewInt64CounterPacketConn(conn, []*atomic.Int64{&t.upload}, nil, []*atomic.Int64{&t.download}, nil)
}

func (t *proxyTrafficTracker) RoutedFlow(_ context.Context, metadata adapter.InboundContext, _ adapter.Rule, matchOutbound adapter.Outbound) tun.FlowTracker {
	if !isProxyOutbound(metadata.OutboundChain, matchOutbound) {
		return nil
	}
	return &proxyFlowTracker{upload: &t.upload, download: &t.download}
}

func isProxyOutbound(chain []adapter.Outbound, matchOutbound adapter.Outbound) bool {
	outbound := matchOutbound
	if len(chain) > 0 {
		outbound = chain[len(chain)-1]
	}
	if outbound == nil {
		return false
	}
	switch outbound.Type() {
	case C.TypeDirect, C.TypeBlock, C.TypeDNS:
		return false
	}
	switch strings.ToLower(outbound.Tag()) {
	case "__direct", "direct", "__block", "reject":
		return false
	}
	return true
}

type proxyFlowTracker struct {
	upload   *atomic.Int64
	download *atomic.Int64
}

func (f *proxyFlowTracker) AttachFlow(tun.FlowHandle)      {}
func (f *proxyFlowTracker) CountForward(n int)             { f.upload.Add(int64(n)) }
func (f *proxyFlowTracker) CountReverse(n int)             { f.download.Add(int64(n)) }
func (f *proxyFlowTracker) FlowEstablished()               {}
func (f *proxyFlowTracker) CloseFlow(tun.FlowCloseReason)  {}
