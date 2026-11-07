// Command tunnel provides stdio (SSH ProxyCommand), loopback TCP, and a TCP lab.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/LuizArtur29/go-ws-tunnel/internal/probe"
	"github.com/LuizArtur29/go-ws-tunnel/internal/tunnel"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil && !errors.Is(err, flag.ErrHelp) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("uso: tunnel stdio|client|tcp-proxy [opções]")
	}
	mode := args[0]
	if mode != "stdio" && mode != "client" && mode != "tcp-proxy" {
		return errors.New("comando desconhecido")
	}
	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	rawURL := flags.String("url", "", "wss://DOMINIO/ssh")
	token := flags.String("token-file", "", "arquivo de token")
	ca := flags.String("ca", "", "CA adicional para laboratório")
	listen := flags.String("listen", "127.0.0.1:2222", "porta TCP local (somente loopback)")
	target := flags.String("target", "", "destino loopback fixo do exercício TCP")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("argumentos posicionais não suportados")
	}
	var client *http.Client
	if mode != "tcp-proxy" {
		if err := tunnel.ValidateURL(*rawURL); err != nil {
			return err
		}
		if _, err := probe.ReadToken(*token); err != nil {
			return err
		}
		tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12}
		if *ca != "" {
			pem, err := os.ReadFile(*ca)
			if err != nil {
				return errors.New("falha ao ler CA")
			}
			pool, err := x509.SystemCertPool()
			if err != nil {
				pool = x509.NewCertPool()
			}
			if !pool.AppendCertsFromPEM(pem) {
				return errors.New("CA inválida")
			}
			tlsConfig.RootCAs = pool
		}
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = tlsConfig
		defer transport.CloseIdleConnections()
		client = &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	} else if err := tunnel.ValidateTarget(*target); err != nil {
		return err
	}
	if mode == "stdio" {
		ws, err := tunnel.Dial(ctx, client, *rawURL, *token)
		if err != nil {
			return err
		}
		endpoint := tunnel.Endpoint{Reader: os.Stdin, Writer: os.Stdout, CloseWrite: os.Stdout.Close, Close: func() error { _ = os.Stdin.Close(); return os.Stdout.Close() }}
		if err := tunnel.Relay(ctx, ws, endpoint); err != nil && !errors.Is(err, io.EOF) {
			return errors.New("sessão de transporte encerrada")
		}
		return nil
	}
	if err := tunnel.ValidateTarget(*listen); err != nil {
		return errors.New("listener deve usar IP loopback literal e porta válida")
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return errors.New("falha ao abrir porta local")
	}
	defer listener.Close()
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { <-child.Done(); listener.Close() }()
	slots := make(chan struct{}, 32)
	var workers sync.WaitGroup
	defer workers.Wait()
	// Each accepted local connection owns a distinct WebSocket session.
	fmt.Fprintln(os.Stderr, "Listener local ativo em", listener.Addr())
	for {
		conn, err := listener.Accept()
		if err != nil {
			cancel()
			if ctx.Err() != nil {
				return nil
			}
			return errors.New("listener interrompido")
		}
		select {
		case slots <- struct{}{}:
		default:
			conn.Close()
			continue
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			defer func() { <-slots }()
			defer conn.Close()
			if mode == "tcp-proxy" {
				_ = tunnel.ProxyTCP(child, conn, *target)
				return
			}
			ws, err := tunnel.Dial(child, client, *rawURL, *token)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				return
			}
			if err := tunnel.Relay(child, ws, tunnel.TCP(conn)); err != nil && child.Err() == nil {
				fmt.Fprintln(os.Stderr, "sessão local encerrada")
			}
		}()
	}
}
