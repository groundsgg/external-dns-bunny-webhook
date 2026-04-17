package bunny

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// rewritingTransport redirects all requests to the test server regardless
// of the URL the BunnyClient constructs. This lets us test the real
// production code path (which hardcodes api.bunny.net) without touching it.
type rewritingTransport struct {
	target *url.URL
	rt     http.RoundTripper
}

func (t *rewritingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r.URL.Scheme = t.target.Scheme
	r.URL.Host = t.target.Host
	return t.rt.RoundTrip(r)
}

func newTestClient(t *testing.T, handler http.Handler) Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	httpClient := &http.Client{
		Transport: &rewritingTransport{target: u, rt: http.DefaultTransport},
	}
	return NewDNSClient(httpClient, "test-api-key")
}

func TestClient_ListZones_HappyPathWithSmartRecords(t *testing.T) {
	var gotMethod, gotPath, gotAccessKey, gotAccept string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotAccessKey = r.Header.Get("AccessKey")
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"CurrentPage": 1,
			"TotalItems": 3,
			"HasMoreItems": false,
			"Items": [{
				"Id": 42,
				"Domain": "example.com",
				"Records": [
					{"Id":1,"Type":0,"Name":"plain","Value":"1.1.1.1","Ttl":300,"Weight":100,"MonitorType":0,"Disabled":false,"SmartRoutingType":0},
					{"Id":2,"Type":0,"Name":"lat","Value":"2.2.2.2","Ttl":300,"Weight":100,"MonitorType":0,"Disabled":false,"SmartRoutingType":1,"LatencyZone":"DE"},
					{"Id":3,"Type":0,"Name":"geo","Value":"3.3.3.3","Ttl":300,"Weight":100,"MonitorType":0,"Disabled":false,"SmartRoutingType":2,"GeolocationLatitude":50.11,"GeolocationLongitude":8.68}
				]
			}]
		}`))
	})

	c := newTestClient(t, handler)
	resp, err := c.ListZones(context.Background(), ListZonesRequest{Page: 1})
	if err != nil {
		t.Fatalf("ListZones: %v", err)
	}

	if gotMethod != http.MethodGet {
		t.Errorf("method: got %s want GET", gotMethod)
	}
	if gotPath != "/dnszone" {
		t.Errorf("path: got %s want /dnszone", gotPath)
	}
	if gotAccessKey != "test-api-key" {
		t.Errorf("AccessKey header: got %q want test-api-key", gotAccessKey)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept header: got %q want application/json", gotAccept)
	}

	if len(resp.Items) != 1 || len(resp.Items[0].Records) != 3 {
		t.Fatalf("unexpected response shape: %+v", resp)
	}

	records := resp.Items[0].Records

	if records[0].SmartRoutingType != SmartRoutingNone {
		t.Errorf("record 0 SmartRoutingType: got %v", records[0].SmartRoutingType)
	}

	if records[1].SmartRoutingType != SmartRoutingLatency {
		t.Errorf("record 1 SmartRoutingType: got %v", records[1].SmartRoutingType)
	}
	if records[1].LatencyZone != "DE" {
		t.Errorf("record 1 LatencyZone: got %q want DE", records[1].LatencyZone)
	}

	if records[2].SmartRoutingType != SmartRoutingGeolocation {
		t.Errorf("record 2 SmartRoutingType: got %v", records[2].SmartRoutingType)
	}
	if records[2].GeolocationLatitude == nil || *records[2].GeolocationLatitude != 50.11 {
		t.Errorf("record 2 lat: got %v", records[2].GeolocationLatitude)
	}
	if records[2].GeolocationLongitude == nil || *records[2].GeolocationLongitude != 8.68 {
		t.Errorf("record 2 long: got %v", records[2].GeolocationLongitude)
	}
}

func TestClient_ListZones_PassesPageAndPerPage(t *testing.T) {
	var gotPage, gotPerPage string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPage = r.URL.Query().Get("page")
		gotPerPage = r.URL.Query().Get("perPage")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"CurrentPage":2,"TotalItems":0,"HasMoreItems":false,"Items":[]}`))
	})

	c := newTestClient(t, handler)
	if _, err := c.ListZones(context.Background(), ListZonesRequest{Page: 2, PerPage: 50}); err != nil {
		t.Fatalf("ListZones: %v", err)
	}

	if gotPage != "2" {
		t.Errorf("page query: got %q want 2", gotPage)
	}
	if gotPerPage != "50" {
		t.Errorf("perPage query: got %q want 50", gotPerPage)
	}
}

