# lodestone

A self-hosted BitTorrent indexer, DHT crawler, content classifier and torrent search engine with web UI, GraphQL API and Servarr stack integration.

## Networking considerations

Lodestone uses aggressive DHT crawling which generates significant UDP and TCP traffic to thousands of distinct remote peers. This can overwhelm Docker's connection tracking (conntrack) subsystem and break network connectivity for other containers on the same host.

### Host kernel tuning

Run these commands on the Docker host before starting Lodestone:

```sh
# Increase the conntrack table size so Lodestone's many parallel flows
# don't fill it up and block other containers.
sysctl -w net.netfilter.nf_conntrack_max=262144

# Shorten UDP conntrack timeouts so stale DHT query entries are cleaned
# up faster instead of lingering for minutes.
sysctl -w net.netfilter.nf_conntrack_udp_timeout=30
sysctl -w net.netfilter.nf_conntrack_udp_timeout_stream=60

# Widen the ephemeral port range so the kernel doesn't run out of
# local ports for Lodestone's TCP metadata connections.
sysctl -w net.ipv4.ip_local_port_range="1024 65535"
```

To make these settings persist across reboots, add them to `/etc/sysctl.conf` or `/etc/sysctl.d/99-lodestone.conf`.

### Container networking

The compose stack splits lodestone into two processes so the DHT crawler and the
Web UI each get the networking they need:

- `lodestone-crawler` runs with `network_mode: host`. The crawler sends and
  receives a lot of DHT UDP; on Docker Desktop (macOS/Windows) the bridge NAT
  breaks DHT hash discovery, so the crawler needs host networking.
- `lodestone-web` runs in bridge mode and publishes port `3333`, so the Web UI /
  GraphQL API is reachable from the host, including its LAN/Tailscale address.
  It also runs the queue/classifier.
- `lodestone-crawler-metrics` publishes the crawler's own metrics/health endpoint
  (`:3335`) through a small socat proxy, since host-networked containers cannot
  publish ports themselves.

Both lodestone processes share the same Postgres.

On a Linux Docker host you can instead run a single lodestone service with
`network_mode: host` (combined with the kernel tuning above), which bypasses
Docker's conntrack table entirely.

### Tuning the crawler

The DHT crawler's aggressiveness is controlled by `ScalingFactor` in the crawler configuration. The default (2) is deliberately conservative. If you have plenty of bandwidth, increase it gradually. Values above 5 rarely yield proportional gains.
