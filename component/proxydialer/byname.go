package proxydialer

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	C "github.com/metacubex/mihomo/constant"
)

type Tunnel interface {
	C.Tunnel
	Proxies() map[string]C.Proxy
}

type ProxyResolver = func(name string) (C.Proxy, bool)

type byNameProxyDialer struct {
	proxyName     string
	tunnel        C.Tunnel
	proxyResolver ProxyResolver
}

func (d byNameProxyDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	proxy, err := d.resolveProxy()
	if err != nil {
		return nil, err
	}
	return New(proxy, true).DialContext(ctx, network, address)
}

func (d byNameProxyDialer) ListenPacket(ctx context.Context, network, address string, rAddrPort netip.AddrPort) (net.PacketConn, error) {
	proxy, err := d.resolveProxy()
	if err != nil {
		return nil, err
	}
	return New(proxy, true).ListenPacket(ctx, network, address, rAddrPort)
}

func (d byNameProxyDialer) resolveProxy() (C.Proxy, error) {
	if d.proxyResolver != nil {
		if proxy, ok := d.proxyResolver(d.proxyName); ok && proxy != nil {
			return proxy, nil
		}
	}

	tunnel, _ := d.tunnel.(Tunnel)
	if tunnel == nil {
		return nil, fmt.Errorf("tunnel is invalid, must be proxydialer.Tunnel, but got: %T", d.tunnel)
	}
	proxies := tunnel.Proxies()
	proxy, ok := proxies[d.proxyName]
	if !ok {
		return nil, fmt.Errorf("proxyName[%s] not found", d.proxyName)
	}
	return proxy, nil
}

func NewByName(proxyName string, tunnel C.Tunnel, proxyResolver ...ProxyResolver) C.Dialer {
	var resolver ProxyResolver
	if len(proxyResolver) > 0 {
		resolver = proxyResolver[0]
	}
	return byNameProxyDialer{proxyName: proxyName, tunnel: tunnel, proxyResolver: resolver}
}