func TestClient_CreateRecord_NonSmartBodyOmitsSmartFields(t *testing.T) {
	var gotMethod, gotPath, gotContentType string
	var rawBody []byte
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		rawBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"Id":99,"Type":0,"Name":"api","Value":"1.1.1.1","Ttl":300,"Weight":100,"MonitorType":0,"SmartRoutingType":0}`))
	})

	c := newTestClient(t, handler)
	rec, err := c.CreateRecord(context.Background(), "42", CreateRecordRequest{
		Name:        "api",
		Type:        RecordTypeA,
		Value:       "1.1.1.1",
		TTLSeconds:  300,
		MonitorType: MonitorTypeNone,
		Weight:      100,
		Disabled:    false,
	})
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}

	if gotMethod != http.MethodPut {
		t.Errorf("method: got %s want PUT", gotMethod)
	}
	if gotPath != "/dnszone/42/records" {
		t.Errorf("path: got %s", gotPath)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type: got %q", gotContentType)
	}

	body := string(rawBody)
	if strings.Contains(body, "SmartRoutingType") {
		t.Errorf("non-smart body must not include SmartRoutingType (omitempty): %s", body)
	}
	if strings.Contains(body, "LatencyZone") {
		t.Errorf("non-smart body must not include LatencyZone (omitempty): %s", body)
	}
	if strings.Contains(body, "Geolocation") {
		t.Errorf("non-smart body must not include Geolocation fields (omitempty): %s", body)
	}

	if rec.ID != 99 {
		t.Errorf("returned record ID: got %d want 99", rec.ID)
	}
}

func TestClient_CreateRecord_SmartGeoBodyCarriesCoords(t *testing.T) {
	var rawBody []byte
	var decoded CreateRecordRequest
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		_ = json.Unmarshal(rawBody, &decoded)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"Id":1,"Type":0,"Name":"api","Value":"1.1.1.1","Ttl":300,"SmartRoutingType":2,"GeolocationLatitude":50.11,"GeolocationLongitude":8.68}`))
	})

	c := newTestClient(t, handler)
	lat, lng := 50.11, 8.68
	_, err := c.CreateRecord(context.Background(), "42", CreateRecordRequest{
		Name:                 "api",
		Type:                 RecordTypeA,
		Value:                "1.1.1.1",
		TTLSeconds:           300,
		SmartRoutingType:     SmartRoutingGeolocation,
		GeolocationLatitude:  &lat,
		GeolocationLongitude: &lng,
	})
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}

	if decoded.SmartRoutingType != SmartRoutingGeolocation {
		t.Errorf("decoded SmartRoutingType: got %v want 2", decoded.SmartRoutingType)
	}
	if decoded.GeolocationLatitude == nil || *decoded.GeolocationLatitude != 50.11 {
		t.Errorf("decoded lat: got %v", decoded.GeolocationLatitude)
	}
	if decoded.GeolocationLongitude == nil || *decoded.GeolocationLongitude != 8.68 {
		t.Errorf("decoded long: got %v", decoded.GeolocationLongitude)
	}

	if !strings.Contains(string(rawBody), `"SmartRoutingType":2`) {
		t.Errorf("body should encode SmartRoutingType as int 2: %s", rawBody)
	}
}

