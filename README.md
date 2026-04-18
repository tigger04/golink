# golink

A small self-hosted HTTP redirect service. Domain-agnostic - the binary knows nothing about its public hostname; that is the reverse proxy's concern.

## Quickstart

```bash
git clone git@github.com:tigger04/golink.git
cd golink
make build         # builds ./bin/golink for the host OS
./bin/golink       # serves http://127.0.0.1:18081 by default
```

In another terminal:

```bash
curl -sI http://127.0.0.1:18081/az/B08N5WRWNW
# expected: HTTP 302, Location: https://www.amazon.com/dp/B08N5WRWNW

curl -sI http://127.0.0.1:18081/gh/torvalds/linux
# expected: HTTP 302, Location: https://github.com/torvalds/linux

curl -sI http://127.0.0.1:18081/wiki/Linux
# expected: HTTP 302, Location: https://en.wikipedia.org/wiki/Linux
```

The bind address is taken from the environment or config YAML:

1. `ADDR` env var (full host:port, set by systemd)
2. `addr` field in config YAML
3. fallback `127.0.0.1:18081`

## Make targets

| Target | Purpose |
|---|---|
| `make build` | Build host-OS binary into `./bin/golink` (for local dev) |
| `make test` | Run lint + regression tests |
| `make lint` | `go vet ./...` (golangci-lint upgrade tracked in issue #7) |
| `make install` | Build and symlink `golink` + `goreport` to `~/.local/bin/` |
| `make uninstall` | Remove symlinks from `~/.local/bin/` |
| `make clean` | Remove build artefacts |

## Deployment

Deployment is handled by the hetzner repo's `deploy-app` command. See the [deploy contract](~/code/hetzner/deploy/docs/PROJECT-INTEGRATION.md) for details. This repo owns only the application code, tests, and per-host config in `config/`.

## Analytics

Every HTTP request is recorded to a local SQLite database (`analytics.db` in the state directory). Query with the built-in stats subcommand:

```bash
# On the server directly:
sudo golink stats top --last 7d
sudo golink stats link az
sudo golink stats referers
sudo golink stats misses
sudo golink stats unique
sudo golink stats recent --limit 50

# Remotely via the goreport convenience script:
goreport light-hugger stats top --last 7d
goreport light-hugger logs
goreport light-hugger status
```

All reports accept `--last <duration>` (e.g. `24h`, `7d`, `30d`) and `--csv` for machine-readable output.

## Repository layout

```
golink/
├── cmd/golink/
│   ├── main.go                 # entrypoint, config, server startup
│   └── stats.go                # stats subcommand for analytics queries
├── internal/
│   ├── analytics/              # SQLite event store + query methods + CSV output
│   ├── resolver/               # Resolver interface + templated implementation
│   ├── router/                 # prefix → resolver dispatch + directory loader
│   ├── geoip/                  # DB-IP GeoIP wrapper with self-managed lifecycle
│   └── server/                 # HTTP handler, logging, X-Forwarded-For, analytics
├── scripts/
│   └── goreport                # SSH convenience wrapper for remote stats/logs/status
├── examples/resolvers/         # YAML resolver definitions (az, gh, wiki)
├── config/                     # layered YAML config (defaults + per-host)
├── tests/regression/           # regression tests run by `make test`
├── tests/one_off/              # one-off tests
├── docs/                       # architecture, implementation plan
├── Makefile
├── go.mod
└── LICENSE                     # MIT, Copyright Taḋg Paul
```

## Documentation

- [`docs/architecture.md`](docs/architecture.md) - target architecture for the full v1
- [`docs/implementation-plan.md`](docs/implementation-plan.md) - phased rollout plan
- [`docs/VISION.md`](docs/VISION.md) - original vision

## Acknowledgements

This product includes IP geolocation data created by [DB-IP.com](https://db-ip.com), available under the [Creative Commons Attribution 4.0 International Licence](https://creativecommons.org/licenses/by/4.0/).

## License

MIT, Copyright (c) 2026 Taḋg Paul. See [`LICENSE`](LICENSE).
