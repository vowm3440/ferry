package share

import (
	"context"
	"errors"
	"mime"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"
)

const (
	LANMaxBytes = 1 << 20
	lanTTL      = 10 * time.Minute
)

// LANOptions restricts sharing to a local interface and a short lifetime.
// An empty BindIP selects a non-loopback local address at Start time.
type LANOptions struct {
	BindIP string
	TTL    time.Duration
}

// LANInfo contains a bearer URL. Callers must not log or persist it.
type LANInfo struct {
	Active    bool      `json:"active"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// LAN owns at most one explicitly started, one-use HTTP sharing session.
type LAN struct {
	mu      sync.Mutex
	options LANOptions
	current *lanSession
	closed  bool
}

// NewLAN validates options without opening listeners or inspecting interfaces.
func NewLAN(options LANOptions) (*LAN, error) {
	if options.TTL == 0 {
		options.TTL = lanTTL
	}
	if options.TTL < 0 || options.TTL > lanTTL {
		return nil, errors.New("局域网分享有效期须大于零且不超过 10 分钟")
	}
	if options.BindIP != "" {
		address, err := netip.ParseAddr(options.BindIP)
		if err != nil || (!address.IsGlobalUnicast() && !address.IsLoopback() && !address.IsLinkLocalUnicast()) {
			return nil, errors.New("局域网分享须绑定本机 IP 地址")
		}
	}
	return &LAN{options: options}, nil
}

// Start snapshots data. Cancellation limits this operation; Stop, Close, expiry,
// or the first authorized GET ends the sharing session itself.
func (l *LAN) Start(ctx context.Context, data []byte, mediaType string) (LANInfo, error) {
	if err := ctx.Err(); err != nil {
		return LANInfo{}, err
	}
	mediaType, err := validateLANPayload(data, mediaType)
	if err != nil {
		return LANInfo{}, err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return LANInfo{}, errors.New("局域网分享已关闭")
	}
	if l.current != nil && l.current.status().Active {
		return LANInfo{}, errors.New("局域网分享已开启，请先停止当前分享")
	}
	if l.current != nil {
		if err := l.current.close(); err != nil {
			return LANInfo{}, err
		}
	}
	listener, err := l.listen(ctx)
	if err != nil {
		return LANInfo{}, err
	}
	l.current = newLANSession(listener, data, mediaType, l.options.TTL)
	return l.current.status(), nil
}

func (l *LAN) listen(ctx context.Context) (net.Listener, error) {
	address, err := localLANAddress(l.options.BindIP)
	if err != nil {
		return nil, err
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", net.JoinHostPort(address, "0"))
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("无法启动局域网分享监听")
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(err, listener.Close())
	}
	return listener, nil
}

func validateLANPayload(data []byte, mediaType string) (string, error) {
	if len(data) == 0 || len(data) > LANMaxBytes {
		return "", errors.New("分享内容为空或超过 1 MiB")
	}
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	base, params, err := mime.ParseMediaType(mediaType)
	if err != nil || len(mediaType) > 128 {
		return "", errors.New("分享内容类型无效")
	}
	switch base {
	case "application/octet-stream", "application/json", "application/yaml", "application/x-yaml", "text/plain", "text/yaml", "text/x-yaml":
	default:
		return "", errors.New("不支持的分享内容类型")
	}
	for key, value := range params {
		if key != "charset" || !strings.EqualFold(value, "utf-8") {
			return "", errors.New("分享内容类型参数无效")
		}
	}
	return mime.FormatMediaType(base, params), nil
}

func (l *LAN) Status() LANInfo {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.current == nil {
		return LANInfo{}
	}
	return l.current.status()
}

// Stop revokes the URL and synchronously closes listeners and connections.
func (l *LAN) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.current == nil {
		return nil
	}
	err := l.current.close()
	l.current = nil
	return err
}

// Close permanently disables this owner in addition to stopping its session.
func (l *LAN) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.closed = true
	if l.current == nil {
		return nil
	}
	err := l.current.close()
	l.current = nil
	return err
}
