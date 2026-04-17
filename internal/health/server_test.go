package health

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// pickPort opens a listener on :0, grabs the assigned port, then closes the
// listener so the test's server can bind it.
func pickPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := strings.Split(l.Addr().String(), ":")
	l.Close()
	return port[len(port)-1]
}

func TestServer_GracefulShutdown(t *testing.T) {
	s := &Server{Options: Options{
		Host:         "127.0.0.1",
		Port:         pickPort(t),
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
	}}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Serve(ctx) }()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("graceful shutdown should return nil, got %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("Serve did not return after 15s")
	}
}

func TestServer_BindError(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()

	port := strings.Split(l.Addr().String(), ":")
	s := &Server{Options: Options{
		Host:         "127.0.0.1",
		Port:         port[len(port)-1],
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
	}}

	err = s.Serve(context.Background())
	if err == nil {
		t.Fatal("expected bind error, got nil")
	}
	if !strings.Contains(err.Error(), "address already in use") &&
		!strings.Contains(err.Error(), "bind") {
		t.Fatalf("expected bind-related error, got %v", err)
	}
}

func TestServer_HealthzReports200WhenHealthy(t *testing.T) {
	s := &Server{Options: Options{
		Host:         "127.0.0.1",
		Port:         pickPort(t),
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
	}}
	s.SetHealthy(true)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Serve(ctx)
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://" + s.Options.Addr() + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("status: got %d body=%q", resp.StatusCode, body)
	}
}

func TestServer_HealthzReports503WhenUnhealthy(t *testing.T) {
	s := &Server{Options: Options{
		Host:         "127.0.0.1",
		Port:         pickPort(t),
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
	}}
	s.SetHealthy(false)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Serve(ctx)
	time.Sleep(100 * time.Millisecond)

	resp, err := http.Get("http://" + s.Options.Addr() + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want 503", resp.StatusCode)
	}
}

var _ = errors.Is
