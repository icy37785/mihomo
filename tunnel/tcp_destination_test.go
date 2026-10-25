package tunnel_test

import (
	"fmt"
	"io"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/adapter/outbound"
	C "github.com/metacubex/mihomo/constant"
	RC "github.com/metacubex/mihomo/rules/common"
	"github.com/metacubex/mihomo/transport/socks5"
	"github.com/metacubex/mihomo/tunnel"
)

// Exercise the real TCP tunnel and SOCKS wire destination. A DNS-mapped host
// may select a domain rule, but must not replace the client's real destination.
func TestTCPPreservesDNSMappedDestination(t *testing.T) {
	oldStatus := tunnel.Status()
	oldMode, oldRules, oldProxies := tunnel.Mode(), tunnel.Rules(), tunnel.Proxies()
	oldProviders, oldRuleProviders := tunnel.Providers(), tunnel.RuleProviders()
	t.Cleanup(func() {
		tunnel.OnSuspend()
		tunnel.SetMode(oldMode)
		tunnel.UpdateRules(oldRules, nil, oldRuleProviders)
		tunnel.UpdateProxies(oldProxies, oldProviders)
		switch oldStatus {
		case tunnel.Inner:
			tunnel.OnInnerLoading()
		case tunnel.Running:
			tunnel.OnRunning()
		}
	})
	tunnel.OnRunning()
	tunnel.SetMode(tunnel.Rule)

	for _, tc := range []struct {
		name    string
		dnsMode C.DNSMode
		ip      string
		want    string
	}{
		{"mapped_ipv4", C.DNSMapping, "203.0.113.7", "203.0.113.7:443"},
		{"mapped_ipv6", C.DNSMapping, "2001:db8::7", "[2001:db8::7]:443"},
		{"hosts_ipv4", C.DNSHosts, "203.0.113.7", "203.0.113.7:443"},
		{"hosts_ipv6", C.DNSHosts, "2001:db8::7", "[2001:db8::7]:443"},
		{"mapped_without_ip", C.DNSMapping, "", "api.example.test:443"},
		{"explicit_domain", C.DNSNormal, "203.0.113.7", "api.example.test:443"},
		{"fake_ip_domain", C.DNSFakeIP, "198.18.0.7", "api.example.test:443"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			if err := listener.(*net.TCPListener).SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			serverResult := make(chan error, 1)
			go func() {
				conn, err := listener.Accept()
				if err != nil {
					serverResult <- err
					return
				}
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
				target, command, _, err := socks5.ServerHandshake(conn, nil)
				if err != nil {
					serverResult <- err
					return
				}
				if command != socks5.CmdConnect || target.String() != tc.want {
					_, _ = io.WriteString(conn, "wrong-destination")
					serverResult <- fmt.Errorf("SOCKS destination %s (command %d), want %s", target, command, tc.want)
					return
				}
				payload := make([]byte, len("ping"))
				if _, err = io.ReadFull(conn, payload); err == nil {
					_, err = conn.Write(payload)
				}
				serverResult <- err
			}()

			proxy, err := outbound.NewSocks5(outbound.Socks5Option{
				Name: "test-egress", Server: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port,
			})
			if err != nil {
				t.Fatal(err)
			}
			defer proxy.Close()
			tunnel.UpdateProxies(map[string]C.Proxy{"test-egress": adapter.NewProxy(proxy)}, nil)
			tunnel.UpdateRules([]C.Rule{RC.NewDomain("api.example.test", "test-egress")}, nil, nil)
			metadata := &C.Metadata{
				NetWork: C.TCP, Type: C.INNER, DNSMode: tc.dnsMode,
				Host: "api.example.test", DstPort: 443,
			}
			if tc.ip != "" {
				metadata.DstIP = netip.MustParseAddr(tc.ip)
			}
			client, inbound := net.Pipe()
			defer client.Close()
			_ = client.SetDeadline(time.Now().Add(5 * time.Second))
			tunnelDone := make(chan struct{})
			go func() {
				tunnel.Tunnel.HandleTCPConn(inbound, metadata)
				close(tunnelDone)
			}()
			if _, err := io.WriteString(client, "ping"); err != nil {
				t.Error(err)
			}
			reply, err := io.ReadAll(client)
			if err != nil || string(reply) != "ping" {
				t.Errorf("client reply = %q, error = %v; want ping", reply, err)
			}
			client.Close()
			select {
			case <-tunnelDone:
			case <-time.After(5 * time.Second):
				t.Fatal("TCP tunnel did not finish")
			}
			if err := <-serverResult; err != nil {
				t.Error(err)
			}
			if metadata.Host != "api.example.test" {
				t.Errorf("routing metadata host = %q; want original host", metadata.Host)
			}
		})
	}
}
