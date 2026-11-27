package share

import (
	"errors"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"sync"
)

func localLANAddress(bindIP string) (string, error) {
	addresses, err := localLANAddresses()
	if err != nil {
		return "", err
	}
	if bindIP != "" {
		wanted, _ := netip.ParseAddr(bindIP)
		for _, address := range addresses {
			if wanted.Unmap() == address.Unmap() {
				return address.String(), nil
			}
		}
		return "", errors.New("分享监听地址不属于本机活动网络接口")
	}
	slices.SortFunc(addresses, func(a, b netip.Addr) int {
		if a.Is4() != b.Is4() {
			if a.Is4() {
				return -1
			}
			return 1
		}
		return a.Compare(b)
	})
	for _, address := range addresses {
		if !address.IsLoopback() && address.IsGlobalUnicast() {
			return address.String(), nil
		}
	}
	return "", errors.New("未找到可用于分享的本机局域网地址")
}

func localLANAddresses() ([]netip.Addr, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, errors.New("无法读取本机网络接口")
	}
	var addresses []netip.Addr
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		values, err := iface.Addrs()
		if err != nil {
			return nil, errors.New("无法读取本机网络接口地址")
		}
		for _, value := range values {
			prefix, err := netip.ParsePrefix(value.String())
			if err != nil {
				continue
			}
			address := prefix.Addr().Unmap()
			if address.Is6() && address.IsLinkLocalUnicast() {
				address = address.WithZone(strconv.Itoa(iface.Index))
			}
			addresses = append(addresses, address)
		}
	}
	return addresses, nil
}

type lanListener struct {
	net.Listener
	slots chan struct{}
}

func (l *lanListener) Accept() (net.Conn, error) {
	for {
		connection, err := l.Listener.Accept()
		if err != nil {
			return nil, err
		}
		select {
		case l.slots <- struct{}{}:
			return &lanConnection{Conn: connection, slots: l.slots}, nil
		default:
			connection.Close()
		}
	}
}

type lanConnection struct {
	net.Conn
	slots chan struct{}
	once  sync.Once
	err   error
}

func (c *lanConnection) Close() error {
	c.once.Do(func() {
		c.err = c.Conn.Close()
		<-c.slots
	})
	return c.err
}
