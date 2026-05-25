package config

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	C "github.com/metacubex/mihomo/constant"
	"github.com/metacubex/mihomo/tunnel"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderLocalDialerProxyHiddenProxy(t *testing.T) {
	socksAddr, closeSocksServer := startMinimalSocks5Server(t)
	defer closeSocksServer()

	server, portString, err := net.SplitHostPort(socksAddr)
	require.NoError(t, err)
	port, err := strconv.Atoi(portString)
	require.NoError(t, err)

	cfg := &RawConfig{
		Proxy: []map[string]any{
			{"name": "outer", "type": "reject"},
		},
		ProxyProvider: map[string]map[string]any{
			"dog": {
				"type": "inline",
				"payload": []map[string]any{
					{"name": "outer", "type": "direct", "hidden": true},
					{"name": "inner", "type": "socks5", "server": server, "port": port, "dialer-proxy": "outer"},
				},
			},
		},
		ProxyGroup: []map[string]any{
			{"name": "PROXY", "type": "select", "use": []string{"dog"}},
		},
	}

	proxies, providers, err := parseProxies(cfg)
	require.NoError(t, err)

	oldProxies := tunnel.Proxies()
	oldProviders := tunnel.Providers()
	tunnel.UpdateProxies(proxies, providers)
	defer tunnel.UpdateProxies(oldProxies, oldProviders)

	providerProxies := providers["dog"].Proxies()
	require.Len(t, providerProxies, 1)
	require.Equal(t, "inner", providerProxies[0].Name())

	rawGroup, err := proxies["PROXY"].MarshalJSON()
	require.NoError(t, err)
	groupInfo := struct {
		All []string `json:"all"`
	}{}
	require.NoError(t, json.Unmarshal(rawGroup, &groupInfo))
	assert.Equal(t, []string{"inner"}, groupInfo.All)

	metadata := &C.Metadata{}
	require.NoError(t, metadata.SetRemoteAddress("example.com:80"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, err := providerProxies[0].DialContext(ctx, metadata)
	require.NoError(t, err)
	_ = conn.Close()
}

func startMinimalSocks5Server(t *testing.T) (string, func()) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		_ = handleMinimalSocks5Connect(conn)
	}()

	return ln.Addr().String(), func() {
		_ = ln.Close()
	}
}

func handleMinimalSocks5Connect(conn net.Conn) error {
	defer conn.Close()

	buf := make([]byte, 260)
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return err
	}
	methods := int(buf[1])
	if _, err := io.ReadFull(conn, buf[:methods]); err != nil {
		return err
	}
	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		return err
	}

	if _, err := io.ReadFull(conn, buf[:4]); err != nil {
		return err
	}
	switch buf[3] {
	case 0x01:
		if _, err := io.ReadFull(conn, buf[:4]); err != nil {
			return err
		}
	case 0x03:
		if _, err := io.ReadFull(conn, buf[:1]); err != nil {
			return err
		}
		if _, err := io.ReadFull(conn, buf[:int(buf[0])]); err != nil {
			return err
		}
	case 0x04:
		if _, err := io.ReadFull(conn, buf[:16]); err != nil {
			return err
		}
	default:
		return nil
	}
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return err
	}
	_, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
	return err
}
