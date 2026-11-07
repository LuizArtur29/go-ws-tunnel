package tunnel

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func targetServer(t *testing.T, handler func(*net.TCPConn)) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			workers.Add(1)
			go func() {
				defer workers.Done()
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
				handler(conn.(*net.TCPConn))
			}()
		}
	}()
	t.Cleanup(func() { listener.Close(); <-done; workers.Wait() })
	return listener.Addr().String()
}

func pair(t *testing.T) (*net.TCPConn, *net.TCPConn) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	first, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	second, err := listener.Accept()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { first.Close(); second.Close() })
	_ = first.SetDeadline(time.Now().Add(10 * time.Second))
	_ = second.SetDeadline(time.Now().Add(10 * time.Second))
	return first.(*net.TCPConn), second.(*net.TCPConn)
}

func gateway(t *testing.T, target string, limit int) (*httptest.Server, string, context.Context) {
	t.Helper()
	tokenPath := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(tokenPath, []byte(strings.Repeat("a", 64)), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	handler, err := Handler(ctx, tokenPath, target, limit)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(handler)
	t.Cleanup(func() { cancel(); server.Close() })
	return server, tokenPath, ctx
}

func startRelay(t *testing.T, s *httptest.Server, token string, ctx context.Context) (*net.TCPConn, <-chan error) {
	t.Helper()
	ws, err := Dial(ctx, s.Client(), "wss"+strings.TrimPrefix(s.URL, "https")+"/ssh", token)
	if err != nil {
		t.Fatal(err)
	}
	client, local := pair(t)
	done := make(chan error, 1)
	go func() { done <- Relay(ctx, ws, TCP(local)) }()
	return client, done
}

func await(t *testing.T, done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("relay did not terminate")
	}
}

func TestLargeTransferAndHalfClose(t *testing.T) {
	data := bytes.Repeat([]byte{0, 1, 2, 255, 128, 9, 13, 10}, 512*1024)
	target := targetServer(t, func(conn *net.TCPConn) {
		input, err := io.ReadAll(conn)
		if err != nil {
			return
		}
		_, _ = conn.Write(append([]byte("reply:"), input...))
	})
	s, token, ctx := gateway(t, target, 8)
	client, done := startRelay(t, s, token, ctx)
	if _, err := client.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := client.CloseWrite(); err != nil {
		t.Fatal(err)
	}
	reply, err := io.ReadAll(client)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(reply, append([]byte("reply:"), data...)) {
		t.Fatal("binary data corrupted or truncated")
	}
	await(t, done)
}

func TestReverseHalfClose(t *testing.T) {
	received := make(chan string, 1)
	target := targetServer(t, func(conn *net.TCPConn) { _ = conn.CloseWrite(); data, _ := io.ReadAll(conn); received <- string(data) })
	s, token, ctx := gateway(t, target, 8)
	client, done := startRelay(t, s, token, ctx)
	if data, err := io.ReadAll(client); err != nil || len(data) != 0 {
		t.Fatal("expected initial receive EOF", err)
	}
	if _, err := client.Write([]byte("still writable")); err != nil {
		t.Fatal(err)
	}
	_ = client.CloseWrite()
	await(t, done)
	select {
	case data := <-received:
		if data != "still writable" {
			t.Fatal(data)
		}
	case <-time.After(time.Second):
		t.Fatal("lost data after peer EOF")
	}
}

func TestConcurrentSessions(t *testing.T) {
	target := targetServer(t, func(conn *net.TCPConn) { _, _ = io.Copy(conn, conn) })
	s, token, ctx := gateway(t, target, 8)
	var workers sync.WaitGroup
	for i := 0; i < 4; i++ {
		client, done := startRelay(t, s, token, ctx)
		workers.Add(1)
		go func() {
			defer workers.Done()
			payload := bytes.Repeat([]byte("parallel"), 8192)
			if _, err := client.Write(payload); err != nil {
				t.Error(err)
				return
			}
			_ = client.CloseWrite()
			reply, err := io.ReadAll(client)
			if err != nil || !bytes.Equal(reply, payload) {
				t.Error("concurrent stream corrupted", err)
			}
			await(t, done)
		}()
	}
	workers.Wait()
}

