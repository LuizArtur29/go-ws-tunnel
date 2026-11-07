// Package tunnel transports a TCP byte stream in bounded binary WebSocket
// messages. Explicit EOF frames preserve TCP half-close in both directions.
package tunnel

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LuizArtur29/go-ws-tunnel/internal/probe"
	"github.com/coder/websocket"
)

const Protocol = "go-ws-tunnel.v1"
const ChunkSize = 32 * 1024
const frameData byte = 0
const frameEOF byte = 1

func ValidateTarget(target string) error {
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return errors.New("destino deve ser IP loopback literal e porta")
	}
	ip := net.ParseIP(host)
	n, err := strconv.Atoi(port)
	if ip == nil || !ip.IsLoopback() || err != nil || n < 1 || n > 65535 {
		return errors.New("destino deve ser IP loopback literal e porta válida")
	}
	return nil
}

func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("URL inválida")
	}
	if u.Scheme != "wss" || u.Hostname() == "" || u.User != nil || u.Path != "/ssh" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return errors.New("use wss://DOMINIO[:PORTA]/ssh, sem credenciais, query ou fragmento")
	}
	return nil
}

type Endpoint struct {
	Reader     io.Reader
	Writer     io.Writer
	CloseWrite func() error
	Close      func() error
}

func TCP(conn net.Conn) Endpoint {
	return Endpoint{Reader: conn, Writer: conn, CloseWrite: func() error {
		if half, ok := conn.(interface{ CloseWrite() error }); ok {
			return half.CloseWrite()
		}
		return errors.New("conexão sem suporte a half-close")
	}, Close: conn.Close}
}

// Relay owns endpoint and ws. Cancellation closes both and joins all workers.
// No reconnection: silently restarting a TCP stream would corrupt SSH state.
func Relay(parent context.Context, ws *websocket.Conn, endpoint Endpoint) error {
	ctx, cancel := context.WithCancel(parent)
	ws.SetReadLimit(ChunkSize + 1)
	type event struct {
		side string
		err  error
	}
	events := make(chan event, 4)
	var workers sync.WaitGroup
	launch := func(fn func()) { workers.Add(1); go func() { defer workers.Done(); fn() }() }
	defer func() { cancel(); ws.CloseNow(); _ = endpoint.Close(); workers.Wait() }()
	launch(func() {
		buf := make([]byte, ChunkSize+1)
		buf[0] = frameData
		for {
			n, err := endpoint.Reader.Read(buf[1:])
			if n > 0 {
				op, stop := context.WithTimeout(ctx, 45*time.Second)
				writeErr := ws.Write(op, websocket.MessageBinary, buf[:n+1])
				stop()
				if writeErr != nil {
					events <- event{err: writeErr}
					return
				}
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					events <- event{err: err}
					return
				}
				op, stop := context.WithTimeout(ctx, 45*time.Second)
				writeErr := ws.Write(op, websocket.MessageBinary, []byte{frameEOF})
				stop()
				events <- event{side: "out", err: writeErr}
				return
			}
		}
	})
	launch(func() {
		eof := false
		for {
			kind, data, err := ws.Read(ctx)
			if err != nil {
				if eof && websocket.CloseStatus(err) == websocket.StatusNormalClosure {
					return
				}
				events <- event{err: err}
				return
			}
			if kind != websocket.MessageBinary || len(data) == 0 || eof {
				events <- event{err: errors.New("frame inválido ou dados após EOF")}
				return
			}
			switch data[0] {
			case frameData:
				if len(data) == 1 {
					events <- event{err: errors.New("frame de dados vazio")}
					return
				}
				if _, err := io.Copy(endpoint.Writer, bytes.NewReader(data[1:])); err != nil {
					events <- event{err: err}
					return
				}
			case frameEOF:
				if len(data) != 1 {
					events <- event{err: errors.New("EOF inválido")}
					return
				}
				if err := endpoint.CloseWrite(); err != nil {
					events <- event{err: err}
					return
				}
				eof = true
				events <- event{side: "in"}
				// Keep reading control frames so ping/pong continues after half-close.
			default:
				events <- event{err: errors.New("tipo de frame desconhecido")}
				return
			}
		}
	})
	launch(func() {
		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				op, stop := context.WithTimeout(ctx, 10*time.Second)
				err := ws.Ping(op)
				stop()
				if err != nil {
					events <- event{err: err}
					return
				}
			}
		}
	})
	incoming, outgoing := false, false
	for !(incoming && outgoing) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case result := <-events:
			if result.err != nil {
				return result.err
			}
			if result.side == "in" {
				incoming = true
			}
			if result.side == "out" {
				outgoing = true
			}
		}
	}
	// A normal close handshake avoids racing a peer's final EOF notification.
	_ = ws.Close(websocket.StatusNormalClosure, "stream complete")
	return nil
}

