---
title: Servarr Integration
description: Integrating lodestone with applications from the Servarr stack
parent: Guides
layout: default
nav_order: 8
redirect_from:
  - /tutorials/servarr-integration.html
---

# Servarr Integration

**lodestone**'s HTTP server exposes an endpoint at `/torznab`, allowing it to integrate with any application that supports [the Torznab specification](https://torznab.github.io/spec-1.3-draft/index.html), most notably apps in [the Servarr stack](https://wiki.servarr.com/) (Prowlarr, Sonarr, Radarr etc.).

## Adding **lodestone** as an indexer in Prowlarr

To get started, open your Prowlarr instance, click "Add Indexer", and select "Generic Torznab" from the list.

![Prowlarr Add Indexer](/assets/images/prowlarr-1.png)

The required settings are fairly basic. Assuming you've adapted from the [example docker-compose file]({% link setup/installation.md %}#docker), and Prowlarr is on the same Docker network as **lodestone**, then Prowlarr should be able to access the Torznab endpoint of your **lodestone** instance at `http://lodestone:3333/torznab`. No further configuration should be needed, just click the "Test" button to ensure everything is working.

![Prowlarr configure lodestone](/assets/images/prowlarr-2.png)

[Depending on your Prowlarr configuration](https://wiki.servarr.com/prowlarr/settings#applications), the **lodestone** indexer should now be synced to your other \*arr applications. Alternatively, you can add **lodestone** as an indexer directly in those applications, following the same steps as above.
