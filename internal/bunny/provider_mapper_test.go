package bunny

import (
	"sort"
	"testing"

	"sigs.k8s.io/external-dns/endpoint"
)

func TestAggregateRecords_GroupsMultipleTargets(t *testing.T) {
	zone := &Zone{
		Domain: "example.com",
		Records: []*Record{
			{Name: "api", Type: RecordTypeA, TTLSeconds: 300, Value: "1.1.1.1", Weight: 100},
			{Name: "api", Type: RecordTypeA, TTLSeconds: 300, Value: "2.2.2.2", Weight: 100},
			{Name: "api", Type: RecordTypeAAAA, TTLSeconds: 300, Value: "::1", Weight: 100},
			{Name: "www", Type: RecordTypeCNAME, TTLSeconds: 600, Value: "api.example.com", Weight: 100},
		},
	}

	eps := aggregateRecords(zone)
	if len(eps) != 3 {
		t.Fatalf("expected 3 endpoints (api A, api AAAA, www CNAME), got %d", len(eps))
	}

	for _, ep := range eps {
		switch {
		case ep.DNSName == "api.example.com" && ep.RecordType == "A":
			sort.Strings(ep.Targets)
			if len(ep.Targets) != 2 || ep.Targets[0] != "1.1.1.1" || ep.Targets[1] != "2.2.2.2" {
				t.Errorf("api A targets: got %v want [1.1.1.1, 2.2.2.2]", ep.Targets)
			}
		case ep.DNSName == "api.example.com" && ep.RecordType == "AAAA":
			if len(ep.Targets) != 1 || ep.Targets[0] != "::1" {
				t.Errorf("api AAAA targets: %v", ep.Targets)
			}
		case ep.DNSName == "www.example.com" && ep.RecordType == "CNAME":
			if len(ep.Targets) != 1 || ep.Targets[0] != "api.example.com" {
				t.Errorf("www CNAME targets: %v", ep.Targets)
			}
		default:
			t.Errorf("unexpected endpoint: %s %s %v", ep.DNSName, ep.RecordType, ep.Targets)
		}
	}
}

func TestAggregateRecords_HandlesApex(t *testing.T) {
	zone := &Zone{
		Domain: "example.com",
		Records: []*Record{
			{Name: "", Type: RecordTypeA, TTLSeconds: 300, Value: "1.1.1.1", Weight: 100},
		},
	}
	eps := aggregateRecords(zone)
	if len(eps) != 1 || eps[0].DNSName != "example.com" {
		t.Fatalf("apex DNSName: got %q want example.com (eps=%v)", eps[0].DNSName, eps)
	}
}

func TestAggregateRecords_SmartLatencyDifferentZones(t *testing.T) {
	zone := &Zone{
		Domain: "example.com",
		Records: []*Record{
			{Name: "api", Type: RecordTypeA, TTLSeconds: 300, Value: "1.1.1.1", Weight: 100, SmartRoutingType: SmartRoutingLatency, LatencyZone: "DE"},
			{Name: "api", Type: RecordTypeA, TTLSeconds: 300, Value: "2.2.2.2", Weight: 100, SmartRoutingType: SmartRoutingLatency, LatencyZone: "US"},
		},
	}
	eps := aggregateRecords(zone)
	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints (one per zone), got %d", len(eps))
	}
	ids := map[string]bool{}
	for _, ep := range eps {
		ids[ep.SetIdentifier] = true
	}
	if !ids["latency:DE"] || !ids["latency:US"] {
		t.Errorf("SetIdentifiers: got %v want [latency:DE latency:US]", ids)
	}
}

func TestAggregateRecords_SmartLatencySameZoneAggregates(t *testing.T) {
	zone := &Zone{
		Domain: "example.com",
		Records: []*Record{
			{Name: "api", Type: RecordTypeA, TTLSeconds: 300, Value: "1.1.1.1", Weight: 100, SmartRoutingType: SmartRoutingLatency, LatencyZone: "DE"},
			{Name: "api", Type: RecordTypeA, TTLSeconds: 300, Value: "2.2.2.2", Weight: 100, SmartRoutingType: SmartRoutingLatency, LatencyZone: "DE"},
		},
	}
	eps := aggregateRecords(zone)
	if len(eps) != 1 {
		t.Fatalf("expected 1 endpoint (same zone collapses), got %d", len(eps))
	}
	if eps[0].SetIdentifier != "latency:DE" {
		t.Errorf("SetIdentifier: got %q want latency:DE", eps[0].SetIdentifier)
	}
	if len(eps[0].Targets) != 2 {
		t.Errorf("Targets: got %v want 2 targets", eps[0].Targets)
	}
}

