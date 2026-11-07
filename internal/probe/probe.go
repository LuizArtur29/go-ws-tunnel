// Package probe validates an authenticated binary WebSocket connection over TLS.
// It deliberately doesn't open connections to SSH or other TCP destinations.
package probe

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"
)

const MaxMessage = 4096

func ReadToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", errors.New("não foi possível ler o arquivo de token")
	}
	token := strings.TrimSpace(string(data))
	if len(token) < 32 || len(token) > 256 {
		return "", errors.New("token deve ter entre 32 e 256 caracteres ASCII sem espaços")
	}
	for _, c := range token {
		if c < 33 || c > 126 {
			return "", errors.New("token contém caracteres inválidos")
		}
	}
	return token, nil
}

func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("URL inválida")
	}
	if u.Scheme != "wss" || u.Hostname() == "" || u.User != nil || u.Path != "/probe" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return errors.New("use wss://DOMINIO[:PORTA]/probe, sem credenciais, query ou fragmento")
	}
	return nil
}

// Handler only echoes bounded binary messages. The token is re-read on each
// handshake, so replacing the file revokes access for new connections.
func Handler(ctx context.Context, tokenFile string, maxSessions int) http.Handler {
	if maxSessions < 1 {
		panic("maxSessions must be positive")
	}
	slots := make(chan struct{}, maxSessions)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/probe" || r.URL.RawQuery != "" || r.URL.ForceQuery {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		token, err := ReadToken(tokenFile)
		if err != nil {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		auth := r.Header.Values("Authorization")
		if len(auth) != 1 || len(r.Header.Values("Origin")) != 0 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		got := sha256.Sum256([]byte(auth[0]))
		want := sha256.Sum256([]byte("Bearer " + token))
		if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
		if err != nil {
			return
		}
		defer conn.CloseNow()
		conn.SetReadLimit(MaxMessage)
		for {
			op, cancel := context.WithTimeout(ctx, 2*time.Minute)
			kind, data, err := conn.Read(op)
			if err == nil && kind != websocket.MessageBinary {
				cancel()
				_ = conn.Close(websocket.StatusUnsupportedData, "binary messages required")
				return
			}
			if err == nil {
				err = conn.Write(op, websocket.MessageBinary, data)
			}
			cancel()
			if err != nil {
				return
			}
		}
	})
}

// RoundTrip checks the payload itself, not just whether a handshake succeeded.
func RoundTrip(ctx context.Context, conn *websocket.Conn) (time.Duration, error) {
	payload := make([]byte, 64)
	if _, err := rand.Read(payload); err != nil {
		return 0, err
	}
	start := time.Now()
	if err := conn.Write(ctx, websocket.MessageBinary, payload); err != nil {
		return 0, fmt.Errorf("write: %w", err)
	}
	kind, reply, err := conn.Read(ctx)
	if err != nil {
		return 0, fmt.Errorf("read: %w", err)
	}
	if kind != websocket.MessageBinary || !bytes.Equal(payload, reply) {
		return 0, errors.New("resposta diferente dos dados enviados")
	}
	return time.Since(start), nil
}
