//go:build !with_quic

package boxinclude

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2ray"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func init() {
	v2ray.RegisterQUICConstructor(
		func(context.Context, logger.ContextLogger, option.V2RayQUICOptions, tls.ServerConfig, adapter.V2RayServerTransportHandler) (adapter.V2RayServerTransport, error) {
			return nil, C.ErrQUICNotIncluded
		},
		func(context.Context, N.Dialer, M.Socksaddr, option.V2RayQUICOptions, tls.Config) (adapter.V2RayClientTransport, error) {
			return nil, C.ErrQUICNotIncluded
		},
	)
}

func registerQUICOutbounds(registry *outbound.Registry) {
	outbound.Register[option.TUICOutboundOptions](registry, C.TypeTUIC, func(context.Context, adapter.Router, log.ContextLogger, string, option.TUICOutboundOptions) (adapter.Outbound, error) {
		return nil, C.ErrQUICNotIncluded
	})
	outbound.Register[option.Hysteria2OutboundOptions](registry, C.TypeHysteria2, func(context.Context, adapter.Router, log.ContextLogger, string, option.Hysteria2OutboundOptions) (adapter.Outbound, error) {
		return nil, C.ErrQUICNotIncluded
	})
}
