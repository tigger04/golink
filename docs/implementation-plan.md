<!-- Version: 0.7 | Last updated: 2026-04-18 -->

# Implementation Plan

v1 of golink is delivered as one piece of work, in two phases. The work is small: a Go binary, a few YAML files, a Makefile, a CI workflow, a README. Production hardening (systemd unit, sandbox directives, deploy mechanism) is handled by the hetzner deploy tooling, not this repo.

## Target

A Go binary serving `https://{host}/{prefix}/...` with YAML-defined resolvers (Amazon, GitHub, Wikipedia at launch). Domain-agnostic: the binary knows nothing about its public hostname; that is the reverse proxy's job. Architecture is in [docs/architecture.md](architecture.md).

## Two phases, one issue

### Phase 1 - Build the application

A working Go binary that runs locally and in CI on Linux:

- Project skeleton, `go.mod`, MIT `LICENSE`, `THIRD-PARTY-NOTICES.md`, README with quickstart and DB-IP attribution
- Makefile: `build`, `test`, `lint`, `install`, `uninstall`, `sync`, `release` targets
- HTTP server with the resolver layer, GeoIP wrapper, structured logging, SIGHUP reload, graceful SIGTERM
- One YAML-driven templated resolver type
- Three example resolver YAMLs (`az.yaml`, `gh.yaml`, `wiki.yaml`) under `examples/resolvers/`
- Live resolver configs under `resolvers/` (separate from examples)
- Self-managed GeoIP database via DB-IP IP-to-Country Lite (download on first start, daily refresh check, monthly refresh cadence)
- Click analytics: SQLite event store, `golink stats` subcommand with 6 report types, CSV output
- `goreport` convenience script for remote stats/logs/status over SSH/Tailscale
- `-version` flag, `-credits` flag (prints third-party notices)
- Tests under `tests/regression/` covering user-facing behaviour through the same entry point a real user would use
- **GitHub Actions workflow** running `make test` on `ubuntu-latest` for every push and PR - this is the only way I can verify Linux behaviour from a macOS dev host

**End state.** `make test` passes locally on macOS and in CI on Linux. Running `./golink` (with sensible defaults so it just works without flags) serves correct redirects via curl for all three resolver types.

### Phase 2 - Ship to production

Deploy to a Hetzner VPS via the hetzner repo's deploy tooling. The deploy tooling owns systemd, hardening, state directories, restart, and atomic install - golink just needs to plug into it.

- Deployment via `deploy-app --host <hostname> golink 18081` (from the hetzner repo)
- Per-host config in `config/<hostname>.yaml` (addr, base_url, extra_domains)
- Caddyfile site stanzas managed by the hetzner repo (one per domain)
- DNS records managed by the hetzner repo's deploy tooling
- Smoke tests from the public internet (UTs in the issue)

**End state.** The deployed instance serves `/az/{asin}`, `/gh/{user}/{repo}`, and `/wiki/{article}` correctly for real internet users over HTTPS via Caddy. The GeoIP database refreshes itself once a month with no manual intervention. Additional vanity domains (e.g. `remy.lobb.ie`) route through golink for click analytics via the `extra_domains` config.

## Tracking

A single GitHub issue covers both phases. The phase boundary is a natural review checkpoint but not a separate SDLC gate - both phases must be complete before the issue is APPROVED.

## What this repo does NOT ship

These are deliberately not in scope, because the hetzner deploy tooling provides them:

- A systemd unit (rendered from the `goapp.service.template` in the hetzner repo)
- Hardening directives (provided by the hetzner template's strict defaults; golink fits without overrides)
- A reverse proxy config (Caddyfiles live in the hetzner repo)
- An install/deploy script for the server (use the hetzner repo's `deploy-app`)
- A GeoIP fetch script or systemd timer (golink self-fetches; no external machinery needed)
- A MaxMind license key workflow (DB-IP needs no key)
