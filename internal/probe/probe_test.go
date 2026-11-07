package probe

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func setup(t *testing.T) (*httptest.Server, string, string) {
	t.Helper()
	token := strings.Repeat("a", 64)
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(token), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	server := httptest.NewTLSServer(Handler(ctx, path, 1))
	t.Cleanup(func() { cancel(); server.Close() })
	return server, token, path
}

func dial(t *testing.T, s *httptest.Server, token, path string, trusted bool, extra http.Header) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	client := s.Client()
	if !trusted {
		client = &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}}
	}
	headers := http.Header{"Authorization": []string{"Bearer " + token}}
	for key, values := range extra {
		headers[key] = values
	}
	return websocket.Dial(ctx, "wss"+strings.TrimPrefix(s.URL, "https")+path, &websocket.DialOptions{HTTPClient: client, HTTPHeader: headers})
}

func TestTLSRoundTrip(t *testing.T) {
	s, token, _ := setup(t)
	conn, _, err := dial(t, s, token, "/probe", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for range 3 {
		if _, err := RoundTrip(ctx, conn); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAccessControl(t *testing.T) {
	for _, tc := range []struct {
		name, token, path string
		headers           http.Header
		status            int
	}{
		{"wrong token", "wrong", "/probe", nil, 401},
		{"browser origin", strings.Repeat("a", 64), "/probe", http.Header{"Origin": []string{"https://example.com"}}, 401},
		{"duplicate auth", "unused", "/probe", http.Header{"Authorization": []string{"Bearer x", "Bearer y"}}, 401},
		{"unknown route", strings.Repeat("a", 64), "/ssh", nil, 404},
		{"query", strings.Repeat("a", 64), "/probe?target=other", nil, 404},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _ := setup(t)
			conn, response, err := dial(t, s, tc.token, tc.path, true, tc.headers)
			if conn != nil {
				conn.CloseNow()
			}
			if err == nil || response == nil || response.StatusCode != tc.status {
				t.Fatalf("wanted HTTP %d; response=%v err=%v", tc.status, response, err)
			}
		})
	}
}

func TestUntrustedTLS(t *testing.T) {
	s, token, _ := setup(t)
	conn, _, err := dial(t, s, token, "/probe", false, nil)
	if conn != nil {
		conn.CloseNow()
	}
	if err == nil {
		t.Fatal("untrusted certificate accepted")
	}
}

func TestRotation(t *testing.T) {
	s, old, path := setup(t)
	next := strings.Repeat("b", 64)
	if err := os.WriteFile(path, []byte(next), 0600); err != nil {
		t.Fatal(err)
	}
	conn, response, err := dial(t, s, old, "/probe", true, nil)
	if conn != nil {
		conn.CloseNow()
	}
	if err == nil || response == nil || response.StatusCode != 401 {
		t.Fatal("old credential accepted")
	}
	conn, _, err = dial(t, s, next, "/probe", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	conn.CloseNow()
}

func TestProtocolLimits(t *testing.T) {
	for _, tc := range []struct {
		name   string
		kind   websocket.MessageType
		data   []byte
		status websocket.StatusCode
	}{
		{"text", websocket.MessageText, []byte("hello"), websocket.StatusUnsupportedData},
		{"oversize", websocket.MessageBinary, make([]byte, MaxMessage+1), websocket.StatusMessageTooBig},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, token, _ := setup(t)
			conn, _, err := dial(t, s, token, "/probe", true, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.CloseNow()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := conn.Write(ctx, tc.kind, tc.data); err != nil {
				t.Fatal(err)
			}
			_, _, err = conn.Read(ctx)
			if websocket.CloseStatus(err) != tc.status {
				t.Fatalf("wanted %v got %v", tc.status, err)
			}
		})
	}
}

func TestSessionLimit(t *testing.T) {
	s, token, _ := setup(t)
	first, _, err := dial(t, s, token, "/probe", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.CloseNow()
	second, response, err := dial(t, s, token, "/probe", true, nil)
	if second != nil {
		second.CloseNow()
	}
	if err == nil || response == nil || response.StatusCode != 503 {
		t.Fatal("connection limit ignored")
	}
}

func TestCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	token := strings.Repeat("c", 64)
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(token), 0600); err != nil {
		t.Fatal(err)
	}
	s := httptest.NewTLSServer(Handler(ctx, path, 1))
	defer s.Close()
	defer cancel()
	conn, _, err := dial(t, s, token, "/probe", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.CloseNow()
	cancel()
	deadline, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if _, _, err := conn.Read(deadline); err == nil {
		t.Fatal("connection survived server cancellation")
	}
}

func TestURLValidation(t *testing.T) {
	for _, raw := range []string{"ws://example.com/probe", "wss://a:b@example.com/probe", "wss://example.com/probe?token=x", "wss://example.com/probe#x", "wss://example.com/ssh"} {
		if ValidateURL(raw) == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	if err := ValidateURL("wss://localhost:8443/probe"); err != nil {
		t.Fatal(err)
	}
}
