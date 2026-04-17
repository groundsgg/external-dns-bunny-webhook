package bunny

import (
	"fmt"

	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/provider"
)

// aggregateRecords groups a zone's records into external-dns endpoints.
// Records with the same (name, type, set-identifier) aggregate into one
// endpoint with multiple targets. The set-identifier is taken from the
// Bunny record's Comment field when set (preserves user-provided
// SetIdentifier across reconciles); otherwise it falls back to the smart
// routing discriminator for records that were created before this fix or
// added directly via the Bunny dashboard. Unsupported record types are
// skipped. Apex records (Name == "") use the bare zone domain as DNSName.
func aggregateRecords(zone *Zone) []*endpoint.Endpoint {
	type key struct {
		name          string
		typ           string
		setIdentifier string
	}
	groups := map[key][]*Record{}
	order := []key{}

	for _, r := range zone.Records {
		typ := r.Type.String()
		if !provider.SupportedRecordType(typ) {
			continue
		}
		k := key{r.Name, typ, recordSetIdentifier(r)}
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
		ep.SetIdentifier = k.setIdentifier
		ps := providerSpecificOptionsFromRecord(first)
		ps.ApplyToEndpoint(ep)
		out = append(out, ep)
	}

	return out
}

// recordSetIdentifier picks the string to use for an endpoint's
// SetIdentifier. Prefers the Bunny record's Comment field (where we store
// the user's SetIdentifier on write) and falls back to smartRecordDiscriminator
// for records that predate that convention.
func recordSetIdentifier(r *Record) string {
	if r.Comment != "" {
		return r.Comment
	}
	return smartRecordDiscriminator(r)
}

// smartRecordDiscriminator returns a stable, human-readable key that
// distinguishes smart records with different routing settings. Used as a
// fallback SetIdentifier for records without a Comment. Non-smart records
// return "" so they don't pollute the SetIdentifier field.
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
