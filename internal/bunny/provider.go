package bunny

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/puzpuzpuz/xsync/v3"
	"github.com/samber/oops"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

var (
	_ provider.Provider = (*Provider)(nil)
)

const (
	providerValueZoneId   = "BunnyZoneID"
	providerValueRecordId = "BunnyRecordID"
)

type Options struct {
	APIKey               string   `env:"API_KEY, required"`
	DryRun               bool     `env:"DRY_RUN, default=false"`
	ExcludeDomains       []string `env:"EXCLUDE_DOMAINS"`
	ExcludeDomainsRegexp string   `env:"EXCLUDE_DOMAINS_REGEXP"`
	IncludeDomains       []string `env:"INCLUDE_DOMAINS"`
	IncludeDomainsRegexp string   `env:"INCLUDE_DOMAINS_REGEXP"`
}

type Provider struct {
	Options Options
	client  Client
	filter  endpoint.DomainFilterInterface
	zoneMap *xsync.MapOf[string, int64]
}

func NewProvider(client Client, options Options) *Provider {
	provider := &Provider{
		Options: options,
		client:  client,
		filter:  getDomainFilter(options),
		zoneMap: xsync.NewMapOf[string, int64](),
	}

	// On startup, fetch zones so that all available zones are cached. This
	// is necessary to avoid making a call to the API during creates as we
	// need the zone ID to create a record. In addition, this data is used
	// to accurately exctract recordName from the full dnsName. Without it,
	// we could not accurately handle all the expected TLDs without maintaing
	// an internal list.
	_, err := provider.fetchZones(context.Background())
	if err != nil {
		slog.Error("Failed to fetch zones on startup.",
			slog.Any("error", err))
	}

	return provider
}

func (p *Provider) allZones() []string {
	var zones []string

	p.zoneMap.Range(func(key string, value int64) bool {
		zones = append(zones, key)
		return true
	})

	return zones
}

func (p *Provider) cacheZone(zone *Zone) {
	p.zoneMap.Store(zone.Domain, zone.ID)
}

func (p *Provider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	errs := oops.In("Provider").Span("Records")

	zones, err := p.fetchZones(ctx)
	if err != nil {
		slog.Error("Failed to fetch zones", slog.Any("error", err))
		return nil, errs.Wrapf(err, "failed to fetch zones")
	}

	var endpoints []*endpoint.Endpoint
	for _, zone := range zones {
		endpoints = append(endpoints, aggregateRecords(zone)...)
	}
	return endpoints, nil
}

func (p *Provider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	if changes == nil || !changes.HasChanges() {
		slog.Debug("Skipping request to apply changes because no changes are present")
		return nil
	}

	errs := oops.In("Provider").
		With("creates", len(changes.Create)).
		With("deletes", len(changes.Delete)).
		With("updates", len(changes.UpdateNew)).
		Span("ApplyChanges")

	// If we are in dry-run mode, we can skip the creation of endpoints and
	// only log the changes that would have been made.
	if p.Options.DryRun {
		return p.applyChangesDryRun(ctx, changes)
	}

	err := p.createEndpoints(ctx, changes.Create)
	if err != nil {
		slog.Error("Failed to create endpoints",
			slog.Any("error", err))

		return errs.Wrapf(err, "failed to apply creates")
	}

	// If we have no deletions or updates, we can return early to avoid making a (potentially)
	// expensive call to the Bunny.net API.
	if len(changes.Delete) == 0 && len(changes.UpdateOld) == 0 {
		return nil
	}

	var lookupEndpoints []*endpoint.Endpoint
	lookupEndpoints = append(lookupEndpoints, changes.Delete...)
	lookupEndpoints = append(lookupEndpoints, changes.UpdateOld...)

	tuples, err := p.fetchIdentifiers(ctx, lookupEndpoints)
	if err != nil {
		slog.Error("Failed to fetch identifiers",
			slog.Any("error", err))

		return errs.Wrapf(err, "failed to fetch identifiers")
	}

	err = p.deleteEndpoints(ctx, tuples, changes.Delete)
	if err != nil {
		slog.Error("Failed to delete endpoints",
			slog.Any("error", err))

		return errs.Wrapf(err, "failed to apply deletes")
	}

	err = p.updateEndpoints(ctx, tuples, changes.UpdateOld, changes.UpdateNew)
	if err != nil {
		slog.Error("Failed to update endpoints",
			slog.Any("error", err))

		return errs.Wrapf(err, "failed to apply updates")
	}

	return nil
}

