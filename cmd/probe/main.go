// Command probe tests WebSocket over TLS before implementing the SSH tunnel.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/LuizArtur29/go-ws-tunnel/internal/probe"
	"github.com/LuizArtur29/go-ws-tunnel/internal/tunnel"
	"github.com/coder/websocket"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		// Avoid printing library errors that may contain private URLs or credentials.
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("uso: probe server|client [opções]; use -h após o comando")
	}
	switch args[0] {
	case "server":
		return server(ctx, args[1:])
	case "client":
		return client(ctx, args[1:])
	default:
		return errors.New("comando inválido: use server ou client")
	}
}

func server(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	listen := flags.String("listen", "127.0.0.1:8443", "endereço de escuta TLS")
	unixPath := flags.String("unix", "", "socket Unix para TLS atrás de proxy; exclusivo com -listen")
	cert := flags.String("cert", "", "certificado PEM (cadeia completa)")
	key := flags.String("key", "", "chave privada PEM")
	token := flags.String("token-file", "", "arquivo com token compartilhado")
	sshTarget := flags.String("ssh-target", "", "habilita /ssh para destino loopback fixo, ex. 127.0.0.1:22")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *cert == "" || *key == "" || *token == "" || flags.NArg() != 0 {
		return errors.New("informe -cert, -key e -token-file")
	}
	if _, err := probe.ReadToken(*token); err != nil {
		return err
	}
	tcpExplicit := false
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "listen" {
			tcpExplicit = true
		}
	})
	if *unixPath != "" && tcpExplicit {
		return errors.New("use apenas -listen ou -unix")
	}
	listener, err := openListener(*listen, *unixPath)
	if err != nil {
		return errors.New("não foi possível abrir o listener; confira endereço, permissões e socket existente")
	}
	defer listener.Close()
	mux := http.NewServeMux()
	mux.Handle("/probe", probe.Handler(ctx, *token, 8))
	if *sshTarget != "" {
		handler, err := tunnel.Handler(ctx, *token, *sshTarget, 8)
		if err != nil {
			return err
		}
		mux.Handle("/ssh", handler)
	}
	srv := &http.Server{
		Addr: *listen, Handler: mux,
		ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second,
		MaxHeaderBytes: 8192, TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = srv.Shutdown(shutdown)
		case <-done:
		}
	}()
	defer close(done)
	fmt.Println("Servidor TLS iniciado; diagnóstico /probe, SSH opcional.")
	err = srv.ServeTLS(listener, *cert, *key)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	if err != nil {
		return errors.New("não foi possível iniciar o servidor TLS: confira porta, certificado e chave")
	}
	return nil
}

func client(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("client", flag.ContinueOnError)
	rawURL := flags.String("url", "", "wss://DOMINIO[:PORTA]/probe")
	tokenFile := flags.String("token-file", "", "arquivo com token compartilhado")
	ca := flags.String("ca", "", "certificado de CA adicional para laboratório")
	duration := flags.Duration("duration", 30*time.Second, "duração do teste")
	interval := flags.Duration("interval", time.Second, "intervalo entre trocas; aumente para testar ociosidade")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := probe.ValidateURL(*rawURL); err != nil {
		return err
	}
	if *duration <= 0 || *interval <= 0 || flags.NArg() != 0 {
		return errors.New("duração e intervalo devem ser positivos; não use argumentos posicionais")
	}
	token, err := probe.ReadToken(*tokenFile)
	if err != nil {
		return err
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
	if *ca != "" {
		pem, err := os.ReadFile(*ca)
		if err != nil {
			return errors.New("não foi possível ler a CA")
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pem) {
			return errors.New("CA PEM inválida")
		}
		tlsConfig.RootCAs = roots
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig
	defer transport.CloseIdleConnections()
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	conn, response, err := websocket.Dial(dialCtx, *rawURL, &websocket.DialOptions{
		HTTPClient:      &http.Client{Transport: transport, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
		HTTPHeader:      http.Header{"Authorization": []string{"Bearer " + token}},
		CompressionMode: websocket.CompressionDisabled,
	})
	cancel()
	if err != nil {
		if response != nil {
			return fmt.Errorf("handshake recusado: HTTP %d", response.StatusCode)
		}
		return errors.New("falha na conexão: verifique DNS, TLS, proxy e acesso à porta")
	}
	defer conn.CloseNow()
	conn.SetReadLimit(probe.MaxMessage)
	started := time.Now()
	deadline := started.Add(*duration)
	count := 0
	for {
		if err := ctx.Err(); err != nil {
			return errors.New("teste interrompido")
		}
		op, cancel := context.WithTimeout(ctx, 10*time.Second)
		latency, err := probe.RoundTrip(op, conn)
		cancel()
		if err != nil {
			return errors.New("falha durante troca de dados; conexão não permaneceu utilizável")
		}
		count++
		fmt.Printf("troca=%d rtt=%s\n", count, latency.Round(time.Microsecond))
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		wait := min(*interval, remaining)
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return errors.New("teste interrompido")
		case <-timer.C:
		}
	}
	_ = conn.Close(websocket.StatusNormalClosure, "probe completed")
	fmt.Printf("OK: %d trocas verificadas em %s na mesma conexão TLS.\n", count, time.Since(started).Round(time.Millisecond))
	return nil
}

// openListener never removes an existing path: it may belong to another process.
// Unix sockets are removed by net.UnixListener.Close during normal shutdown.
func openListener(address, unixPath string) (net.Listener, error) {
	if unixPath == "" {
		return net.Listen("tcp", address)
	}
	listener, err := net.Listen("unix", unixPath)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(unixPath, 0600); err != nil {
		_ = listener.Close()
		return nil, err
	}
	return listener, nil
}