func Handler(ctx context.Context, tokenFile, target string, maxSessions int) (http.Handler, error) {
	if err := ValidateTarget(target); err != nil {
		return nil, err
	}
	if maxSessions < 1 {
		return nil, errors.New("limite deve ser positivo")
	}
	slots := make(chan struct{}, maxSessions)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ssh" || r.URL.RawQuery != "" || r.URL.ForceQuery {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !probe.Authorize(w, r, tokenFile) {
			return
		}
		offered := false
		for _, value := range r.Header.Values("Sec-WebSocket-Protocol") {
			for _, protocol := range strings.Split(value, ",") {
				if strings.TrimSpace(protocol) == Protocol {
					offered = true
				}
			}
		}
		if !offered {
			http.Error(w, "unsupported protocol", http.StatusBadRequest)
			return
		}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
		default:
			http.Error(w, "busy", http.StatusServiceUnavailable)
			return
		}
		ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{Protocol}, CompressionMode: websocket.CompressionDisabled})
		if err != nil {
			return
		}
		defer ws.CloseNow()
		op, stop := context.WithTimeout(ctx, 10*time.Second)
		conn, err := (&net.Dialer{}).DialContext(op, "tcp", target)
		stop()
		if err != nil {
			_ = ws.Close(websocket.StatusInternalError, "SSH unavailable")
			return
		}
		_ = Relay(ctx, ws, TCP(conn))
	}), nil
}

func Dial(ctx context.Context, client *http.Client, rawURL, tokenFile string) (*websocket.Conn, error) {
	if err := ValidateURL(rawURL); err != nil {
		return nil, err
	}
	token, err := probe.ReadToken(tokenFile)
	if err != nil {
		return nil, err
	}
	op, stop := context.WithTimeout(ctx, 15*time.Second)
	defer stop()
	ws, response, err := websocket.Dial(op, rawURL, &websocket.DialOptions{HTTPClient: client, HTTPHeader: http.Header{"Authorization": []string{"Bearer " + token}}, Subprotocols: []string{Protocol}, CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		if response != nil {
			return nil, fmt.Errorf("handshake HTTP %d", response.StatusCode)
		}
		return nil, errors.New("falha na conexão WSS: confira DNS, TLS, proxy e rede")
	}
	if ws.Subprotocol() != Protocol {
		ws.CloseNow()
		return nil, errors.New("servidor não negociou protocolo v1")
	}
	return ws, nil
}

// ProxyTCP is the introductory TCP-only relay, with half-close and cancellation.
func ProxyTCP(ctx context.Context, client net.Conn, target string) error {
	upstream, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", target)
	if err != nil {
		client.Close()
		return err
	}
	defer upstream.Close()
	defer client.Close()
	done := make(chan error, 2)
	copyHalf := func(dst, src net.Conn) {
		_, err := io.Copy(dst, src)
		if err == nil {
			err = TCP(dst).CloseWrite()
		}
		done <- err
	}
	go copyHalf(upstream, client)
	go copyHalf(client, upstream)
	var result error
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err != nil {
				result = err
				upstream.Close()
				client.Close()
			}
		case <-ctx.Done():
			upstream.Close()
			client.Close()
			for ; i < 2; i++ {
				<-done
			}
			return ctx.Err()
		}
	}
	return result
}
