#!/usr/bin/env bash
#
# Build the current working tree and roll it out to every lodestone host.
#
# Both hosts build their own image from the same source (the MacBook Pro is
# arm64, the Mac Pro is x86_64), so nothing is cross-compiled and each host runs
# a native binary. A host is only redeployed if its build succeeds, and it is
# rolled back automatically if the post-deploy crawl check fails.
#
# Usage:
#   scripts/deploy.sh                 # build + deploy + verify everywhere
#   scripts/deploy.sh --host local    # only the MacBook Pro
#   scripts/deploy.sh --host macpro   # only the Mac Pro
#   scripts/deploy.sh --fast          # skip the throughput check
#   scripts/deploy.sh --build-only    # build images, do not touch containers
#
# Exit status is non-zero if any host failed to build, deploy or verify.
set -uo pipefail

REPO_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Host layout. Override via environment if things move.
REMOTE_HOST="${LODESTONE_REMOTE_HOST:-macpro}"
REMOTE_COMPOSE_DIR="${LODESTONE_REMOTE_COMPOSE_DIR:-/Users/andrew/Services/lodestone}"
REMOTE_BUILD_DIR="${LODESTONE_REMOTE_BUILD_DIR:-/Users/andrew/lodestone-build}"

IMAGE="lodestone:current"
CANDIDATE="lodestone:candidate"
PREVIOUS="lodestone:previous"

WARMUP_SECONDS="${LODESTONE_WARMUP:-120}"
VERIFY_SECONDS="${LODESTONE_VERIFY:-60}"

TARGETS="local macpro"
FAST=0
BUILD_ONLY=0

while [ $# -gt 0 ]; do
  case "$1" in
    --host) TARGETS="$2"; shift 2 ;;
    --host=*) TARGETS="${1#*=}"; shift ;;
    --fast) FAST=1; shift ;;
    --build-only) BUILD_ONLY=1; shift ;;
    -h|--help) awk 'NR>1 && /^#/ {sub(/^# ?/,""); print; next} NR>1 {exit}' "$0"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

# Docker Desktop is not on PATH in a non-interactive ssh session.
DOCKER_PATH='export PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH"'

say()  { printf '\n\033[1m==> %s\033[0m\n' "$*"; }
ok()   { printf '\033[32m    ok\033[0m %s\n' "$*"; }
warn() { printf '\033[33m    !! \033[0m %s\n' "$*"; }
fail() { printf '\033[31m  FAIL\033[0m %s\n' "$*"; }

# Runs a command string on either the local host or the remote one.
# The command is passed through untouched, so it must be safe in a shell.
run_on() { # $1 = local|remote ; $2 = command
  local where="$1" cmd="$2"
  if [ "$where" = "local" ]; then
    bash -c "$cmd"
  else
    ssh -o BatchMode=yes -o ConnectTimeout=15 "$REMOTE_HOST" "$cmd"
  fi
}

reachable() { # $1 = local|remote
  run_on "$1" "true" >/dev/null 2>&1
}

# Reads one prometheus sample from the crawler's metrics endpoint.
# $1 = exact metric prefix, e.g. lodestone_dht_ktable_nodes_count
# Produces a command string with no single quotes in it, so it survives both
# `bash -c` and `ssh`.
metrics_probe() {
  printf "curl -sf --max-time 10 http://localhost:3335/metrics | sed -n 's/^%s //p' | head -1" "$1"
}

# Confirms a host is actually crawling, not merely "container started".
# Returns non-zero (after rolling back) if it is not.
verify_host() { # $1 = local|remote
  local where="$1" nodes before after delta

  say "verifying crawl on $where (warmup ${WARMUP_SECONDS}s, window ${VERIFY_SECONDS}s)"
  sleep "$WARMUP_SECONDS"

  nodes=$(run_on "$where" "$(metrics_probe lodestone_dht_ktable_nodes_count)")
  if [ -z "$nodes" ] || [ "$nodes" = "0" ]; then
    fail "routing table is empty (nodes=${nodes:-none})"
    return 1
  fi
  ok "routing table nodes: $nodes"

  if [ "$FAST" = "1" ]; then
    warn "skipping throughput check (--fast)"
    return 0
  fi

  before=$(run_on "$where" "$(metrics_probe 'lodestone_dht_crawler_persisted_total{entity="Torrent"}')")
  sleep "$VERIFY_SECONDS"
  after=$(run_on "$where" "$(metrics_probe 'lodestone_dht_crawler_persisted_total{entity="Torrent"}')")

  if [ -z "$before" ] || [ -z "$after" ]; then
    fail "could not read persisted_total"
    return 1
  fi

  delta=$(awk -v a="$before" -v b="$after" 'BEGIN { printf "%d", b - a }')
  if [ "$delta" -le 0 ]; then
    fail "no torrents persisted in ${VERIFY_SECONDS}s (persisted_total stuck at $after)"
    return 1
  fi

  ok "persisted $delta torrents in ${VERIFY_SECONDS}s (~$((delta * 3600 / VERIFY_SECONDS))/h)"
  return 0
}

