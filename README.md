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

### Host networking

The `docker-compose.yml` uses `network_mode: host` for the lodestone service. This bypasses Docker's NAT/masquerading layer entirely — DHT traffic goes directly through the host's network stack without creating per-flow conntrack entries in Docker's bridge. Combined with the kernel tuning above, this prevents Lodestone's traffic from starving other containers.

### Tuning the crawler

The DHT crawler's aggressiveness is controlled by `ScalingFactor` in the crawler configuration. The default (2) is deliberately conservative. If you have plenty of bandwidth, increase it gradually. Values above 5 rarely yield proportional gains.