func TestCancellationClosesBothDirections(t *testing.T) {
	target := targetServer(t, func(conn *net.TCPConn) { _, _ = io.Copy(io.Discard, conn) })
	s, token, parent := gateway(t, target, 8)
	ctx, cancel := context.WithCancel(parent)
	client, done := startRelay(t, s, token, ctx)
	cancel()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected cancellation")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation leaked workers")
	}
	if _, err := client.Read(make([]byte, 1)); err == nil {
		t.Fatal("local stream stayed open")
	}
}

func TestAuthProtocolAndLimits(t *testing.T) {
	var hits atomic.Int32
	accepted := make(chan struct{}, 8)
	target := targetServer(t, func(conn *net.TCPConn) { hits.Add(1); accepted <- struct{}{}; _, _ = io.Copy(io.Discard, conn) })
	s, token, ctx := gateway(t, target, 1)
	url := "wss" + strings.TrimPrefix(s.URL, "https") + "/ssh"
	for _, tc := range []struct {
		path, auth string
		protocols  []string
		status     int
	}{
		{"/ssh", "wrong", []string{Protocol}, 401},
		{"/ssh", strings.Repeat("a", 64), nil, 400},
		{"/ssh?target=other", strings.Repeat("a", 64), []string{Protocol}, 404},
	} {
		ws, response, err := websocket.Dial(ctx, "wss"+strings.TrimPrefix(s.URL, "https")+tc.path, &websocket.DialOptions{HTTPClient: s.Client(), HTTPHeader: http.Header{"Authorization": []string{"Bearer " + tc.auth}}, Subprotocols: tc.protocols})
		if ws != nil {
			ws.CloseNow()
		}
		if err == nil || response == nil || response.StatusCode != tc.status {
			t.Fatalf("wanted %d got %v %v", tc.status, response, err)
		}
	}
	if hits.Load() != 0 {
		t.Fatal("unauthorized request touched TCP target")
	}
	first, err := Dial(ctx, s.Client(), url, token)
	if err != nil {
		t.Fatal(err)
	}
	defer first.CloseNow()
	select {
	case <-accepted:
	case <-time.After(time.Second):
		t.Fatal("target not opened")
	}
	second, response, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPClient: s.Client(), HTTPHeader: http.Header{"Authorization": []string{"Bearer " + strings.Repeat("a", 64)}}, Subprotocols: []string{Protocol}})
	if second != nil {
		second.CloseNow()
	}
	if err == nil || response == nil || response.StatusCode != 503 {
		t.Fatal("session limit ignored")
	}
}

func TestMalformedFrameClosesTarget(t *testing.T) {
	closed := make(chan struct{}, 1)
	target := targetServer(t, func(conn *net.TCPConn) { _, _ = io.Copy(io.Discard, conn); closed <- struct{}{} })
	s, token, ctx := gateway(t, target, 8)
	ws, err := Dial(ctx, s.Client(), "wss"+strings.TrimPrefix(s.URL, "https")+"/ssh", token)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	op, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := ws.Write(op, websocket.MessageBinary, []byte{99}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ws.Read(op); err == nil {
		t.Fatal("malformed frame accepted")
	}
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("TCP target leaked")
	}
}

func TestTCPProxyHalfClose(t *testing.T) {
	target := targetServer(t, func(conn *net.TCPConn) {
		data, _ := io.ReadAll(conn)
		_, _ = conn.Write(append([]byte("reply:"), data...))
	})
	client, local := pair(t)
	done := make(chan error, 1)
	go func() { done <- ProxyTCP(context.Background(), local, target) }()
	_, _ = client.Write([]byte("tcp lab"))
	_ = client.CloseWrite()
	reply, err := io.ReadAll(client)
	if err != nil || string(reply) != "reply:tcp lab" {
		t.Fatal(string(reply), err)
	}
	await(t, done)
}

func TestConfiguration(t *testing.T) {
	for _, target := range []string{"0.0.0.0:22", "example.com:22", "localhost:22", "192.0.2.1:22", "127.0.0.1:0"} {
		if ValidateTarget(target) == nil {
			t.Fatal("accepted target", target)
		}
	}
	for _, target := range []string{"127.0.0.1:22", "[::1]:22"} {
		if err := ValidateTarget(target); err != nil {
			t.Fatal(err)
		}
	}
	for _, url := range []string{"ws://localhost/ssh", "wss://user@host/ssh", "wss://host/ssh?x=y", "wss://host/probe"} {
		if ValidateURL(url) == nil {
			t.Fatal("accepted URL", url)
		}
	}
}