# Promotes the candidate image to $IMAGE, recreating the containers.
promote_and_restart() { # $1 = local|remote ; $2 = compose dir
  local where="$1" dir="$2"

  # Guard against recreating containers from a stale tag: if the compose file
  # doesn't reference the tag we are about to maintain, the deploy would appear
  # to succeed while actually running the previous image.
  if ! run_on "$where" "grep -q 'image: $IMAGE' '$dir/docker-compose.yml'"; then
    fail "$dir/docker-compose.yml does not reference '$IMAGE'"
    return 1
  fi

  run_on "$where" "$DOCKER_PATH && cd '$dir' && { docker image inspect '$IMAGE' >/dev/null 2>&1 && docker tag '$IMAGE' '$PREVIOUS' || true; } && docker tag '$CANDIDATE' '$IMAGE'" || return 1
  run_on "$where" "$DOCKER_PATH && cd '$dir' && docker compose up -d --force-recreate lodestone-crawler lodestone-web" || return 1
}

rollback() { # $1 = local|remote ; $2 = compose dir
  local where="$1" dir="$2"

  warn "rolling back to $PREVIOUS"
  run_on "$where" "$DOCKER_PATH && cd '$dir' && docker image inspect '$PREVIOUS' >/dev/null 2>&1 && docker tag '$PREVIOUS' '$IMAGE' && docker compose up -d --force-recreate lodestone-crawler lodestone-web" >/dev/null 2>&1
}

deploy_local() {
  say "MacBook Pro — building"
  if ! docker build -t "$CANDIDATE" "$REPO_DIR" >/tmp/lodestone-build-local.log 2>&1; then
    fail "build failed, nothing deployed (see /tmp/lodestone-build-local.log)"
    tail -20 /tmp/lodestone-build-local.log
    return 1
  fi
  ok "built $CANDIDATE"

  [ "$BUILD_ONLY" = "1" ] && { ok "skipping deploy (--build-only)"; return 0; }

  say "MacBook Pro — recreating"
  if ! promote_and_restart local "$REPO_DIR"; then
    fail "compose up failed"
    return 1
  fi

  if ! verify_host local; then
    rollback local "$REPO_DIR"
    return 1
  fi
}

deploy_macpro() {
  if ! reachable remote; then
    fail "cannot reach $REMOTE_HOST"
    return 1
  fi

  say "Mac Pro — syncing source"
  if ! rsync -a --delete \
      --exclude 'data/' --exclude '.serena/' --exclude '.DS_Store' --exclude 'node_modules/' \
      "$REPO_DIR/" "$REMOTE_HOST:$REMOTE_BUILD_DIR/"; then
    fail "rsync failed"
    return 1
  fi
  ok "source synced to $REMOTE_BUILD_DIR"

  say "Mac Pro — building (native x86_64)"
  if ! run_on remote "$DOCKER_PATH && cd '$REMOTE_BUILD_DIR' && docker build -t '$CANDIDATE' ." \
        >/tmp/lodestone-build-macpro.log 2>&1; then
    fail "build failed, nothing deployed (see /tmp/lodestone-build-macpro.log)"
    tail -20 /tmp/lodestone-build-macpro.log
    return 1
  fi
  ok "built $CANDIDATE"

  [ "$BUILD_ONLY" = "1" ] && { ok "skipping deploy (--build-only)"; return 0; }

  say "Mac Pro — recreating"
  if ! promote_and_restart remote "$REMOTE_COMPOSE_DIR"; then
    fail "compose up failed"
    return 1
  fi

  if ! verify_host remote; then
    rollback remote "$REMOTE_COMPOSE_DIR"
    return 1
  fi
}

FAILED=""

for target in $TARGETS; do
  case "$target" in
    local)  deploy_local  || FAILED="$FAILED local" ;;
    macpro) deploy_macpro || FAILED="$FAILED macpro" ;;
    *) fail "unknown host '$target'"; FAILED="$FAILED $target" ;;
  esac
done

echo
if [ -n "$FAILED" ]; then
  fail "deploy incomplete:$FAILED"
  exit 1
fi

ok "deployed to:$TARGETS"
