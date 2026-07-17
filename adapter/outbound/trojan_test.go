package outbound

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/metacubex/mihomo/common/structure"
	C "github.com/metacubex/mihomo/constant"
)

func TestTrojanOptionDecodeSkipTLS(t *testing.T) {
	decoder := structure.NewDecoder(structure.Option{
		TagName:          "proxy",
		WeaklyTypedInput: true,
		KeyReplacer:      structure.DefaultKeyReplacer,
	})
	option := &TrojanOption{}
	if err := decoder.Decode(map[string]any{
		"name":         "plain-trojan",
		"type":         "trojan",
		"server":       "127.0.0.1",
		"port":         18443,
		"password":     "password",
		"skip-tls":     true,
		"dialer-proxy": "outer-ss",
	}, option); err != nil {
		t.Fatalf("decode TrojanOption: %v", err)
	}
	if !option.SkipTLS {
		t.Fatal("skip-tls was not decoded into TrojanOption.SkipTLS")
	}
	if option.DialerProxy != "outer-ss" {
		t.Fatalf("dialer-proxy was not decoded with skip-tls: %q", option.DialerProxy)
	}
}

func TestTrojanStreamConnContextSkipTLSWritesPlainHeader(t *testing.T) {
	proxy, err := NewTrojan(TrojanOption{
		Name:     "plain-trojan",
		Server:   "127.0.0.1",
		Port:     18443,
		Password: "password",
		SkipTLS:  true,
	})
	if err != nil {
		t.Fatalf("NewTrojan: %v", err)
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	deadline := time.Now().Add(time.Second)
	_ = clientConn.SetDeadline(deadline)
	_ = serverConn.SetDeadline(deadline)

	errCh := make(chan error, 1)
	go func() {
		conn, err := proxy.StreamConnContext(context.Background(), clientConn, testTrojanMetadata())
		if conn != nil {
			_ = conn.Close()
		}
		errCh <- err
	}()

	want := testTrojanHeader("password")
	got := make([]byte, len(want))
	if _, err := io.ReadFull(serverConn, got); err != nil {
		t.Fatalf("read plain Trojan header: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("plain Trojan header mismatch:\n got %x\nwant %x", got, want)
	}
	if err := <-errCh; err != nil {
		t.Fatalf("StreamConnContext returned error: %v", err)
	}
}

func TestTrojanStreamConnContextDefaultsToTLS(t *testing.T) {
	proxy, err := NewTrojan(TrojanOption{
		Name:           "tls-trojan",
		Server:         "127.0.0.1",
		Port:           18443,
		Password:       "password",
		SkipCertVerify: true,
	})
	if err != nil {
		t.Fatalf("NewTrojan: %v", err)
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	deadline := time.Now().Add(time.Second)
	_ = clientConn.SetDeadline(deadline)
	_ = serverConn.SetDeadline(deadline)

	errCh := make(chan error, 1)
	go func() {
		conn, err := proxy.StreamConnContext(context.Background(), clientConn, testTrojanMetadata())
		if conn != nil {
			_ = conn.Close()
		}
		errCh <- err
	}()

	header := make([]byte, 5)
	if _, err := io.ReadFull(serverConn, header); err != nil {
		t.Fatalf("read TLS record header: %v", err)
	}
	if bytes.Equal(header, testTrojanHeader("password")[:len(header)]) {
		t.Fatalf("default Trojan path wrote plain Trojan header prefix: %x", header)
	}
	if header[0] != 0x16 {
		t.Fatalf("default Trojan path did not start a TLS handshake, first bytes: %x", header)
	}
	_ = serverConn.Close()
	_ = clientConn.Close()
	<-errCh
}

func TestTrojanRejectsJLSWithSkipTLS(t *testing.T) {
	_, err := NewTrojan(TrojanOption{
		Name:     "jls-skip-tls",
		Server:   "127.0.0.1",
		Port:     18443,
		Password: "password",
		SkipTLS:  true,
		JLSOpts: JLSOptions{
			Username: "user",
			Password: "pass",
		},
	})
	if err == nil {
		t.Fatal("NewTrojan expected to reject JLS with skip-tls, got nil error")
	}
	if !strings.Contains(err.Error(), "JLS requires TLS") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTrojanRejectsSkipTLSWithWSNetwork(t *testing.T) {
	_, err := NewTrojan(TrojanOption{
		Name:     "skip-tls-ws",
		Server:   "127.0.0.1",
		Port:     18443,
		Password: "password",
		SkipTLS:  true,
		Network:  "ws",
	})
	if err == nil || !strings.Contains(err.Error(), "skip-tls is only supported") {
		t.Fatalf("expected skip-tls/network conflict error, got: %v", err)
	}
}

func TestTrojanRejectsECHWithSkipTLS(t *testing.T) {
	_, err := NewTrojan(TrojanOption{
		Name:     "skip-tls-ech",
		Server:   "127.0.0.1",
		Port:     18443,
		Password: "password",
		SkipTLS:  true,
		ECHOpts:  ECHOptions{Enable: true},
	})
	if err == nil || !strings.Contains(err.Error(), "skip-tls is incompatible with ECH") {
		t.Fatalf("expected skip-tls/ECH conflict error, got: %v", err)
	}
}

func TestTrojanRejectsRealityWithSkipTLS(t *testing.T) {
	_, err := NewTrojan(TrojanOption{
		Name:        "skip-tls-reality",
		Server:      "127.0.0.1",
		Port:        18443,
		Password:    "password",
		SkipTLS:     true,
		RealityOpts: RealityOptions{PublicKey: strings.Repeat("A", 43)},
	})
	if err == nil || !strings.Contains(err.Error(), "skip-tls is incompatible with REALITY") {
		t.Fatalf("expected skip-tls/REALITY conflict error, got: %v", err)
	}
}

func testTrojanMetadata() *C.Metadata {
	return &C.Metadata{
		NetWork: C.TCP,
		Host:    "example.com",
		DstPort: 443,
	}
}

func testTrojanHeader(password string) []byte {
	sum := sha256.Sum224([]byte(password))
	header := make([]byte, 0, 76)
	header = append(header, []byte(hex.EncodeToString(sum[:]))...)
	header = append(header, '\r', '\n')
	header = append(header, 0x01)
	header = append(header, 0x03, byte(len("example.com")))
	header = append(header, []byte("example.com")...)
	header = append(header, 0x01, 0xbb)
	header = append(header, '\r', '\n')
	return header
}