func (p *Provider) applyChangesDryRun(ctx context.Context, changes *plan.Changes) error {
	if changes == nil || !changes.HasChanges() {
		slog.Debug("DRY RUN: Skipping request to apply changes because no changes are present")
		return nil
	}

	errs := oops.In("Provider").
		With("creates", len(changes.Create)).
		With("deletes", len(changes.Delete)).
		With("updates", len(changes.UpdateNew)).
		Span("applyChangesDryRun")

	for _, ep := range changes.Create {
		slog.InfoContext(ctx, "DRY RUN: Create record",
			slog.Group("record",
				slog.Any("name", ep.DNSName),
				slog.Any("type", ep.RecordType),
				slog.Any("value", ep.Targets),
				slog.Any("ttl", ep.RecordTTL),
			))
	}

	// If we have no deletions or updates, we can return early to avoid making a (potentially)
	// expensive call to the Bunny.net API.
	if len(changes.Delete) == 0 && len(changes.UpdateOld) == 0 {
		return nil
	}

	var lookupEndpoints []*endpoint.Endpoint
	lookupEndpoints = append(lookupEndpoints, changes.Delete...)
	lookupEndpoints = append(lookupEndpoints, changes.UpdateOld...)

	tuples, err := p.fetchIdentifiers(ctx, lookupEndpoints)
	if err != nil {
		slog.Error("Failed to fetch identifiers",
			slog.Any("error", err))

		return errs.Wrapf(err, "failed to fetch identifiers")
	}

	for _, ep := range changes.Delete {
		for _, t := range ep.Targets {
			tuple, ok := tuples[identifierKey(ep.DNSName, ep.RecordType, t)]
			if !ok {
				slog.InfoContext(ctx, "DRY RUN: Delete target (not found in Bunny)",
					slog.String("dns_name", ep.DNSName),
					slog.String("type", ep.RecordType),
					slog.String("value", t))
				continue
			}
			slog.InfoContext(ctx, "DRY RUN: Delete record",
				slog.Int64("zone_id", tuple.ZoneID),
				slog.Int64("record_id", tuple.RecordID),
				slog.String("dns_name", ep.DNSName),
				slog.String("value", t))
		}
	}

	for _, ep := range changes.UpdateOld {
		newEP := matchEndpoint(changes.UpdateNew, ep.DNSName, ep.RecordType)
		for _, t := range ep.Targets {
			tuple, ok := tuples[identifierKey(ep.DNSName, ep.RecordType, t)]
			if !ok {
				slog.InfoContext(ctx, "DRY RUN: Update target (not found in Bunny)",
					slog.String("dns_name", ep.DNSName),
					slog.String("type", ep.RecordType),
					slog.String("value", t))
				continue
			}
			var newTargets []string
			if newEP != nil {
				newTargets = newEP.Targets
			}
			slog.InfoContext(ctx, "DRY RUN: Update record",
				slog.Int64("zone_id", tuple.ZoneID),
				slog.Int64("record_id", tuple.RecordID),
				slog.String("dns_name", ep.DNSName),
				slog.String("value", t),
				slog.Any("new_targets", newTargets))
		}
	}

	return nil
}

