package server_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/server"
)

func TestBindGuardRefusesNonLoopbackWithoutTLS(t *testing.T) {
	cases := []struct {
		name          string
		addr          string
		hasTLS        bool
		allowInsecure bool
		wantErr       bool
	}{
		{"loopback ip", "127.0.0.1:0", false, false, false},
		{"loopback name", "localhost:8080", false, false, false},
		{"ipv6 loopback", "[::1]:8080", false, false, false},
		{"all interfaces", ":8080", false, false, true},
		{"public address", "0.0.0.0:8080", false, false, true},
		{"public with tls", "0.0.0.0:8080", true, false, false},
		{"public with opt-out", "0.0.0.0:8080", false, true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := server.CheckBindSafety(tc.addr, tc.hasTLS, tc.allowInsecure)
			if tc.wantErr && err == nil {
				t.Fatalf("binding %q was allowed, want a refusal", tc.addr)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("binding %q was refused: %v", tc.addr, err)
			}
		})
	}
}

func TestNewRefusesAPublicBindWithoutTLS(t *testing.T) {
	_, err := server.New(server.Config{Addr: "0.0.0.0:0", Handler: okHandler()})
	if err == nil {
		t.Fatal("a public bind without tls was accepted")
	}
	if !strings.Contains(err.Error(), "tls") {
		t.Errorf("error = %q, want it to name the missing condition", err)
	}
}

func TestNewAcceptsAPublicBindWithTheOptOut(t *testing.T) {
	if _, err := server.New(server.Config{
		Addr: "0.0.0.0:0", Handler: okHandler(), AllowInsecure: true,
	}); err != nil {
		t.Fatalf("the insecure opt-out was refused: %v", err)
	}
}

func TestNewRejectsAnIncompleteConfiguration(t *testing.T) {
	if _, err := server.New(server.Config{}); err == nil {
		t.Error("a server without a handler was accepted")
	}
	if _, err := server.New(server.Config{Handler: okHandler(), CertFile: "cert.pem"}); err == nil {
		t.Error("a certificate without a key was accepted")
	}
	if _, err := server.New(server.Config{Handler: okHandler(), KeyFile: "key.pem"}); err == nil {
		t.Error("a key without a certificate was accepted")
	}
}

func TestNewReportsAnUnreadableCertificate(t *testing.T) {
	dir := t.TempDir()
	cert := filepath.Join(dir, "cert.pem")
	key := filepath.Join(dir, "key.pem")
	if err := os.WriteFile(cert, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("writing certificate: %v", err)
	}
	if err := os.WriteFile(key, []byte("not a key"), 0o600); err != nil {
		t.Fatalf("writing key: %v", err)
	}

	_, err := server.New(server.Config{Addr: "127.0.0.1:0", Handler: okHandler(),
		CertFile: cert, KeyFile: key})
	if err == nil {
		t.Fatal("an unparseable certificate was accepted")
	}
	if !strings.Contains(err.Error(), cert) {
		t.Errorf("error = %q, want it to name the failing file", err)
	}
}

func TestListenReportsABoundAddress(t *testing.T) {
	srv := mustServer(t, server.Config{Addr: "127.0.0.1:0", Handler: okHandler()})
	if err := srv.Listen(); err != nil {
		t.Fatalf("listening: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	if srv.Addr() == "" {
		t.Fatal("the bound address is unknown after Listen")
	}
	if srv.TLSEnabled() {
		t.Error("plain http reports tls enabled")
	}
}

func TestListenFailureNamesTheAddress(t *testing.T) {
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("taking a port: %v", err)
	}
	defer func() { _ = taken.Close() }()

	srv := mustServer(t, server.Config{Addr: taken.Addr().String(), Handler: okHandler()})
	err = srv.Listen()
	if err == nil {
		t.Fatal("binding an occupied port succeeded")
	}
	if !strings.Contains(err.Error(), taken.Addr().String()) {
		t.Errorf("error = %q, want it to name the address", err)
	}
}

