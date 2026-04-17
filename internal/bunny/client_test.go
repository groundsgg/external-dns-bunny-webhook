package bunny

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestRedactAuthHeaders(t *testing.T) {
	in := http.Header{}
	in.Set("AccessKey", "secret-token")
	in.Set("Authorization", "Bearer secret")
	in.Set("X-Trace-Id", "abc123")

	out := redactAuthHeaders(in)
	if out.Get("AccessKey") != "[REDACTED]" {
		t.Errorf("AccessKey not redacted: %q", out.Get("AccessKey"))
	}
	if out.Get("Authorization") != "[REDACTED]" {
		t.Errorf("Authorization not redacted: %q", out.Get("Authorization"))
	}
	if out.Get("X-Trace-Id") != "abc123" {
		t.Errorf("non-auth header should pass through: %q", out.Get("X-Trace-Id"))
	}
	if in.Get("AccessKey") != "secret-token" {
		t.Error("input header was mutated")
	}
}

func TestCreateRecordRequest_JSONRoundTripsSmartFields(t *testing.T) {
	lat, lng := 50.11, 8.68
	in := CreateRecordRequest{
		Name:                 "api",
		Type:                 RecordTypeA,
		Value:                "1.1.1.1",
		TTLSeconds:           300,
		SmartRoutingType:     SmartRoutingGeolocation,
		GeolocationLatitude:  &lat,
		GeolocationLongitude: &lng,
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out CreateRecordRequest
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.SmartRoutingType != SmartRoutingGeolocation {
		t.Errorf("SmartRoutingType: got %v", out.SmartRoutingType)
	}
	if out.GeolocationLatitude == nil || *out.GeolocationLatitude != 50.11 {
		t.Errorf("lat: got %v", out.GeolocationLatitude)
	}
	if out.GeolocationLongitude == nil || *out.GeolocationLongitude != 8.68 {
		t.Errorf("long: got %v", out.GeolocationLongitude)
	}
}

func TestUpdateRecordRequest_JSONRoundTripsLatencyZone(t *testing.T) {
	in := UpdateRecordRequest{
		Value:            "1.1.1.1",
		TTLSeconds:       300,
		SmartRoutingType: SmartRoutingLatency,
		LatencyZone:      "DE",
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out UpdateRecordRequest
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.SmartRoutingType != SmartRoutingLatency {
		t.Errorf("SmartRoutingType: got %v", out.SmartRoutingType)
	}
	if out.LatencyZone != "DE" {
		t.Errorf("LatencyZone: got %q", out.LatencyZone)
	}
}