// AdjustEndpoints canonicalizes a set of candidate endpoints.
// It is called with a set of candidate endpoints obtained from the various sources.
// It returns a set modified as required by the provider. The provider is responsible for
// adding, removing, and modifying the ProviderSpecific properties to match
// the endpoints that the provider returns in `Records` so that the change plan will not have
// unnecessary (potentially failing) changes. It may also modify other fields, add, or remove
// Endpoints. It is permitted to modify the supplied endpoints.
func (p *Provider) AdjustEndpoints(incoming []*endpoint.Endpoint) ([]*endpoint.Endpoint, error) {
	errs := oops.In("Provider").
		Span("AdjustEndpoints")

	fetched, err := p.Records(context.Background())
	if err != nil {
		slog.Error("Failed to fetch records",
			slog.Any("error", err))

		return nil, errs.Wrapf(err, "failed to fetch records")
	}

	for _, editing := range incoming {
		for _, checked := range fetched {
			if editing.DNSName != checked.DNSName || editing.RecordType != checked.RecordType || editing.SetIdentifier != checked.SetIdentifier {
				continue
			}

			for key, value := range checked.Labels {
				editing.Labels[key] = value
			}
		}
	}

	return incoming, nil
}

// GetDomainFilter returns the domain filter used by this provider.
func (p *Provider) GetDomainFilter() endpoint.DomainFilterInterface {
	return p.filter
}

// getZoneID returns the zone ID for a given record name using the zone
// map. If the zone ID is not found, an error is returned. The record name
// is expected to be a fully qualified DNS name (record + domain. e.g. foo.example.com).
func (p *Provider) getZoneID(dnsName string) (int64, error) {
	errs := oops.In("Provider").
		Span("getZoneID").
		With("dnsName", dnsName)

	_, domainName, ok := extractRecordComponents(p.allZones(), dnsName)
	if !ok {
		return 0, errs.Errorf("failed to extract components for %q", dnsName)
	}

	zoneID, ok := p.zoneMap.Load(domainName)
	if !ok {
		return 0, errs.Errorf("zone ID for DNS name %q (%s) not found", dnsName, domainName)
	}

	return zoneID, nil
}

// createEndpoints creates the given endpoints. Each target on an endpoint
// becomes a separate Bunny record.
func (p *Provider) createEndpoints(ctx context.Context, creates []*endpoint.Endpoint) error {
	errs := oops.In("Provider").Span("createEndpoints").With("creates", len(creates))

	for _, create := range creates {
		bunnyZoneID, err := p.getZoneID(create.DNSName)
		if err != nil {
			return errs.Wrapf(err, "failed to create record %q", create.DNSName)
		}
		recordName, domainName, ok := extractRecordComponents(p.allZones(), create.DNSName)
		if !ok {
			return errs.Errorf("failed to extract components for %q", create.DNSName)
		}

		opts, err := providerSpecificOptionsFromEndpoint(create)
		if err != nil {
			return errs.Wrapf(err, "failed to create record %q", create.DNSName)
		}

		for _, target := range create.Targets {
			record := CreateRecordRequest{
				Name:        recordName,
				Type:        RecordTypeFromString(create.RecordType),
				Value:       target,
				TTLSeconds:  int(create.RecordTTL),
				MonitorType: opts.MonitorType,
				Weight:      opts.Weight,
				Disabled:    opts.Disabled,
			}

			created, err := p.client.CreateRecord(ctx, strconv.FormatInt(bunnyZoneID, 10), record)
			if err != nil {
				return err
			}
			slog.InfoContext(ctx, "Record created.",
				slog.String("zone", domainName),
				slog.Int64("zone_id", bunnyZoneID),
				slog.Group("record",
					slog.Int64("id", created.ID),
					slog.String("name", record.Name),
					slog.String("type", record.Type.String()),
					slog.String("value", record.Value),
					slog.Int("ttl", record.TTLSeconds),
				))
		}
	}
	return nil
}

