---
title: Installation
description: Installation instructions for lodestone
parent: Setup
layout: default
nav_order: 1
---

# Installation

## Docker

The quickest way to get up-and-running with **lodestone** is with [Docker Compose](https://docs.docker.com/compose/). The following `docker-compose.yml` is a minimal example. For a more full-featured example including VPN routing and observability services see the [docker compose configuration in the GitHub repository](https://github.com/ghobs91/lodestone/blob/main/docker-compose.yml).

```yml
services:
  lodestone:
    image: ghcr.io/ghobs91/lodestone:latest
    container_name: lodestone
    ports:
      # API and WebUI port:
      - "3333:3333"
      # BitTorrent ports:
      - "3334:3334/tcp"
      - "3334:3334/udp"
    restart: unless-stopped
    environment:
      - POSTGRES_HOST=postgres
      - POSTGRES_PASSWORD=postgres
    #      - TMDB_API_KEY=your_api_key
    volumes:
      - ./config:/root/.config/lodestone
    command:
      - worker
      - run
      - --keys=http_server
      - --keys=queue_server
      # disable the next line to run without DHT crawler
      - --keys=dht_crawler
    depends_on:
      postgres:
        condition: service_healthy

  postgres:
    image: postgres:16-alpine
    container_name: lodestone-postgres
    volumes:
      - ./data/postgres:/var/lib/postgresql/data
    #    ports:
    #      - "5432:5432" Expose this port if you'd like to dig around in the database
    restart: unless-stopped
    environment:
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=lodestone
      - PGUSER=postgres
    shm_size: 1g
    healthcheck:
      test:
        - CMD-SHELL
        - pg_isready
      start_period: 20s
      interval: 10s
```

After running `docker compose up -d` you should be able to access the web interface at `http://localhost:3333`. The DHT crawler should have started and you should see items appear in the web UI within around a minute.

To upgrade your installation you can run:

```sh
docker compose down lodestone
docker pull ghcr.io/ghobs91/lodestone:latest
docker compose up -d lodestone
```

## go install

You can also install **lodestone** natively with `go install github.com/ghobs91/lodestone`. If you choose this method you will need to [configure]({% link setup/configuration.md %}) (at a minimum) a Postgres instance for lodestone to connect to.

## Running the CLI

The **lodestone** CLI is the entrypoint into the application. Take note of the command needed to run the CLI, depending on your installation method.

- If you are using the docker-compose example above, you can run the CLI (while the stack is started) with `docker exec -it lodestone lodestone`.
- If you installed lodestone with `go install`, you can run the CLI with `lodestone`.

When referring to CLI commands in the rest of the documentation, for simplicity we will use `lodestone`; please substitute this for the correct command. For example, to show the CLI help, run:

```sh
lodestone --help
```

## Starting **lodestone**

**lodestone** runs as multiple worker processes that can be started either individually or all at once. To start all workers, run:

```sh
lodestone worker run --all
```

Alternatively, specify individual workers to start:

```sh
lodestone worker run --keys=http_server,queue_server,dht_crawler
```