func TestClient_CreateRecord_SmartLatencyBodyCarriesZone(t *testing.T) {
	var rawBody []byte
	var decoded CreateRecordRequest
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ = io.ReadAll(r.Body)
		_ = json.Unmarshal(rawBody, &decoded)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"Id":1,"Type":0,"Name":"api","Value":"1.1.1.1","Ttl":300,"SmartRoutingType":1,"LatencyZone":"DE"}`))
	})

	c := newTestClient(t, handler)
	_, err := c.CreateRecord(context.Background(), "42", CreateRecordRequest{
		Name:             "api",
		Type:             RecordTypeA,
		Value:            "1.1.1.1",
		TTLSeconds:       300,
		SmartRoutingType: SmartRoutingLatency,
		LatencyZone:      "DE",
	})
	if err != nil {
		t.Fatalf("CreateRecord: %v", err)
	}

	if decoded.SmartRoutingType != SmartRoutingLatency {
		t.Errorf("decoded SmartRoutingType: got %v want 1", decoded.SmartRoutingType)
	}
	if decoded.LatencyZone != "DE" {
		t.Errorf("decoded LatencyZone: got %q want DE", decoded.LatencyZone)
	}

	if !strings.Contains(string(rawBody), `"SmartRoutingType":1`) {
		t.Errorf("body should encode SmartRoutingType as int 1: %s", rawBody)
	}
	if strings.Contains(string(rawBody), "Geolocation") {
		t.Errorf("latency body must not include Geolocation fields: %s", rawBody)
	}
}

func TestClient_UpdateRecord_UsesPOSTAndCorrectPath(t *testing.T) {
	var gotMethod, gotPath string
	var decoded UpdateRecordRequest
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &decoded)
		w.WriteHeader(http.StatusNoContent)
	})

	c := newTestClient(t, handler)
	err := c.UpdateRecord(context.Background(), 42, 7, UpdateRecordRequest{
		Value:       "2.2.2.2",
		TTLSeconds:  600,
		MonitorType: MonitorTypeHTTP,
		Weight:      50,
		Disabled:    true,
	})
	if err != nil {
		t.Fatalf("UpdateRecord: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method: got %s want POST", gotMethod)
	}
	if gotPath != "/dnszone/42/records/7" {
		t.Errorf("path: got %s want /dnszone/42/records/7", gotPath)
	}
	if decoded.Value != "2.2.2.2" || decoded.TTLSeconds != 600 || decoded.Weight != 50 || decoded.Disabled != true {
		t.Errorf("decoded body: %+v", decoded)
	}
	if decoded.MonitorType != MonitorTypeHTTP {
		t.Errorf("MonitorType: got %v want http", decoded.MonitorType)
	}
}

func TestClient_UpdateRecord_SmartGeoRoundTrip(t *testing.T) {
	var decoded UpdateRecordRequest
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &decoded)
		w.WriteHeader(http.StatusNoContent)
	})

	c := newTestClient(t, handler)
	lat, lng := 38.13, -78.45
	err := c.UpdateRecord(context.Background(), 42, 7, UpdateRecordRequest{
		Value:                "1.1.1.1",
		TTLSeconds:           300,
		SmartRoutingType:     SmartRoutingGeolocation,
		GeolocationLatitude:  &lat,
		GeolocationLongitude: &lng,
	})
	if err != nil {
		t.Fatalf("UpdateRecord: %v", err)
	}

	if decoded.SmartRoutingType != SmartRoutingGeolocation {
		t.Errorf("SmartRoutingType: got %v", decoded.SmartRoutingType)
	}
	if decoded.GeolocationLatitude == nil || *decoded.GeolocationLatitude != 38.13 {
		t.Errorf("lat: got %v", decoded.GeolocationLatitude)
	}
	if decoded.GeolocationLongitude == nil || *decoded.GeolocationLongitude != -78.45 {
		t.Errorf("long: got %v", decoded.GeolocationLongitude)
	}
}

func TestClient_DeleteRecord_UsesDELETEAndCorrectPath(t *testing.T) {
	var gotMethod, gotPath string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})

	c := newTestClient(t, handler)
	err := c.DeleteRecord(context.Background(), 42, 7)
	if err != nil {
		t.Fatalf("DeleteRecord: %v", err)
	}

	if gotMethod != http.MethodDelete {
		t.Errorf("method: got %s want DELETE", gotMethod)
	}
	if gotPath != "/dnszone/42/records/7" {
		t.Errorf("path: got %s want /dnszone/42/records/7", gotPath)
	}
}

func TestClient_CreateRecord_4xxReturnsWrappedError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"Message":"Invalid record"}`))
	})

	c := newTestClient(t, handler)
	_, err := c.CreateRecord(context.Background(), "42", CreateRecordRequest{
		Name:  "api",
		Type:  RecordTypeA,
		Value: "1.1.1.1",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "400") {
		t.Errorf("error should mention 400: %q", msg)
	}
}
