package bunny

import (
	"sort"
	"testing"
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
