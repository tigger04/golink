# golink

A small self-hosted HTTP redirect service. Domain-agnostic — the binary knows nothing about its public hostname; that is the reverse proxy's concern.

> **Status: v0 hello-world.** This repository currently contains a placeholder hello-world handler used to prove the deploy pipeline to kepler-452. The real resolver layer (geo-aware Amazon ASIN forwarding, GitHub shortcuts, language-aware Wikipedia) lands in a follow-up issue. See [`docs/architecture.md`](docs/architecture.md) for the target design and [`docs/implementation-plan.md`](docs/implementation-plan.md) for the rollout plan.

## Quickstart

```bash
git clone git@github.com:tigger04/golink.git
cd golink
make build         # builds ./bin/golink for the host OS
./bin/golink       # serves http://127.0.0.1:18081 by default
```

In another terminal:

```bash
curl -v http://127.0.0.1:18081/
# expected: HTTP 200, body "hello from golink dev\n"
```

The bind address is taken from the environment, in order:

1. `ADDR` (full host:port, e.g. `127.0.0.1:18081`)
2. `PORT` (port only, bound to `127.0.0.1`)
3. fallback `127.0.0.1:18081`

This matches the convention enforced by [kepler-452's deploy contract](../../hetzner/kepler-452/docs/PROJECT-INTEGRATION.md).

## Make targets

| Target | Purpose |
|---|---|
| `make build` | Build host-OS binary into `./bin/golink` (for local dev) |
| `make test` | Run lint + regression tests |
| `make lint` | `go vet ./...` (golangci-lint upgrade tracked in issue #7) |
| `make deploy` | Push to origin, then git-pull + build on kepler-452 via `deploy-app.sh` |
| `make logs` | Tail `journalctl -u golink` over Tailscale |
| `make status` | `systemctl status golink` + recent journal over Tailscale |
| `make clean` | Remove build artefacts |

`make deploy` requires a clean working tree (all changes committed and pushed). The build happens on the server — no cross-compilation needed. Expects `~/code/hetzner/kepler-452/deploy-app.sh` to be present (override with `HETZNER_REPO=…`).

## Deployment

golink is deployed to `kepler-452` (a Hetzner VPS on the operator's tailnet) via the deploy machinery in [`~/code/hetzner/kepler-452/`](../../hetzner/kepler-452/). The deploy script handles the systemd unit, sandbox hardening, atomic install, and restart. golink itself ships only the Go binary and this Makefile target.

### Caddyfile entry (one-time setup)

Caddy on kepler-452 needs a site stanza pointing at golink's loopback port. Add the following to `~/code/hetzner/kepler-452/Caddyfile` and run `kepler-452/setup.sh deploy`:

```caddyfile
go.tigger.dev {
    tls {
        issuer acme {
            disable_http_challenge
        }
    }
    reverse_proxy 127.0.0.1:18081
    encode gzip
}
```

The `disable_http_challenge` line is required because port 80 is closed at the Hetzner cloud firewall — Caddy uses TLS-ALPN-01 over port 443 for ACME challenges.

### DNS

Add an `A` record for `go.tigger.dev` pointing at kepler-452's public IPv4 (`87.99.147.117`).

## Repository layout

```
golink/
├── cmd/golink/main.go       # entrypoint
├── tests/regression/        # regression tests run by `make test`
├── tests/one_off/           # one-off tests (none in v0)
├── docs/                    # architecture, implementation plan
├── Makefile
├── go.mod
└── LICENSE                  # MIT, Copyright Taḋg Paul
```

## Documentation

- [`docs/architecture.md`](docs/architecture.md) — target architecture for the full v1
- [`docs/implementation-plan.md`](docs/implementation-plan.md) — phased rollout plan
- [`docs/VISION.md`](docs/VISION.md) — original vision

## License

MIT, Copyright (c) 2026 Taḋg Paul. See [`LICENSE`](LICENSE).
