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
