package bunny

import (
	"log/slog"
	"strconv"

	"sigs.k8s.io/external-dns/endpoint"
)

const (
	providerSpecificDisabled         = "webhook/bunny-disabled"
	providerSpecificMonitorType      = "webhook/bunny-monitor-type"
	providerSpecificWeight           = "webhook/bunny-weight"
	providerSpecificSmartType        = "webhook/bunny-smart-type"
	providerSpecificSmartLatencyZone = "webhook/bunny-smart-latency-zone"
	providerSpecificSmartGeoLat      = "webhook/bunny-smart-geo-lat"
	providerSpecificSmartGeoLong     = "webhook/bunny-smart-geo-long"

	// defaultWeight mirrors Bunny's default record weight. Used both as
	// the zero-value fallback in parsing and as the "don't emit if
	// unchanged" threshold in serialization.
	defaultWeight = 100
)

type providerSpecificOptions struct {
	Disabled    bool
	MonitorType MonitorType
	Weight      int
	SmartType   SmartRoutingType
	LatencyZone string
	GeoLat      *float64
	GeoLong     *float64
}

func providerSpecificOptionsFromEndpoint(e *endpoint.Endpoint) providerSpecificOptions {
	opts := providerSpecificOptions{}

	if disabled, ok := e.GetProviderSpecificProperty(providerSpecificDisabled); ok {
		var err error
		opts.Disabled, err = strconv.ParseBool(disabled)
		if err != nil {
			opts.Disabled = false
		}
	}

	if monitorType, ok := e.GetProviderSpecificProperty(providerSpecificMonitorType); ok {
		opts.MonitorType = MonitorTypeFromString(monitorType)
	}

	if weight, ok := e.GetProviderSpecificProperty(providerSpecificWeight); ok {
		var err error
		opts.Weight, err = strconv.Atoi(weight)
		if err != nil {
			opts.Weight = defaultWeight
		}

		if opts.Weight < 1 {
			opts.Weight = 1
		}

		if opts.Weight > defaultWeight {
			opts.Weight = defaultWeight
		}
	}

	if opts.Weight == 0 {
		opts.Weight = defaultWeight
	}

	opts.SmartType, opts.LatencyZone, opts.GeoLat, opts.GeoLong = parseSmartRouting(e)

	return opts
}

// parseSmartRouting inspects the smart-routing ProviderSpecific keys on an
// endpoint. Invalid or inconsistent configurations (unknown type, latency
// without a zone, geo without coords, out-of-range coords) fall back to
// SmartRoutingNone with a warn log. The goal is to never break reconcile
// because of a typo — the basic DNS record still gets created.
func parseSmartRouting(e *endpoint.Endpoint) (SmartRoutingType, string, *float64, *float64) {
	raw, ok := e.GetProviderSpecificProperty(providerSpecificSmartType)
	if !ok {
		return SmartRoutingNone, "", nil, nil
	}

	st, recognized := SmartRoutingTypeFromString(raw)
	if !recognized {
		slog.Warn("Unknown smart-type annotation — falling back to none",
			slog.String("dns_name", e.DNSName),
			slog.String("smart_type", raw))
		return SmartRoutingNone, "", nil, nil
	}

	switch st {
	case SmartRoutingNone:
		return SmartRoutingNone, "", nil, nil

	case SmartRoutingLatency:
		zone, ok := e.GetProviderSpecificProperty(providerSpecificSmartLatencyZone)
		if !ok || zone == "" {
			slog.Warn("smart-type=latency but no latency-zone — falling back to none",
				slog.String("dns_name", e.DNSName))
			return SmartRoutingNone, "", nil, nil
		}
		return SmartRoutingLatency, zone, nil, nil

	case SmartRoutingGeolocation:
		lat, latOK := parseCoord(e, providerSpecificSmartGeoLat, -90, 90)
		lng, lngOK := parseCoord(e, providerSpecificSmartGeoLong, -180, 180)
		if !latOK || !lngOK {
			slog.Warn("smart-type=geo but lat/long missing or invalid — falling back to none",
				slog.String("dns_name", e.DNSName))
			return SmartRoutingNone, "", nil, nil
		}
		return SmartRoutingGeolocation, "", &lat, &lng
	}

	return SmartRoutingNone, "", nil, nil
}

func parseCoord(e *endpoint.Endpoint, key string, min, max float64) (float64, bool) {
	raw, ok := e.GetProviderSpecificProperty(key)
	if !ok || raw == "" {
		return 0, false
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	if v < min || v > max {
		return 0, false
	}
	return v, true
}

func providerSpecificOptionsFromRecord(r *Record) *providerSpecificOptions {
	opts := &providerSpecificOptions{
		MonitorType: r.MonitorType,
		Weight:      r.Weight,
		Disabled:    r.Disabled,
		SmartType:   r.SmartRoutingType,
		LatencyZone: r.LatencyZone,
		GeoLat:      r.GeolocationLatitude,
		GeoLong:     r.GeolocationLongitude,
	}

	return opts
}

func (p *providerSpecificOptions) ApplyToEndpoint(e *endpoint.Endpoint) {
	// Only emit non-default values — otherwise external-dns's planner sees
	// a permanent diff between the user's source (no property set) and the
	// webhook's round-trip (property set to its default) and issues
	// redundant updates on every reconcile.
	if p.MonitorType != MonitorTypeNone {
		e.WithProviderSpecific(providerSpecificMonitorType, p.MonitorType.String())
	}
	if p.Weight != defaultWeight {
		e.WithProviderSpecific(providerSpecificWeight, strconv.Itoa(p.Weight))
	}
	if p.Disabled {
		e.WithProviderSpecific(providerSpecificDisabled, strconv.FormatBool(p.Disabled))
	}

	// Smart keys are only emitted when set so non-smart endpoints stay clean.
	if p.SmartType != SmartRoutingNone {
		e.WithProviderSpecific(providerSpecificSmartType, p.SmartType.String())
	}
	if p.LatencyZone != "" {
		e.WithProviderSpecific(providerSpecificSmartLatencyZone, p.LatencyZone)
	}
	if p.GeoLat != nil {
		e.WithProviderSpecific(providerSpecificSmartGeoLat, strconv.FormatFloat(*p.GeoLat, 'f', -1, 64))
	}
	if p.GeoLong != nil {
		e.WithProviderSpecific(providerSpecificSmartGeoLong, strconv.FormatFloat(*p.GeoLong, 'f', -1, 64))
	}
}

// providerSpecificOptionsEqual compares two options by value, dereferencing
// the coordinate pointers. Go's struct equality compares pointer addresses,
// which is wrong here.
func providerSpecificOptionsEqual(a, b providerSpecificOptions) bool {
	if a.Disabled != b.Disabled ||
		a.MonitorType != b.MonitorType ||
		a.Weight != b.Weight ||
		a.SmartType != b.SmartType ||
		a.LatencyZone != b.LatencyZone {
		return false
	}
	return floatPtrEqual(a.GeoLat, b.GeoLat) && floatPtrEqual(a.GeoLong, b.GeoLong)
}

func floatPtrEqual(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
