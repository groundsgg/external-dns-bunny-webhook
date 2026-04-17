package bunny

import (
	"context"
	"testing"

	"github.com/puzpuzpuz/xsync/v3"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
)

// newTestProvider builds a Provider directly (bypassing NewProvider's
// startup zone fetch) so tests can wire a mock client without making
// real API calls.
func newTestProvider(t *testing.T, mc *mockClient) *Provider {
	t.Helper()
	return &Provider{
		Options: Options{},
		client:  mc,
		filter:  endpoint.NewDomainFilter(nil),
		zoneMap: xsync.NewMapOf[string, int64](),
	}
}

func TestIdentifierKey(t *testing.T) {
	if identifierKey("api.example.com", "A") != "api.example.com|A" {
		t.Errorf("unexpected key shape")
	}
	if identifierKey("api.example.com", "A") == identifierKey("api.example.com", "AAAA") {
		t.Errorf("different types should produce different keys")
	}
}

func TestApplyChanges_NilOrEmpty(t *testing.T) {
	mc := &mockClient{}
	p := newTestProvider(t, mc)

	if err := p.ApplyChanges(context.Background(), nil); err != nil {
		t.Fatalf("nil changes should be a no-op, got %v", err)
	}
	if err := p.ApplyChanges(context.Background(), &plan.Changes{}); err != nil {
		t.Fatalf("empty changes should be a no-op, got %v", err)
	}
	if got := mc.CountByMethod("CreateRecord"); got != 0 {
		t.Errorf("expected 0 create calls, got %d", got)
	}
	if got := mc.CountByMethod("DeleteRecord"); got != 0 {
		t.Errorf("expected 0 delete calls, got %d", got)
	}

	// Same shape under DryRun — exercises applyChangesDryRun's nil guard.
	pdry := newTestProvider(t, mc)
	pdry.Options.DryRun = true
	if err := pdry.ApplyChanges(context.Background(), nil); err != nil {
		t.Fatalf("dry-run nil changes should be a no-op, got %v", err)
	}
	if err := pdry.ApplyChanges(context.Background(), &plan.Changes{}); err != nil {
		t.Fatalf("dry-run empty changes should be a no-op, got %v", err)
	}
}