func TestGracefulShutdownCompletesInFlightRequests(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	srv := mustServer(t, server.Config{
		Addr:            "127.0.0.1:0",
		ShutdownTimeout: 5 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(started)
			<-release
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("finished"))
		}),
	})
	if err := srv.Listen(); err != nil {
		t.Fatalf("listening: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()

	type result struct {
		body string
		err  error
	}
	responses := make(chan result, 1)
	go func() {
		resp, err := http.Get("http://" + srv.Addr() + "/")
		if err != nil {
			responses <- result{err: err}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		buf := make([]byte, 8)
		n, _ := resp.Body.Read(buf)
		responses <- result{body: string(buf[:n])}
	}()

	<-started
	cancel()
	close(release)

	got := <-responses
	if got.err != nil {
		t.Fatalf("the in-flight request failed: %v", got.err)
	}
	if got.body != "finished" {
		t.Errorf("body = %q, want the handler's response", got.body)
	}
	if err := <-done; err != nil {
		t.Fatalf("serving: %v", err)
	}

	late, err := http.Get("http://" + srv.Addr() + "/")
	if err == nil {
		_ = late.Body.Close()
		t.Error("a new connection was accepted after shutdown")
	}
}

func TestShutdownTimeoutIsReported(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	srv := mustServer(t, server.Config{
		Addr:            "127.0.0.1:0",
		ShutdownTimeout: 20 * time.Millisecond,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			close(started)
			<-release
			w.WriteHeader(http.StatusOK)
		}),
	})
	if err := srv.Listen(); err != nil {
		t.Fatalf("listening: %v", err)
	}
	defer close(release)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()
	go func() {
		resp, err := http.Get("http://" + srv.Addr() + "/")
		if err == nil {
			_ = resp.Body.Close()
		}
	}()

	<-started
	cancel()
	if err := <-done; !errors.Is(err, server.ErrShutdownTimeout) {
		t.Fatalf("serve returned %v, want the shutdown timeout", err)
	}
}

func TestServeRunsAndStopsWorkers(t *testing.T) {
	ran := make(chan string, 3)
	stopped := make(chan string, 3)
	worker := func(name string, fail bool) server.Worker {
		return server.FuncWorker{WorkerName: name, Fn: func(ctx context.Context) error {
			ran <- name
			if fail {
				return errors.New("worker failed")
			}
			<-ctx.Done()
			stopped <- name
			return ctx.Err()
		}}
	}

	srv := mustServer(t, server.Config{
		Addr:    "127.0.0.1:0",
		Handler: okHandler(),
		Workers: []server.Worker{worker("sweeper", false), worker("failing", true), nil},
	})
	if err := srv.Listen(); err != nil {
		t.Fatalf("listening: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()

	seen := map[string]bool{}
	for range 2 {
		seen[<-ran] = true
	}
	if !seen["sweeper"] || !seen["failing"] {
		t.Fatalf("workers that ran = %v, want both", seen)
	}

	resp, err := http.Get("http://" + srv.Addr() + "/")
	if err != nil {
		t.Fatalf("the http surface stopped serving after a worker failed: %v", err)
	}
	_ = resp.Body.Close()

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serving: %v", err)
	}
	if got := <-stopped; got != "sweeper" {
		t.Errorf("stopped worker = %q, want the sweeper", got)
	}
}

func TestServeOverTLS(t *testing.T) {
	cert, key := writeSelfSigned(t)
	srv := mustServer(t, server.Config{
		Addr: "127.0.0.1:0", Handler: okHandler(), CertFile: cert, KeyFile: key,
	})
	if !srv.TLSEnabled() {
		t.Fatal("a configured certificate did not enable tls")
	}
	if err := srv.Listen(); err != nil {
		t.Fatalf("listening: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 -- self-signed test certificate
	}}
	resp, err := client.Get("https://" + srv.Addr() + "/")
	if err != nil {
		t.Fatalf("requesting over tls: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	_ = resp.Body.Close()

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serving: %v", err)
	}
}

func TestServeListensWhenTheCallerDidNot(t *testing.T) {
	srv := mustServer(t, server.Config{Addr: "127.0.0.1:0", Handler: okHandler()})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx) }()

	deadline := time.After(2 * time.Second)
	for srv.Addr() == "" {
		select {
		case <-deadline:
			t.Fatal("serve never bound an address")
		default:
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("serving: %v", err)
	}
}

func mustServer(t *testing.T, cfg server.Config) *server.Server {
	t.Helper()
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("building server: %v", err)
	}
	return srv
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// writeSelfSigned writes a throwaway certificate and key for the TLS test.
func writeSelfSigned(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating certificate: %v", err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshalling key: %v", err)
	}

	dir := t.TempDir()
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	writePEM(t, certPath, "CERTIFICATE", der)
	writePEM(t, keyPath, "EC PRIVATE KEY", keyDER)
	return certPath, keyPath
}

func writePEM(t *testing.T, path, blockType string, der []byte) {
	t.Helper()
	f, err := os.Create(path) // #nosec G304 -- path is inside the test's temporary directory
	if err != nil {
		t.Fatalf("creating %q: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := pem.Encode(f, &pem.Block{Type: blockType, Bytes: der}); err != nil {
		t.Fatalf("encoding %q: %v", path, err)
	}
}
