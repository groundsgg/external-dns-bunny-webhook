package bunny

import (
	"strconv"
	"testing"

	"sigs.k8s.io/external-dns/endpoint"
)

func TestProviderSpecificOptionsFromEndpoint_Defaults(t *testing.T) {
	ep := &endpoint.Endpoint{}
	opts := providerSpecificOptionsFromEndpoint(ep)
	if opts.Weight != 100 {
		t.Errorf("default weight: got %d want 100", opts.Weight)
	}
	if opts.Disabled {
		t.Errorf("default disabled: got true want false")
	}
	if opts.MonitorType != MonitorTypeNone {
		t.Errorf("default monitor: got %v want none", opts.MonitorType)
	}
}

func TestProviderSpecificOptionsFromEndpoint_WeightClamping(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"50", 50},
		{"1", 1},
		{"100", 100},
		{"0", 1},
		{"-5", 1},
		{"150", 100},
		{"abc", 100},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			ep := &endpoint.Endpoint{}
			ep.WithProviderSpecific(providerSpecificWeight, tc.in)
			opts := providerSpecificOptionsFromEndpoint(ep)
			if opts.Weight != tc.want {
				t.Fatalf("weight=%q: got %d want %d", tc.in, opts.Weight, tc.want)
			}
		})
	}
}

func TestProviderSpecificOptionsFromEndpoint_DisabledAndMonitor(t *testing.T) {
	ep := &endpoint.Endpoint{}
	ep.WithProviderSpecific(providerSpecificDisabled, "true")
	ep.WithProviderSpecific(providerSpecificMonitorType, "http")
	opts := providerSpecificOptionsFromEndpoint(ep)
	if !opts.Disabled {
		t.Error("disabled: got false want true")
	}
	if opts.MonitorType != MonitorTypeHTTP {
		t.Errorf("monitor: got %v want http", opts.MonitorType)
	}
}

func TestProviderSpecificOptionsFromRecord(t *testing.T) {
	r := &Record{
		Disabled:    true,
		MonitorType: MonitorTypePing,
		Weight:      42,
	}
	opts := providerSpecificOptionsFromRecord(r)
	if !opts.Disabled || opts.MonitorType != MonitorTypePing || opts.Weight != 42 {
		t.Fatalf("unexpected opts: %+v", opts)
	}
}

func TestApplyToEndpoint_RoundTrip(t *testing.T) {
	src := &providerSpecificOptions{
		Disabled:    true,
		MonitorType: MonitorTypeHTTP,
		Weight:      75,
	}
	ep := &endpoint.Endpoint{}
	src.ApplyToEndpoint(ep)

	got := providerSpecificOptionsFromEndpoint(ep)
	if got.Disabled != src.Disabled || got.MonitorType != src.MonitorType || got.Weight != src.Weight {
		t.Fatalf("round-trip mismatch:\nwrote %+v\nread  %+v", src, got)
	}

	if v, ok := ep.GetProviderSpecificProperty(providerSpecificWeight); !ok || v != strconv.Itoa(src.Weight) {
		t.Errorf("weight property: got %q ok=%v", v, ok)
	}
}

func TestProviderSpecificOptionsFromEndpoint_SmartNone(t *testing.T) {
	ep := &endpoint.Endpoint{}
	ep.WithProviderSpecific(providerSpecificSmartType, "none")
	opts := providerSpecificOptionsFromEndpoint(ep)
	if opts.SmartType != SmartRoutingNone {
		t.Errorf("SmartType: got %v want none", opts.SmartType)
	}
}

func TestProviderSpecificOptionsFromEndpoint_SmartLatencyValid(t *testing.T) {
	ep := &endpoint.Endpoint{}
	ep.WithProviderSpecific(providerSpecificSmartType, "latency")
	ep.WithProviderSpecific(providerSpecificSmartLatencyZone, "DE")
	opts := providerSpecificOptionsFromEndpoint(ep)
	if opts.SmartType != SmartRoutingLatency {
		t.Errorf("SmartType: got %v want latency", opts.SmartType)
	}
	if opts.LatencyZone != "DE" {
		t.Errorf("LatencyZone: got %q want DE", opts.LatencyZone)
	}
}

func TestProviderSpecificOptionsFromEndpoint_SmartLatencyMissingZone(t *testing.T) {
	ep := &endpoint.Endpoint{}
	ep.WithProviderSpecific(providerSpecificSmartType, "latency")
	opts := providerSpecificOptionsFromEndpoint(ep)
	if opts.SmartType != SmartRoutingNone {
		t.Errorf("SmartType without zone should fall back to none, got %v", opts.SmartType)
	}
}

func TestProviderSpecificOptionsFromEndpoint_SmartGeoValid(t *testing.T) {
	ep := &endpoint.Endpoint{}
	ep.WithProviderSpecific(providerSpecificSmartType, "geo")
	ep.WithProviderSpecific(providerSpecificSmartGeoLat, "50.11")
	ep.WithProviderSpecific(providerSpecificSmartGeoLong, "8.68")
	opts := providerSpecificOptionsFromEndpoint(ep)
	if opts.SmartType != SmartRoutingGeolocation {
		t.Errorf("SmartType: got %v want geo", opts.SmartType)
	}
	if opts.GeoLat == nil || *opts.GeoLat != 50.11 {
		t.Errorf("GeoLat: got %v", opts.GeoLat)
	}
	if opts.GeoLong == nil || *opts.GeoLong != 8.68 {
		t.Errorf("GeoLong: got %v", opts.GeoLong)
	}
}

