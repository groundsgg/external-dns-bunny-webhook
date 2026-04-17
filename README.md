# ExternalDNS - Bunny Webhook Provider

ExternalDNS is a Kubernetes add-on for automatically managing Domain Name System (DNS) records for
Kubernetes services by using different DNS providers. By default, Kubernetes manages DNS records
internally, but ExternalDNS takes this functionality a step further by delegating the management of
DNS records to an external DNS provider such as this one.

This repository contains a provider that implements an ExternalDNS webhook provider for [Bunny.net](https://bunny.net).

## Important

This provider is not officially supported by [Bunny.net](https://bunny.net), but is maintained by the team at Contaim Labs
for the community. If you encounter any issues, please open an issue on this repository. If you have any questions
about Bunny.net, please reach out to their support team.

## Deployment

You can deploy the provider using any Kubernetes deployment method, such as Helm or kubectl. Examples for the official
external-dns Helm chart are provided below.

### External DNS Helm Chart

The default configuration is designed to work seamlessly with the official ExternalDNS Helm chart. Be sure to create the
`external-dns-bunny-secret` secret with the `api-key` key containing your Bunny.net API key or modify the configuration
to use a different secret or method of providing the API key.

The values file should look similar to the following:

```yaml
namespace: external-dns
provider:
  name: webhook
  webhook:
    image:
      repository: ghcr.io/groundsgg/external-dns-bunny-webhook
      tag: v0.4.0
    env:
      - name: BUNNY_API_KEY
        valueFrom:
          secretKeyRef:
            name: external-dns-bunny-secret
            key: api-key
```

To deploy the provider using the Helm chart, add the repository. You can skip this step if you already have the
repository added.

```shell
helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/
```

Once the repository is added, install the chart with the values file.

```shell
helm upgrade --install external-dns external-dns/external-dns --version 1.15.0
```

Additional configuration options are available below and may be set using environment variables.

## Configuration

The provider can be configured using the following environment variables:

| Environment Variable | Required | Description | Default |
|----------------------|----------|-------------|---------|
| `BUNNY_API_KEY` | Yes | The API key used to authenticate with the Bunny.net API. | |
| `BUNNY_DRY_RUN` | No | If set to `true`, the provider will not make any changes to the DNS records. | `false` |
| `WEBHOOK_HOST` | No | The host to use for the webhook endpoint. | `localhost` |
| `WEBHOOK_PORT` | No | The port to use for the webhook endpoint. | `8888` |
| `WEBHOOK_READ_TIMEOUT` | No | The read timeout for the webhook endpoint. | `60s` |
| `WEBHOOK_WRITE_TIMEOUT` | No | The write timeout for the webhook endpoint. | `60s` |
| `HEALTH_HOST` | No | The host to use for the health endpoint. | `0.0.0.0` |
| `HEALTH_PORT` | No | The port to use for the health endpoint. | `8080` |
| `HEALTH_READ_TIMEOUT` | No | The read timeout for the health endpoint. | `60s` |
| `HEALTH_WRITE_TIMEOUT` | No | The write timeout for the health endpoint. | `60s` |

## Provider-Specific Annotations

The following annotations may be added to sources to control behavior of the DNS records created by this provider:

### `external-dns.alpha.kubernetes.io/webhook-bunny-disabled`

If set to `true`, the DNS record will be managed but set to disabled in the Bunny API. This annotation is optional
and will default to `false` if not provided. Disabling a record will cause it to not respond to DNS queries,
but will still be managed by the provider and visible in the Bunny.net dashboard.

### `external-dns.alpha.kubernetes.io/webhook-bunny-monitor-type`

The monitor type to use for the DNS record. Valid values are `none` (default), `http`, and `ping`. This
annotation is optional and will default to `none` if not provided, which will create a standard DNS record
without any monitoring.

### `external-dns.alpha.kubernetes.io/webhook-bunny-weight`

The weight to use for the DNS record. Valid values are between 1 and 100. This annotation is optional and will
default to `100` if not provided. Any value outside of the valid range will be set to the nearest valid value,
and any non-integer value will result in the default value being used.

### Smart DNS Records

Smart DNS records route traffic based on latency (closest Bunny.net datacenter
region) or explicit geographic coordinates. Smart records are supported on
A and AAAA records.

To model multiple smart records under the same hostname (e.g., one per region),
create one Kubernetes source per record and differentiate them with
`external-dns.alpha.kubernetes.io/set-identifier`. The webhook encodes the
smart settings into `SetIdentifier` automatically when reading back from
Bunny (`latency:<zone>` or `geo:<lat>,<long>`), so external-dns sees a stable
view across reconciles.

#### `external-dns.alpha.kubernetes.io/webhook-bunny-smart-type`

The type of smart routing to apply. Valid values: `none` (default), `latency`,
`geo`. Unknown values fall back to `none` with a warning log.

#### `external-dns.alpha.kubernetes.io/webhook-bunny-smart-latency-zone`

Required if `smart-type=latency`. The Bunny.net zone string (typically an
ISO country/region code like `DE`, `US`, `SG`). Missing or empty values fall
back to non-smart with a warning log.

#### `external-dns.alpha.kubernetes.io/webhook-bunny-smart-geo-lat`

Required if `smart-type=geo`. A latitude between -90 and 90.

#### `external-dns.alpha.kubernetes.io/webhook-bunny-smart-geo-long`

Required if `smart-type=geo`. A longitude between -180 and 180.

Out-of-range or unparseable coordinates fall back to non-smart with a
warning log.

#### Example: latency-routed record across two regions

```yaml
# Kubernetes Service in the EU cluster
apiVersion: v1
kind: Service
metadata:
  name: api-eu
  annotations:
    external-dns.alpha.kubernetes.io/hostname: api.example.com
    external-dns.alpha.kubernetes.io/set-identifier: eu
    external-dns.alpha.kubernetes.io/webhook-bunny-smart-type: latency
    external-dns.alpha.kubernetes.io/webhook-bunny-smart-latency-zone: "DE"
---
# Kubernetes Service in the US cluster
apiVersion: v1
kind: Service
metadata:
  name: api-us
  annotations:
    external-dns.alpha.kubernetes.io/hostname: api.example.com
    external-dns.alpha.kubernetes.io/set-identifier: us
    external-dns.alpha.kubernetes.io/webhook-bunny-smart-type: latency
    external-dns.alpha.kubernetes.io/webhook-bunny-smart-latency-zone: "US"
```

#### Note: pre-existing dashboard records

If you have smart records created by hand in the Bunny.net dashboard and you
don't have a matching Kubernetes source annotation, external-dns will detect
drift and try to strip the smart settings. Either add the matching annotations
to your source, or exclude the records from external-dns management via the
ownership TXT registry.

## Development

A development environment can be set up using [Tilt](https://tilt.dev) by running the following command:

```shell
tilt up
```

This will start the development environment and open a browser window with the Tilt dashboard. The provider will
automatically reload when changes are made to the source code.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
