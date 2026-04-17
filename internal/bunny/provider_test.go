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

func TestExtractRecordComponents(t *testing.T) {
	cases := []struct {
		name     string
		zones    []string
		dns      string
		wantName string
		wantZone string
		wantOK   bool
	}{
		{"apex", []string{"example.com"}, "example.com", "", "example.com", true},
		{"subdomain", []string{"example.com"}, "api.example.com", "api", "example.com", true},
		{"deep subdomain", []string{"example.com"}, "a.b.c.example.com", "a.b.c", "example.com", true},
		{"suffix attack", []string{"example.com"}, "malicious-example.com", "", "", false},
		{"longest match", []string{"example.com", "b.example.com"}, "a.b.example.com", "a", "b.example.com", true},
		{"longest match reversed", []string{"b.example.com", "example.com"}, "a.b.example.com", "a", "b.example.com", true},
		{"no match", []string{"example.com"}, "foo.bar", "", "", false},
		{"empty zones", nil, "anything.com", "", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotName, gotZone, gotOK := extractRecordComponents(tc.zones, tc.dns)
			if gotName != tc.wantName || gotZone != tc.wantZone || gotOK != tc.wantOK {
				t.Errorf("extractRecordComponents(%v, %q) = (%q,%q,%v) want (%q,%q,%v)",
					tc.zones, tc.dns, gotName, gotZone, gotOK, tc.wantName, tc.wantZone, tc.wantOK)
			}
		})
	}
}
