package bunny

import (
	"context"
	"fmt"
	"sort"
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
	if identifierKey("api.example.com", "A", "1.1.1.1") != "api.example.com|A|1.1.1.1" {
		t.Errorf("unexpected key shape")
	}
	if identifierKey("api.example.com", "A", "1.1.1.1") == identifierKey("api.example.com", "AAAA", "1.1.1.1") {
		t.Errorf("different types should produce different keys")
	}
	if identifierKey("api.example.com", "A", "1.1.1.1") == identifierKey("api.example.com", "A", "2.2.2.2") {
		t.Errorf("different values should produce different keys")
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

func newPopulatedProvider(t *testing.T, mc *mockClient, zones ...*Zone) *Provider {
	t.Helper()
	p := newTestProvider(t, mc)
	for _, z := range zones {
		p.cacheZone(z)
	}
	return p
}

func TestApplyChanges_CreatesOneRecordPerTarget(t *testing.T) {
	zone := &Zone{ID: 42, Domain: "example.com"}
	mc := &mockClient{listResp: &ListZonesResponse{Items: []*Zone{zone}}}
	p := newPopulatedProvider(t, mc, zone)

	changes := &plan.Changes{
		Create: []*endpoint.Endpoint{{
			DNSName:    "api.example.com",
			RecordType: "A",
			RecordTTL:  300,
			Targets:    endpoint.Targets{"1.1.1.1", "2.2.2.2"},
		}},
	}

	if err := p.ApplyChanges(context.Background(), changes); err != nil {
		t.Fatalf("ApplyChanges: %v", err)
	}
	if got := mc.CountByMethod("CreateRecord"); got != 2 {
		t.Errorf("CreateRecord calls: got %d want 2", got)
	}

	values := []string{}
	for _, c := range mc.Calls() {
		if c.method != "CreateRecord" {
			continue
		}
		values = append(values, c.args.(CreateRecordRequest).Value)
	}
	sort.Strings(values)
	if values[0] != "1.1.1.1" || values[1] != "2.2.2.2" {
		t.Errorf("created values: got %v", values)
	}
}

func TestApplyChanges_DiffsTargetsOnUpdate(t *testing.T) {
	zone := &Zone{ID: 42, Domain: "example.com", Records: []*Record{
		{ID: 1, Name: "api", Type: RecordTypeA, Value: "1.1.1.1", TTLSeconds: 300, Weight: 100},
		{ID: 2, Name: "api", Type: RecordTypeA, Value: "2.2.2.2", TTLSeconds: 300, Weight: 100},
	}}
	mc := &mockClient{listResp: &ListZonesResponse{Items: []*Zone{zone}}}
	p := newPopulatedProvider(t, mc, zone)

	old := &endpoint.Endpoint{
		DNSName: "api.example.com", RecordType: "A", RecordTTL: 300,
		Targets: endpoint.Targets{"1.1.1.1", "2.2.2.2"},
	}
	new := &endpoint.Endpoint{
		DNSName: "api.example.com", RecordType: "A", RecordTTL: 300,
		Targets: endpoint.Targets{"1.1.1.1", "3.3.3.3"},
	}
	changes := &plan.Changes{UpdateOld: []*endpoint.Endpoint{old}, UpdateNew: []*endpoint.Endpoint{new}}

	if err := p.ApplyChanges(context.Background(), changes); err != nil {
		t.Fatalf("ApplyChanges: %v", err)
	}

	if got := mc.CountByMethod("DeleteRecord"); got != 1 {
		t.Errorf("DeleteRecord calls: got %d want 1 (the 2.2.2.2 record)", got)
	}
	if got := mc.CountByMethod("CreateRecord"); got != 1 {
		t.Errorf("CreateRecord calls: got %d want 1 (the new 3.3.3.3 record)", got)
	}
	if got := mc.CountByMethod("UpdateRecord"); got != 0 {
		t.Errorf("UpdateRecord calls: got %d want 0 (unchanged 1.1.1.1)", got)
	}
}

func TestApplyChanges_UpdateMetadataTouchesEverySurvivingRecord(t *testing.T) {
	zone := &Zone{ID: 42, Domain: "example.com", Records: []*Record{
		{ID: 1, Name: "api", Type: RecordTypeA, Value: "1.1.1.1", TTLSeconds: 300, Weight: 100},
		{ID: 2, Name: "api", Type: RecordTypeA, Value: "2.2.2.2", TTLSeconds: 300, Weight: 100},
	}}
	mc := &mockClient{listResp: &ListZonesResponse{Items: []*Zone{zone}}}
	p := newPopulatedProvider(t, mc, zone)

	old := &endpoint.Endpoint{
		DNSName: "api.example.com", RecordType: "A", RecordTTL: 300,
		Targets: endpoint.Targets{"1.1.1.1", "2.2.2.2"},
	}
	new := &endpoint.Endpoint{
		DNSName: "api.example.com", RecordType: "A", RecordTTL: 600,
		Targets: endpoint.Targets{"1.1.1.1", "2.2.2.2"},
	}
	changes := &plan.Changes{UpdateOld: []*endpoint.Endpoint{old}, UpdateNew: []*endpoint.Endpoint{new}}

	if err := p.ApplyChanges(context.Background(), changes); err != nil {
		t.Fatalf("ApplyChanges: %v", err)
	}

	if got := mc.CountByMethod("UpdateRecord"); got != 2 {
		t.Errorf("UpdateRecord calls: got %d want 2 (TTL changed for both surviving)", got)
	}
}

func TestApplyChanges_DeletesEveryTarget(t *testing.T) {
	zone := &Zone{ID: 42, Domain: "example.com", Records: []*Record{
		{ID: 1, Name: "api", Type: RecordTypeA, Value: "1.1.1.1", TTLSeconds: 300, Weight: 100},
		{ID: 2, Name: "api", Type: RecordTypeA, Value: "2.2.2.2", TTLSeconds: 300, Weight: 100},
	}}
	mc := &mockClient{listResp: &ListZonesResponse{Items: []*Zone{zone}}}
	p := newPopulatedProvider(t, mc, zone)

	changes := &plan.Changes{
		Delete: []*endpoint.Endpoint{{
			DNSName: "api.example.com", RecordType: "A", RecordTTL: 300,
			Targets: endpoint.Targets{"1.1.1.1", "2.2.2.2"},
		}},
	}

	if err := p.ApplyChanges(context.Background(), changes); err != nil {
		t.Fatalf("ApplyChanges: %v", err)
	}
	if got := mc.CountByMethod("DeleteRecord"); got != 2 {
		t.Errorf("DeleteRecord calls: got %d want 2", got)
	}
}

func TestAdjustEndpoints_EmptyZoneCacheSkipsAPI(t *testing.T) {
	mc := &mockClient{}
	p := newTestProvider(t, mc)

	in := []*endpoint.Endpoint{{DNSName: "api.example.com", RecordType: "A"}}
	out, err := p.AdjustEndpoints(in)
	if err != nil {
		t.Fatalf("AdjustEndpoints: %v", err)
	}
	if len(out) != 1 {
		t.Errorf("expected pass-through, got %d", len(out))
	}
	if got := mc.CountByMethod("ListZones"); got != 0 {
		t.Errorf("ListZones calls: got %d want 0 (empty cache should skip)", got)
	}
}

func TestNewProvider_FailsWhenZoneFetchFails(t *testing.T) {
	mc := &mockClient{listErr: fmt.Errorf("forced API failure")}
	_, err := NewProvider(mc, Options{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNewProvider_SucceedsWhenZoneFetchOK(t *testing.T) {
	mc := &mockClient{listResp: &ListZonesResponse{Items: []*Zone{{ID: 1, Domain: "example.com"}}}}
	p, err := NewProvider(mc, Options{})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if p == nil {
		t.Fatal("expected provider, got nil")
	}
	zones := p.allZones()
	if len(zones) != 1 || zones[0] != "example.com" {
		t.Fatalf("zone cache: %v", zones)
	}
}

// TestAdjustEndpoints_PreservesIncomingLabels covers the pass-through case.
// The label-copy path inside AdjustEndpoints is currently dead code in tests
// because aggregateRecords does not populate endpoint.Labels — when that
// changes, add a separate test exercising the copy path explicitly.
func TestAdjustEndpoints_PreservesIncomingLabels(t *testing.T) {
	zone := &Zone{ID: 42, Domain: "example.com", Records: []*Record{
		{ID: 1, Name: "api", Type: RecordTypeA, Value: "1.1.1.1", TTLSeconds: 300, Weight: 100},
	}}
	mc := &mockClient{listResp: &ListZonesResponse{Items: []*Zone{zone}}}
	p := newPopulatedProvider(t, mc, zone)

	incoming := &endpoint.Endpoint{
		DNSName:    "api.example.com",
		RecordType: "A",
		Targets:    endpoint.Targets{"1.1.1.1"},
		Labels:     endpoint.Labels{"caller": "external-dns"},
	}
	out, err := p.AdjustEndpoints([]*endpoint.Endpoint{incoming})
	if err != nil {
		t.Fatalf("AdjustEndpoints: %v", err)
	}
	if len(out) != 1 || out[0].Labels["caller"] != "external-dns" {
		t.Fatalf("incoming labels lost: %v", out)
	}
}
