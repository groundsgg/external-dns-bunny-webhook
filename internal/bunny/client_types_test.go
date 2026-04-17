package bunny

import "testing"

func TestRecordType_String(t *testing.T) {
	cases := []struct {
		in   RecordType
		want string
	}{
		{RecordTypeA, "A"},
		{RecordTypeAAAA, "AAAA"},
		{RecordTypeCNAME, "CNAME"},
		{RecordTypeTXT, "TXT"},
		{RecordTypeMX, "MX"},
		{RecordTypeRDR, "RDR"},
		{RecordTypeFlatten, "FLATTEN"},
		{RecordTypePZ, "PZ"},
		{RecordTypeSRV, "SRV"},
		{RecordTypeCAA, "CAA"},
		{RecordTypePTR, "PTR"},
		{RecordTypeSCR, "SCR"},
		{RecordTypeNS, "NS"},
		{RecordType(-1), "?"},
		{RecordType(99), "?"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.in.String(); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestMonitorType_String(t *testing.T) {
	cases := []struct {
		in   MonitorType
		want string
	}{
		{MonitorTypeNone, "none"},
		{MonitorTypePing, "ping"},
		{MonitorTypeHTTP, "http"},
		{MonitorType(-1), "none"},
		{MonitorType(99), "none"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.in.String(); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestRecordTypeFromString(t *testing.T) {
	cases := []struct {
		in     string
		want   RecordType
		wantOK bool
	}{
		{"A", RecordTypeA, true},
		{"AAAA", RecordTypeAAAA, true},
		{"CNAME", RecordTypeCNAME, true},
		{"NS", RecordTypeNS, true},
		{"XYZ", RecordType(0), false},
		{"", RecordType(0), false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := RecordTypeFromString(tc.in)
			if ok != tc.wantOK {
				t.Errorf("ok: got %v want %v", ok, tc.wantOK)
			}
			if ok && got != tc.want {
				t.Errorf("type: got %v want %v", got, tc.want)
			}
		})
	}
}

func TestMonitorTypeFromString(t *testing.T) {
	cases := []struct {
		in   string
		want MonitorType
	}{
		{"ping", MonitorTypePing},
		{"PING", MonitorTypePing},
		{"http", MonitorTypeHTTP},
		{"HTTP", MonitorTypeHTTP},
		{"none", MonitorTypeNone},
		{"", MonitorTypeNone},
		{"garbage", MonitorTypeNone},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := MonitorTypeFromString(tc.in); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestSmartRoutingType_String(t *testing.T) {
	cases := []struct {
		in   SmartRoutingType
		want string
	}{
		{SmartRoutingNone, "none"},
		{SmartRoutingLatency, "latency"},
		{SmartRoutingGeolocation, "geo"},
		{SmartRoutingType(-1), "none"},
		{SmartRoutingType(99), "none"},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			if got := tc.in.String(); got != tc.want {
				t.Errorf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestSmartRoutingTypeFromString(t *testing.T) {
	cases := []struct {
		in     string
		want   SmartRoutingType
		wantOK bool
	}{
		{"none", SmartRoutingNone, true},
		{"", SmartRoutingNone, true},
		{"latency", SmartRoutingLatency, true},
		{"LATENCY", SmartRoutingLatency, true},
		{"geo", SmartRoutingGeolocation, true},
		{"geolocation", SmartRoutingGeolocation, true},
		{"Geolocation", SmartRoutingGeolocation, true},
		{"garbage", SmartRoutingNone, false},
		{"geography", SmartRoutingNone, false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, ok := SmartRoutingTypeFromString(tc.in)
			if ok != tc.wantOK {
				t.Errorf("ok: got %v want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("value: got %v want %v", got, tc.want)
			}
		})
	}
}
