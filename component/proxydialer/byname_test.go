package proxydialer

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"

	"github.com/metacubex/mihomo/common/utils"
	C "github.com/metacubex/mihomo/constant"

	"github.com/stretchr/testify/assert"
)

type testTunnel struct {
	proxies map[string]C.Proxy
}

func (t testTunnel) HandleTCPConn(net.Conn, *C.Metadata) {}

func (t testTunnel) HandleUDPPacket(C.UDPPacket, *C.Metadata) {}

func (t testTunnel) NatTable() C.NatTable { return nil }

func (t testTunnel) Proxies() map[string]C.Proxy { return t.proxies }

type testProxy struct {
	name string
	err  error
}

func (p testProxy) Name() string { return p.name }

func (p testProxy) Type() C.AdapterType { return C.Direct }

func (p testProxy) Addr() string { return "" }

func (p testProxy) SupportUDP() bool { return false }

func (p testProxy) ProxyInfo() C.ProxyInfo { return C.ProxyInfo{} }

func (p testProxy) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{"name": p.name})
}

func (p testProxy) DialContext(context.Context, *C.Metadata) (C.Conn, error) {
	return nil, p.err
}

func (p testProxy) ListenPacketContext(context.Context, *C.Metadata) (C.PacketConn, error) {
	return nil, p.err
}

func (p testProxy) SupportUOT() bool { return false }

func (p testProxy) IsL3Protocol(*C.Metadata) bool { return false }

func (p testProxy) Unwrap(*C.Metadata, bool) C.Proxy { return nil }

func (p testProxy) Close() error { return nil }

func (p testProxy) Adapter() C.ProxyAdapter { return p }

func (p testProxy) AliveForTestUrl(string) bool { return false }

func (p testProxy) DelayHistory() []C.DelayHistory { return nil }

func (p testProxy) DelayHistoryForTestUrl(string) []C.DelayHistory { return nil }

func (p testProxy) ExtraDelayHistories() map[string]C.ProxyState { return nil }

func (p testProxy) LastDelayForTestUrl(string) uint16 { return 0 }

func (p testProxy) URLTest(context.Context, string, utils.IntRanges[uint16]) (uint16, error) {
	return 0, p.err
}

func (p testProxy) StatusTest(context.Context, string) (uint16, bool, error) {
	return 0, false, p.err
}

func TestByNameProxyDialerPrefersLocalResolver(t *testing.T) {
	globalProxy := testProxy{name: "target", err: errors.New("global proxy used")}
	localProxy := testProxy{name: "target", err: errors.New("local proxy used")}
	dialer := NewByName("target", testTunnel{proxies: map[string]C.Proxy{
		"target": globalProxy,
	}}, func(name string) (C.Proxy, bool) {
		if name == "target" {
			return localProxy, true
		}
		return nil, false
	})

	_, err := dialer.DialContext(context.Background(), "tcp", "example.com:443")

	assert.ErrorContains(t, err, "local proxy used")
}

func TestByNameProxyDialerFallsBackToGlobalTunnel(t *testing.T) {
	globalProxy := testProxy{name: "target", err: errors.New("global proxy used")}
	dialer := NewByName("target", testTunnel{proxies: map[string]C.Proxy{
		"target": globalProxy,
	}}, func(string) (C.Proxy, bool) {
		return nil, false
	})

	_, err := dialer.DialContext(context.Background(), "tcp", "example.com:443")

	assert.ErrorContains(t, err, "global proxy used")
}
