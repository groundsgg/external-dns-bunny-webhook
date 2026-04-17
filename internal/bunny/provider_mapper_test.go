package bunny

import "testing"

func TestRecordToEndpoint(t *testing.T) {
	r := &Record{
		Name:        "api",
		Type:        RecordTypeA,
		TTLSeconds:  300,
		Value:       "1.2.3.4",
		Weight:      100,
		MonitorType: MonitorTypeNone,
		Disabled:    false,
	}
	ep := recordToEndpoint("example.com", r)

	if ep.DNSName != "api.example.com" {
		t.Errorf("DNSName: got %q want api.example.com", ep.DNSName)
	}
	if ep.RecordType != "A" {
		t.Errorf("RecordType: got %q want A", ep.RecordType)
	}
	if ep.RecordTTL != 300 {
		t.Errorf("RecordTTL: got %d want 300", ep.RecordTTL)
	}
	if len(ep.Targets) != 1 || ep.Targets[0] != "1.2.3.4" {
		t.Errorf("Targets: got %v want [1.2.3.4]", ep.Targets)
	}
}