func TestProviderSpecificOptionsFromEndpoint_SmartGeoMissingCoord(t *testing.T) {
	cases := []struct {
		name, lat, long string
	}{
		{"missing both", "", ""},
		{"missing long", "50.11", ""},
		{"missing lat", "", "8.68"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ep := &endpoint.Endpoint{}
			ep.WithProviderSpecific(providerSpecificSmartType, "geo")
			if tc.lat != "" {
				ep.WithProviderSpecific(providerSpecificSmartGeoLat, tc.lat)
			}
			if tc.long != "" {
				ep.WithProviderSpecific(providerSpecificSmartGeoLong, tc.long)
			}
			opts := providerSpecificOptionsFromEndpoint(ep)
			if opts.SmartType != SmartRoutingNone {
				t.Errorf("expected fallback to none, got %v", opts.SmartType)
			}
		})
	}
}

func TestProviderSpecificOptionsFromEndpoint_SmartGeoOutOfRange(t *testing.T) {
	cases := []struct {
		name, lat, long string
	}{
		{"lat too high", "91", "0"},
		{"lat too low", "-91", "0"},
		{"long too high", "0", "181"},
		{"long too low", "0", "-181"},
		{"lat not a number", "abc", "0"},
		{"long not a number", "0", "xyz"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ep := &endpoint.Endpoint{}
			ep.WithProviderSpecific(providerSpecificSmartType, "geo")
			ep.WithProviderSpecific(providerSpecificSmartGeoLat, tc.lat)
			ep.WithProviderSpecific(providerSpecificSmartGeoLong, tc.long)
			opts := providerSpecificOptionsFromEndpoint(ep)
			if opts.SmartType != SmartRoutingNone {
				t.Errorf("expected fallback to none for %s, got %v", tc.name, opts.SmartType)
			}
		})
	}
}

func TestProviderSpecificOptionsFromEndpoint_SmartUnknownType(t *testing.T) {
	ep := &endpoint.Endpoint{}
	ep.WithProviderSpecific(providerSpecificSmartType, "garbage")
	opts := providerSpecificOptionsFromEndpoint(ep)
	if opts.SmartType != SmartRoutingNone {
		t.Errorf("expected none for unknown smart-type, got %v", opts.SmartType)
	}
}

func TestProviderSpecificOptionsFromRecord_Smart(t *testing.T) {
	lat, lng := 50.11, 8.68
	r := &Record{
		MonitorType:          MonitorTypeNone,
		Weight:               100,
		Disabled:             false,
		SmartRoutingType:     SmartRoutingGeolocation,
		GeolocationLatitude:  &lat,
		GeolocationLongitude: &lng,
	}
	opts := providerSpecificOptionsFromRecord(r)
	if opts.SmartType != SmartRoutingGeolocation {
		t.Errorf("SmartType: got %v want geo", opts.SmartType)
	}
	if opts.GeoLat == nil || *opts.GeoLat != 50.11 || opts.GeoLong == nil || *opts.GeoLong != 8.68 {
		t.Errorf("coords: got (%v, %v)", opts.GeoLat, opts.GeoLong)
	}
}

func TestApplyToEndpoint_SmartRoundTrip(t *testing.T) {
	lat, lng := 50.11, 8.68
	src := &providerSpecificOptions{
		Disabled:    false,
		MonitorType: MonitorTypeNone,
		Weight:      100,
		SmartType:   SmartRoutingGeolocation,
		GeoLat:      &lat,
		GeoLong:     &lng,
	}
	ep := &endpoint.Endpoint{}
	src.ApplyToEndpoint(ep)

	got := providerSpecificOptionsFromEndpoint(ep)
	if got.SmartType != SmartRoutingGeolocation {
		t.Errorf("SmartType round-trip: got %v", got.SmartType)
	}
	if got.GeoLat == nil || *got.GeoLat != 50.11 {
		t.Errorf("GeoLat round-trip: got %v", got.GeoLat)
	}
	if got.GeoLong == nil || *got.GeoLong != 8.68 {
		t.Errorf("GeoLong round-trip: got %v", got.GeoLong)
	}
}

func TestProviderSpecificOptionsEqual(t *testing.T) {
	lat, lng := 50.11, 8.68
	lat2, lng2 := 50.11, 8.68
	a := providerSpecificOptions{SmartType: SmartRoutingGeolocation, GeoLat: &lat, GeoLong: &lng, Weight: 100}
	b := providerSpecificOptions{SmartType: SmartRoutingGeolocation, GeoLat: &lat2, GeoLong: &lng2, Weight: 100}
	if !providerSpecificOptionsEqual(a, b) {
		t.Errorf("equal structs with different pointer addresses should compare equal")
	}

	latDiff := 51.0
	c := providerSpecificOptions{SmartType: SmartRoutingGeolocation, GeoLat: &latDiff, GeoLong: &lng, Weight: 100}
	if providerSpecificOptionsEqual(a, c) {
		t.Errorf("different lat should not compare equal")
	}

	d := providerSpecificOptions{SmartType: SmartRoutingGeolocation, GeoLat: nil, GeoLong: &lng, Weight: 100}
	if providerSpecificOptionsEqual(a, d) {
		t.Errorf("nil vs non-nil pointer should not compare equal")
	}

	e := providerSpecificOptions{SmartType: SmartRoutingNone, Weight: 100}
	f := providerSpecificOptions{SmartType: SmartRoutingNone, Weight: 100}
	if !providerSpecificOptionsEqual(e, f) {
		t.Errorf("both nil coords should compare equal")
	}
}
