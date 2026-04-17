package bunny

import "strings"

type RecordType int

const (
	RecordTypeA RecordType = iota
	RecordTypeAAAA
	RecordTypeCNAME
	RecordTypeTXT
	RecordTypeMX
	RecordTypeRDR
	RecordTypeFlatten
	RecordTypePZ
	RecordTypeSRV
	RecordTypeCAA
	RecordTypePTR
	RecordTypeSCR
	RecordTypeNS
)

func (r RecordType) String() string {
	if r < RecordTypeA || r > RecordTypeNS {
		return "?"
	}

	return [...]string{"A", "AAAA", "CNAME", "TXT", "MX", "RDR", "FLATTEN", "PZ", "SRV", "CAA", "PTR", "SCR", "NS"}[r]
}

func RecordTypeFromString(s string) (RecordType, bool) {
	switch s {
	case "A":
		return RecordTypeA, true
	case "AAAA":
		return RecordTypeAAAA, true
	case "CNAME":
		return RecordTypeCNAME, true
	case "TXT":
		return RecordTypeTXT, true
	case "MX":
		return RecordTypeMX, true
	case "RDR":
		return RecordTypeRDR, true
	case "FLATTEN":
		return RecordTypeFlatten, true
	case "PZ":
		return RecordTypePZ, true
	case "SRV":
		return RecordTypeSRV, true
	case "CAA":
		return RecordTypeCAA, true
	case "PTR":
		return RecordTypePTR, true
	case "SCR":
		return RecordTypeSCR, true
	case "NS":
		return RecordTypeNS, true
	}
	return RecordType(0), false
}

// MonitorType is an enum for the type of monitor attached to a record.
type MonitorType int

const (
	MonitorTypeNone MonitorType = iota
	MonitorTypePing
	MonitorTypeHTTP
)

func (m MonitorType) String() string {
	if m < MonitorTypeNone || m > MonitorTypeHTTP {
		return "none"
	}
	return [...]string{"none", "ping", "http"}[m]
}

func MonitorTypeFromString(s string) MonitorType {
	switch strings.ToLower(s) {
	case "ping":
		return MonitorTypePing
	case "http":
		return MonitorTypeHTTP
	default:
		return MonitorTypeNone
	}
}

type Record struct {
	ID                    int64       `json:"Id"`
	Type                  RecordType  `json:"Type"`
	TTLSeconds            int         `json:"Ttl"`
	Value                 string      `json:"Value"`
	Name                  string      `json:"Name"`
	Weight                int         `json:"Weight"`
	Priority              int         `json:"Priority"`
	Port                  int         `json:"Port"`
	Flags                 int         `json:"Flags"`
	Tag                   string      `json:"Tag"`
	MonitorType           MonitorType `json:"MonitorType"`
	Accelerated           bool        `json:"Accelerated"`
	AcceleratedPullZoneID int64       `json:"AcceleratedPullZoneId"`
	LinkName              string      `json:"LinkName"`
	Disabled              bool        `json:"Disabled"`
	Comment               string      `json:"Comment"`
}

type Zone struct {
	ID      int64     `json:"Id"`
	Domain  string    `json:"Domain"`
	Records []*Record `json:"Records"`
}
