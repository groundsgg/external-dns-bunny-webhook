package webhook

import (
	"context"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

type noopProvider struct{}

func (noopProvider) Records(context.Context) ([]*endpoint.Endpoint, error) {
	return nil, nil
}
func (noopProvider) ApplyChanges(context.Context, *plan.Changes) error { return nil }
func (noopProvider) AdjustEndpoints(eps []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	return eps, nil
}
func (noopProvider) GetDomainFilter() endpoint.DomainFilterInterface {
	return endpoint.NewDomainFilter(nil)
}

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

func TestServer_RequiresProvider(t *testing.T) {
	s := &Server{Options: Options{
		Host:         "127.0.0.1",
		Port:         pickPort(t),
		ReadTimeout:  time.Second,
		WriteTimeout: time.Second,
	}}
	err := s.Serve(context.Background())
	if err == nil || !strings.Contains(err.Error(), "provider is required") {
		t.Fatalf("expected provider-required error, got %v", err)
	}
}

func TestServer_GracefulShutdown(t *testing.T) {
	s := &Server{
		Options: Options{
			Host:         "127.0.0.1",
			Port:         pickPort(t),
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		},
		Provider: &noopProvider{},
	}

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

func TestServer_HealthyCallback(t *testing.T) {
	var mu sync.Mutex
	calls := []bool{}
	s := &Server{
		Options: Options{
			Host:         "127.0.0.1",
			Port:         pickPort(t),
			ReadTimeout:  time.Second,
			WriteTimeout: time.Second,
		},
		Provider: &noopProvider{},
		HealthyFunc: func(b bool) {
			mu.Lock()
			defer mu.Unlock()
			calls = append(calls, b)
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	go s.Serve(ctx)
	time.Sleep(100 * time.Millisecond)
	cancel()
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 || calls[0] != true || calls[1] != false {
		t.Fatalf("expected [true, false], got %v", calls)
	}
}