// updateEndpoints applies updates by diffing each endpoint's target set:
// targets only in old are deleted, targets only in new are created, and
// targets in both are touched only if endpoint-level metadata (TTL,
// monitor type, weight, disabled) differs between old and new.
func (p *Provider) updateEndpoints(
	ctx context.Context,
	identifiers map[string]identifierTuple,
	olds []*endpoint.Endpoint,
	news []*endpoint.Endpoint,
) error {
	for _, desired := range news {
		old := matchEndpoint(olds, desired.DNSName, desired.RecordType)
		if old == nil {
			return fmt.Errorf("update %q: no matching old endpoint", desired.DNSName)
		}

		oldTargets := stringSet(old.Targets)
		newTargets := stringSet(desired.Targets)

		// Deletions: targets in old that aren't in new.
		for t := range oldTargets {
			if _, kept := newTargets[t]; kept {
				continue
			}
			tuple, ok := identifiers[identifierKey(old.DNSName, old.RecordType, t)]
			if !ok {
				slog.Warn("Update: target not found in Bunny — skipping delete",
					slog.String("dns_name", old.DNSName),
					slog.String("type", old.RecordType),
					slog.String("value", t))
				continue
			}
			if err := p.client.DeleteRecord(ctx, tuple.ZoneID, tuple.RecordID); err != nil {
				return err
			}
		}

		// Creations: targets in new that aren't in old.
		bunnyZoneID, err := p.getZoneID(desired.DNSName)
		if err != nil {
			return err
		}
		recordName, _, ok := extractRecordComponents(p.allZones(), desired.DNSName)
		if !ok {
			return fmt.Errorf("update %q: cannot extract components", desired.DNSName)
		}
		opts, _ := providerSpecificOptionsFromEndpoint(desired)

		for t := range newTargets {
			if _, existed := oldTargets[t]; existed {
				continue
			}
			record := CreateRecordRequest{
				Name:        recordName,
				Type:        RecordTypeFromString(desired.RecordType),
				Value:       t,
				TTLSeconds:  int(desired.RecordTTL),
				MonitorType: opts.MonitorType,
				Weight:      opts.Weight,
				Disabled:    opts.Disabled,
			}
			if _, err := p.client.CreateRecord(ctx, strconv.FormatInt(bunnyZoneID, 10), record); err != nil {
				return err
			}
		}

		// Surviving targets: only touch if metadata differs.
		if !endpointMetadataEqual(old, desired) {
			for t := range newTargets {
				if _, existed := oldTargets[t]; !existed {
					continue
				}
				tuple, ok := identifiers[identifierKey(old.DNSName, old.RecordType, t)]
				if !ok {
					continue
				}
				record := UpdateRecordRequest{
					TTLSeconds:  int(desired.RecordTTL),
					Value:       t,
					MonitorType: opts.MonitorType,
					Weight:      opts.Weight,
					Disabled:    opts.Disabled,
				}
				if err := p.client.UpdateRecord(ctx, tuple.ZoneID, tuple.RecordID, record); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func matchEndpoint(eps []*endpoint.Endpoint, dnsName, recordType string) *endpoint.Endpoint {
	for _, ep := range eps {
		if ep.DNSName == dnsName && ep.RecordType == recordType {
			return ep
		}
	}
	return nil
}

func stringSet(in []string) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for _, s := range in {
		out[s] = struct{}{}
	}
	return out
}

// endpointMetadataEqual reports whether two endpoints share the same
// non-target settings (TTL + provider-specific options).
func endpointMetadataEqual(a, b *endpoint.Endpoint) bool {
	if a.RecordTTL != b.RecordTTL {
		return false
	}
	ao, _ := providerSpecificOptionsFromEndpoint(a)
	bo, _ := providerSpecificOptionsFromEndpoint(b)
	return ao == bo
}

// deleteEndpoints deletes every Bunny record matching the (name, type, value)
// of each target on each endpoint.
func (p *Provider) deleteEndpoints(
	ctx context.Context,
	identifiers map[string]identifierTuple,
	deletions []*endpoint.Endpoint,
) error {
	for _, deletion := range deletions {
		for _, t := range deletion.Targets {
			tuple, ok := identifiers[identifierKey(deletion.DNSName, deletion.RecordType, t)]
			if !ok {
				slog.Warn("Delete: record not found in Bunny — skipping",
					slog.String("dns_name", deletion.DNSName),
					slog.String("type", deletion.RecordType),
					slog.String("value", t))
				continue
			}
			if err := p.client.DeleteRecord(ctx, tuple.ZoneID, tuple.RecordID); err != nil {
				return err
			}
			slog.InfoContext(ctx, "Record deleted.",
				slog.Int64("zone_id", tuple.ZoneID),
				slog.Int64("record_id", tuple.RecordID))
		}
	}
	return nil
}

type identifierTuple struct {
	ZoneID   int64
	RecordID int64
}

// identifierKey identifies a single Bunny DNS record by its DNS name, type,
// and value. Including the value lets us address individual records when an
// endpoint has multiple targets (e.g. round-robin A records).
func identifierKey(dnsName, recordType, value string) string {
	return dnsName + "|" + recordType + "|" + value
}

// fetchIdentifiers fetches the zone and record identifiers for the given
// endpoints, indexed by (DNSName, RecordType, Target value). Pass every
// target you might want to address — the function returns the identifiers
// it could resolve and silently omits the rest (callers handle absence).
func (p *Provider) fetchIdentifiers(ctx context.Context, endpoints []*endpoint.Endpoint) (map[string]identifierTuple, error) {
	identifiers := make(map[string]identifierTuple)

	zones, err := p.fetchZones(ctx)
	if err != nil {
		return nil, err
	}

	domainNames := make([]string, 0, len(zones))
	for _, zone := range zones {
		domainNames = append(domainNames, zone.Domain)
	}

	for _, ep := range endpoints {
		recordName, domainName, ok := extractRecordComponents(domainNames, ep.DNSName)
		if !ok {
			return nil, fmt.Errorf("record %q cannot be handled, no matching zone found", ep.DNSName)
		}

		for _, zone := range zones {
			if zone.Domain != domainName {
				continue
			}
			for _, record := range zone.Records {
				if record.Name != recordName || record.Type.String() != ep.RecordType {
					continue
				}
				key := identifierKey(ep.DNSName, ep.RecordType, record.Value)
				if existing, dup := identifiers[key]; dup {
					slog.Warn("Duplicate Bunny record for (name, type, value) — keeping first; later operations will leave the duplicate behind",
						slog.String("dns_name", ep.DNSName),
						slog.String("type", ep.RecordType),
						slog.String("value", record.Value),
						slog.Int64("kept_record_id", existing.RecordID),
						slog.Int64("dropped_record_id", record.ID))
					continue
				}
				identifiers[key] = identifierTuple{
					ZoneID:   zone.ID,
					RecordID: record.ID,
				}
			}
		}
	}
	return identifiers, nil
}

func (p *Provider) fetchZones(ctx context.Context) ([]*Zone, error) {
	var page = 1
	var zones []*Zone

	for {
		results, err := p.client.ListZones(ctx, ListZonesRequest{
			Page:    page,
			PerPage: 1000,
		})

		if err != nil {
			return nil, err
		}

		for _, zone := range results.Items {
			// Cache the zone ID for lookup during creates.
			p.cacheZone(zone)

			zones = append(zones, zone)
		}

		if !results.HasMoreItems {
			break
		}

		page++
	}

	return zones, nil
}

// extractRecordComponents extracts the record name and zone from a given DNS
// name by matching the DNS name with the list of available zones. It accepts
// either an exact match (apex record, returns "" as the record name) or a
// subdomain (dnsName == record + "." + zone). When multiple zones match
// (overlapping zones like "example.com" and "b.example.com"), the longest
// zone wins so the most specific record name is returned.
func extractRecordComponents(zones []string, dnsName string) (string, string, bool) {
	bestZone := ""
	for _, zone := range zones {
		if dnsName == zone {
			if len(zone) > len(bestZone) {
				bestZone = zone
			}
			continue
		}
		if strings.HasSuffix(dnsName, "."+zone) {
			if len(zone) > len(bestZone) {
				bestZone = zone
			}
		}
	}

	if bestZone == "" {
		return "", "", false
	}
	if dnsName == bestZone {
		return "", bestZone, true
	}
	return dnsName[:len(dnsName)-len(bestZone)-1], bestZone, true
}

func getDomainFilter(options Options) endpoint.DomainFilterInterface {
	if options.ExcludeDomainsRegexp != "" || options.IncludeDomainsRegexp != "" {
		return endpoint.NewRegexDomainFilter(
			regexp.MustCompile(options.IncludeDomainsRegexp),
			regexp.MustCompile(options.ExcludeDomainsRegexp),
		)
	}

	return endpoint.NewDomainFilterWithExclusions(options.IncludeDomains, options.ExcludeDomains)
}
