<!-- Version: 0.6 | Last updated: 2026-04-11 -->

# Implementation Plan

v1 of golink is delivered as one piece of work, in two phases. The work is small: a Go binary, a few YAML files, a Makefile, a CI workflow, a README. Most production hardening (systemd unit, sandbox directives, deploy mechanism) is handled by the kepler-452 deploy tooling, not this repo.

## Target

A Go binary serving `https://{host}/{prefix}/...` with YAML-defined resolvers (Amazon, GitHub, Wikipedia at launch). Domain-agnostic: the binary knows nothing about its public hostname; that is the reverse proxy's job. Architecture is in [docs/architecture.md](architecture.md).

## Two phases, one issue

### Phase 1 - Build the application

A working Go binary that runs locally and in CI on Linux:

- Project skeleton, `go.mod`, MIT `LICENSE`, `THIRD-PARTY-NOTICES.md`, README with quickstart and DB-IP attribution
- Makefile: `build`, `build-linux`, `test`, `lint`, `deploy`, `release`, `sync` targets
- `golangci-lint` config
- HTTP server with the resolver layer, GeoIP wrapper, structured logging, SIGHUP reload, graceful SIGTERM
- One YAML-driven templated resolver type
- Three example resolver YAMLs (`az.yaml`, `gh.yaml`, `wiki.yaml`) under `examples/resolvers/`
- Self-managed GeoIP database via DB-IP IP-to-Country Lite (download on first start, daily refresh check, monthly refresh cadence)
- `-version` flag, `-credits` flag (prints third-party notices)
- Tests under `tests/regression/` covering user-facing behaviour through the same entry point a real user would use
- **GitHub Actions workflow** running `make test` on `ubuntu-latest` for every push and PR - this is the only way I can verify Linux behaviour from a macOS dev host

**End state.** `make test` passes locally on macOS and in CI on Linux. Running `./golink` (with sensible defaults so it just works without flags) serves correct redirects via curl for all three resolver types.

### Phase 2 - Ship to production

Get it onto kepler-452 and live on the chosen domain. The kepler-452 deploy tooling owns systemd, hardening, state directories, restart, and atomic install - golink just needs to plug into it.

- `make deploy` Makefile target that builds a Linux binary and hands off to `~/code/hetzner/kepler-452/deploy-app.sh golink /tmp/golink-build 18081`
- Recommended Caddyfile site stanza documented in golink's README (Taḋg copies it into `~/code/hetzner/kepler-452/Caddyfile` once, runs `setup.sh deploy` to apply)
- DNS A record at the chosen domain pointing at kepler-452's public IP (Taḋg, manual)
- Smoke tests from the public internet (UTs in the issue)

**End state.** The deployed instance serves `/az/{asin}`, `/gh/{user}/{repo}`, and `/wiki/{article}` correctly for real internet users over HTTPS via Caddy on kepler-452. The GeoIP database refreshes itself once a month with no manual intervention. The same `make deploy` command works for any future redeploy.

## Tracking

A single GitHub issue covers both phases. The phase boundary is a natural review checkpoint but not a separate SDLC gate - both phases must be complete before the issue is APPROVED.

## What this repo does NOT ship

These are deliberately *not* in scope for v1, because the kepler-452 deploy tooling provides them:

- A `deploy/golink.service` systemd unit (rendered from the `goapp.service.template` in the kepler-452 repo)
- Hardening directives (provided by the kepler-452 template's strict defaults; golink fits without overrides)
- A reverse proxy config snippet shipped as a file (the recommended block lives in golink's README; the actual Caddyfile is in the kepler-452 repo)
- An install script (use the kepler-452 `deploy-app.sh`)
- A GeoIP fetch script or systemd timer (golink self-fetches; no external machinery needed)
- A MaxMind license key workflow (DB-IP needs no key)
