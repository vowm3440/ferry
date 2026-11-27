//go:build !with_clash_api

package boxinclude

import (
	"context"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/experimental"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
)

func init() {
	experimental.RegisterClashServerConstructor(func(context.Context, log.ObservableFactory, option.ClashAPIOptions) (adapter.LifecycleService, error) {
		return nil, E.New("clash api is not included in this build")
	})
}