func TestAggregateRecords_SmartGeo(t *testing.T) {
	lat1, lng1 := 50.11, 8.68
	lat2, lng2 := 38.13, -78.45
	zone := &Zone{
		Domain: "example.com",
		Records: []*Record{
			{Name: "api", Type: RecordTypeA, TTLSeconds: 300, Value: "1.1.1.1", Weight: 100, SmartRoutingType: SmartRoutingGeolocation, GeolocationLatitude: &lat1, GeolocationLongitude: &lng1},
			{Name: "api", Type: RecordTypeA, TTLSeconds: 300, Value: "2.2.2.2", Weight: 100, SmartRoutingType: SmartRoutingGeolocation, GeolocationLatitude: &lat2, GeolocationLongitude: &lng2},
		},
	}
	eps := aggregateRecords(zone)
	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints (one per coord pair), got %d", len(eps))
	}
	ids := map[string]bool{}
	for _, ep := range eps {
		ids[ep.SetIdentifier] = true
	}
	if !ids["geo:50.1100,8.6800"] || !ids["geo:38.1300,-78.4500"] {
		t.Errorf("SetIdentifiers: got %v", ids)
	}
}

func TestAggregateRecords_MixedSmartAndNonSmart(t *testing.T) {
	zone := &Zone{
		Domain: "example.com",
		Records: []*Record{
			{Name: "api", Type: RecordTypeA, TTLSeconds: 300, Value: "1.1.1.1", Weight: 100, SmartRoutingType: SmartRoutingNone},
			{Name: "api", Type: RecordTypeA, TTLSeconds: 300, Value: "2.2.2.2", Weight: 100, SmartRoutingType: SmartRoutingLatency, LatencyZone: "DE"},
		},
	}
	eps := aggregateRecords(zone)
	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(eps))
	}
	var nonSmart, smart *endpoint.Endpoint
	for _, ep := range eps {
		if ep.SetIdentifier == "" {
			nonSmart = ep
		} else {
			smart = ep
		}
	}
	if nonSmart == nil || nonSmart.Targets[0] != "1.1.1.1" {
		t.Errorf("non-smart endpoint missing or wrong: %v", nonSmart)
	}
	if smart == nil || smart.SetIdentifier != "latency:DE" || smart.Targets[0] != "2.2.2.2" {
		t.Errorf("smart endpoint missing or wrong: %v", smart)
	}
}

func TestAggregateRecords_UsesCommentAsSetIdentifier(t *testing.T) {
	// Comment is set on write to preserve user-provided SetIdentifier.
	// On read, aggregateRecords must surface it as ep.SetIdentifier.
	zone := &Zone{
		Domain: "example.com",
		Records: []*Record{
			{Name: "api", Type: RecordTypeA, TTLSeconds: 300, Value: "1.1.1.1", Weight: 100,
				Comment:          "gc-eu-hetzner-fsn1-0",
				SmartRoutingType: SmartRoutingLatency, LatencyZone: "DE"},
		},
	}
	eps := aggregateRecords(zone)
	if len(eps) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(eps))
	}
	if eps[0].SetIdentifier != "gc-eu-hetzner-fsn1-0" {
		t.Errorf("SetIdentifier: got %q want gc-eu-hetzner-fsn1-0 (Comment wins over smart discriminator)", eps[0].SetIdentifier)
	}
}

func TestAggregateRecords_FallsBackToSmartDiscriminatorWhenCommentEmpty(t *testing.T) {
	// For records predating the Comment-based SetIdentifier (e.g. dashboard-created)
	// the smart discriminator is still used as the SetIdentifier.
	zone := &Zone{
		Domain: "example.com",
		Records: []*Record{
			{Name: "api", Type: RecordTypeA, TTLSeconds: 300, Value: "1.1.1.1", Weight: 100,
				Comment:          "",
				SmartRoutingType: SmartRoutingLatency, LatencyZone: "DE"},
		},
	}
	eps := aggregateRecords(zone)
	if len(eps) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(eps))
	}
	if eps[0].SetIdentifier != "latency:DE" {
		t.Errorf("SetIdentifier fallback: got %q want latency:DE", eps[0].SetIdentifier)
	}
}

func TestAggregateRecords_CommentGroupsIndependentlyFromSmartSettings(t *testing.T) {
	// Two records with identical smart settings but different user-provided
	// SetIdentifiers (Comments) must produce two endpoints, not one.
	zone := &Zone{
		Domain: "example.com",
		Records: []*Record{
			{Name: "api", Type: RecordTypeA, TTLSeconds: 300, Value: "1.1.1.1", Weight: 100,
				Comment: "cluster-a", SmartRoutingType: SmartRoutingLatency, LatencyZone: "DE"},
			{Name: "api", Type: RecordTypeA, TTLSeconds: 300, Value: "2.2.2.2", Weight: 100,
				Comment: "cluster-b", SmartRoutingType: SmartRoutingLatency, LatencyZone: "DE"},
		},
	}
	eps := aggregateRecords(zone)
	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints (different Comments), got %d", len(eps))
	}
	ids := map[string]bool{}
	for _, ep := range eps {
		ids[ep.SetIdentifier] = true
	}
	if !ids["cluster-a"] || !ids["cluster-b"] {
		t.Errorf("expected both cluster-a and cluster-b, got %v", ids)
	}
}
