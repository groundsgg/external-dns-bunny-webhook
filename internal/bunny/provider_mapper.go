package bunny

import (
	"fmt"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/provider"
)

// aggregateRecords groups a zone's records into external-dns endpoints.
// Non-smart records with the same (name, type) aggregate into one endpoint
// with multiple targets, matching the previous behavior. Smart records
// subgroup by smart key — records with identical SmartRoutingType + zone +
// coords collapse into one endpoint (multiple targets), while records with
// different smart settings produce separate endpoints identified by
// SetIdentifier. Unsupported record types are skipped. Apex records
// (Name == "") use the bare zone domain as DNSName.
func aggregateRecords(zone *Zone) []*endpoint.Endpoint {
	type key struct {
		name    string
		typ     string
		smartID string
	}
	groups := map[key][]*Record{}
	order := []key{}

	for _, r := range zone.Records {
		typ := r.Type.String()
		if !provider.SupportedRecordType(typ) {
			continue
		}
		k := key{r.Name, typ, smartRecordDiscriminator(r)}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], r)
	}

	out := make([]*endpoint.Endpoint, 0, len(order))
	for _, k := range order {
		records := groups[k]
		first := records[0]

		dnsName := first.Name + "." + zone.Domain
		if first.Name == "" {
			dnsName = zone.Domain
		}

		targets := make([]string, 0, len(records))
		for _, r := range records {
			targets = append(targets, r.Value)
		}

		ep := endpoint.NewEndpointWithTTL(dnsName, k.typ, endpoint.TTL(first.TTLSeconds), targets...)
		ep.SetIdentifier = k.smartID
		ps := providerSpecificOptionsFromRecord(first)
		ps.ApplyToEndpoint(ep)
		out = append(out, ep)
	}

	return out
}

// smartRecordDiscriminator returns a stable, human-readable key that
// distinguishes smart records with different routing settings for
// external-dns's SetIdentifier. Non-smart records return "" so they
// don't pollute the SetIdentifier field.
func smartRecordDiscriminator(r *Record) string {
	switch r.SmartRoutingType {
	case SmartRoutingLatency:
		return "latency:" + r.LatencyZone
	case SmartRoutingGeolocation:
		if r.GeolocationLatitude != nil && r.GeolocationLongitude != nil {
			return fmt.Sprintf("geo:%.4f,%.4f", *r.GeolocationLatitude, *r.GeolocationLongitude)
		}
	}
	return ""
}
