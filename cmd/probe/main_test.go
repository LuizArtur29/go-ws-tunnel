package main

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LuizArtur29/go-ws-tunnel/internal/probe"
	"github.com/coder/websocket"
)

func TestTLSWebSocketOverUnixSocket(t *testing.T) {
	dir, err := os.MkdirTemp("", "probe-unix-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "probe.sock")
	listener, err := openListener("", path)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("socket permissions must be 0600", err)
	}
	token := strings.Repeat("a", 64)
	tokenPath := filepath.Join(dir, "token")
	if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := httptest.NewUnstartedServer(probe.Handler(ctx, tokenPath, 1))
	server.Listener.Close()
	server.Listener = listener
	server.StartTLS()
	defer server.Close()
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: server.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs}
	transport.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", path)
	}
	defer transport.CloseIdleConnections()
	deadline, stop := context.WithTimeout(ctx, 3*time.Second)
	defer stop()
	conn, _, err := websocket.Dial(deadline, "wss://example.com/probe", &websocket.DialOptions{HTTPClient: &http.Client{Transport: transport}, HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	if _, err := probe.RoundTrip(deadline, conn); err != nil {
		t.Fatal(err)
	}
}

func TestUnixListenerPreservesExistingPath(t *testing.T) {
	dir, err := os.MkdirTemp("", "probe-unix-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "existing")
	if err := os.WriteFile(path, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	if listener, err := openListener("", path); err == nil {
		listener.Close()
		t.Fatal("replaced existing path")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "preserve" {
		t.Fatal("existing file was modified")
	}
	socket := filepath.Join(dir, "probe.sock")
	listener, err := openListener("", socket)
	if err != nil {
		t.Fatal(err)
	}
	listener.Close()
	if _, err := os.Stat(socket); !os.IsNotExist(err) {
		t.Fatal("socket not removed after close")
	}
}
