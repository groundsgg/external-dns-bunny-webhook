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
