package bunny

import (
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/provider"
)

// aggregateRecords groups a zone's records by (name, type) and returns one
// endpoint per group with all values joined as targets. Unsupported record
// types are skipped. Apex records (Name == "") use the bare zone name as
// the DNSName.
func aggregateRecords(zone *Zone) []*endpoint.Endpoint {
	type key struct {
		name string
		typ  string
	}
	groups := map[key][]*Record{}
	order := []key{}

	for _, r := range zone.Records {
		typ := r.Type.String()
		if !provider.SupportedRecordType(typ) {
			continue
		}
		k := key{r.Name, typ}
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
		ps := providerSpecificOptionsFromRecord(first)
		ps.ApplyToEndpoint(ep)
		out = append(out, ep)
	}

	return out
}
