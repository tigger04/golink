<!-- Version: 0.4 | Last updated: 2026-04-11 -->

# Architecture

## Purpose

golink is a single-binary HTTP redirect service. It returns only 3xx redirects (or 404 on miss). It is domain-agnostic: the binary knows nothing about its public hostname, which is a deployment-time concern handled by the reverse proxy and DNS.

The first concrete redirect type the project ships is geo-aware Amazon ASIN forwarding (e.g. `https://{host}/az/{asin}`), but the design is fully data-driven: new redirect types are added by dropping a YAML file into a directory, with no Go code changes.

## Design principles

1. **Redirect-only.** No HTML, no JSON, no bodies. Successful responses are 302. Misses are 404.
2. **Data-driven.** Each redirect type is described entirely by a YAML file. The Go binary contains one generic resolver implementation; behaviour comes from config.
3. **Stateless.** No database. Resolver YAMLs and the GeoIP file are loaded at startup and reloaded on SIGHUP.
4. **Single binary.** Built statically with `go build`. Deployed as one file plus a YAML directory and a GeoIP `.mmdb` file. Managed by systemd on the Hetzner host.
5. **No third-party calls on the request path.** GeoIP is local. Configuration is in memory. The hot path is: parse URL, look up resolver by prefix, extract path variables, optionally look up country, substitute template, write redirect header.
6. **Extensible by drop-in.** Adding a new redirect type means writing one YAML file and sending SIGHUP. No Go code, no deploy.

## High-level shape

```
                +-------------------------+
   HTTPS  --->  |  net/http server (Go)   |
   :443         |  fronted by Caddy on    |
                |  the Hetzner host       |
                +-----------+-------------+
                            |
                            v
                +-------------------------+
                |  Router                 |
                |  prefix -> resolver     |
                +-----------+-------------+
                            |
                            v
                +-------------------------+
                |  Templated resolver     |
                |  (one Go impl,          |
                |   many YAML configs)    |
                +-----------+-------------+
                            |
                            v
                +-------------------------+
                |  GeoIP service          |
                |  MaxMind GeoLite2       |
                |  (mmap'd .mmdb file)    |
                +-------------------------+
```

## Why 302 (temporary), not 301 (permanent)

301 responses are cacheable by browsers and intermediaries. A user who hits `/az/B0XYZ` once and gets a 301 may never ask golink again - their browser jumps straight to the cached destination forever. For a geo-aware service that is exactly wrong: if the user travels, or if the YAML market table changes, the cached 301 keeps sending them to the old destination. 302 forces every request to come back through golink so the routing decision stays live. The cost is one round-trip to a service that does almost nothing.

## Components

### `cmd/golink`

`main.go`. Parses CLI flags, loads the GeoIP database, scans the resolvers directory and registers each YAML as a resolver, starts the HTTP server, installs signal handlers (SIGHUP = reload resolvers + GeoIP, SIGTERM = graceful shutdown).

### `internal/geoip`

Wraps `github.com/oschwald/maxminddb-golang` over a DB-IP IP-to-Country Lite database. Single hot-path method: `CountryCode(net.IP) string` returning an ISO 3166-1 alpha-2 code, or `""` on lookup failure. Owns the database lifecycle: downloads from DB-IP on startup if missing/stale, runs a daily refresh check, atomically swaps the live DB on successful refresh.

### `internal/router`

Holds an immutable `prefix -> Resolver` map built at startup (and rebuilt on reload). Matches the first path segment. Unknown prefix returns 404. No regex routing.

### `internal/resolver`

Defines the core types:

```go
type Request struct {
    Path        string  // path segments after the prefix, joined
    CountryCode string  // ISO alpha-2, "" if unknown
    RemoteIP    net.IP
}

type Result struct {
    URL  string
    Code int  // 302
}

type Resolver interface {
    Resolve(Request) (Result, error)
}
```

`ErrNotFound` is the sentinel for "I understood the request but have nothing for it" -> 404.

### `internal/resolver/templated`

The only resolver implementation in v1. Constructed from a parsed YAML file. Knows how to:

1. Match the request's remaining path against a `path` template, extracting named variables.
2. Optionally look up the client's country code.
3. Pick a URL template - the country-specific one if present, otherwise the `default`.
4. Substitute the captured variables into the template.
5. Return the result as a 302.

If the path doesn't match the template (wrong number of segments, etc.), return `ErrNotFound`. The resolver does no other validation - it does not, for example, check that an ASIN looks like an ASIN. Amazon will return its own 404 for a malformed identifier. Validation is not our job.

### `internal/server`

`net/http` wiring, request logging, panic recovery, graceful shutdown. Listens on a configurable port (CLI flag).

## Configuration

There is no global YAML config. Server-level settings are CLI flags:

```
golink \
  -listen :8080 \
  -geoip /var/lib/golink/GeoLite2-Country.mmdb \
  -resolvers-dir /etc/golink/resolvers
```

All redirect behaviour lives in `*.yaml` files inside the resolvers directory. The filename (minus `.yaml`) becomes the URL prefix.

### Resolver YAML schema

```yaml
# Optional. Defaults to "templated". Reserved for future resolver types.
type: templated

# Path pattern matched against the request path *after* the URL prefix.
# {name} segments are captured and made available for template substitution.
path: "{var1}/{var2}"

# Default URL template. Used when no geo entry matches, or when GeoIP
# returns no country.
default: "https://example.com/{var1}/{var2}"

# Optional. Country-specific URL templates, keyed by ISO 3166-1 alpha-2.
geo:
  DE: "https://example.de/{var1}/{var2}"
  FR: "https://example.fr/{var1}/{var2}"
```

## Example resolvers

These three illustrate what the templated resolver can express. All are real plausible uses, not hypothetical.

### `az.yaml` - geo-aware Amazon ASIN forwarding

```yaml
# {host}/az/{asin}
path: "{asin}"
default: "https://www.amazon.com/dp/{asin}"
geo:
  # Native local markets
  AE: "https://www.amazon.ae/dp/{asin}"
  BE: "https://www.amazon.com.be/dp/{asin}"
  CA: "https://www.amazon.ca/dp/{asin}"
  DE: "https://www.amazon.de/dp/{asin}"
  EG: "https://www.amazon.eg/dp/{asin}"
  ES: "https://www.amazon.es/dp/{asin}"
  FR: "https://www.amazon.fr/dp/{asin}"
  GB: "https://www.amazon.co.uk/dp/{asin}"
  IE: "https://www.amazon.ie/dp/{asin}"
  IN: "https://www.amazon.in/dp/{asin}"
  IT: "https://www.amazon.it/dp/{asin}"
  JP: "https://www.amazon.co.jp/dp/{asin}"
  NL: "https://www.amazon.nl/dp/{asin}"
  PL: "https://www.amazon.pl/dp/{asin}"
  PT: "https://www.amazon.pt/dp/{asin}"
  SA: "https://www.amazon.sa/dp/{asin}"
  SE: "https://www.amazon.se/dp/{asin}"
  SG: "https://www.amazon.sg/dp/{asin}"
  # Non-market countries routed to the closest local market
  AT: "https://www.amazon.de/dp/{asin}"
  CH: "https://www.amazon.de/dp/{asin}"
  CZ: "https://www.amazon.de/dp/{asin}"
  DK: "https://www.amazon.de/dp/{asin}"
  IS: "https://www.amazon.co.uk/dp/{asin}"
  NO: "https://www.amazon.de/dp/{asin}"
  # ...etc - full list lives in the deployed file
```

The full list is derived from a one-time empirical probe of `amazon.{tld}` for all plausible country codes (see commit history of `az.yaml`). Anything not listed falls back to `default`. New Amazon markets need no urgent action - users in those countries simply land on `.com` until someone updates the YAML.

### `gh.yaml` - GitHub shortcut, no geo

```yaml
# {host}/gh/{user}/{repo}
path: "{user}/{repo}"
default: "https://github.com/{user}/{repo}"
```

No `geo` block - GitHub doesn't care where the user is. `{host}/gh/torvalds/linux` redirects to `https://github.com/torvalds/linux`.

### `wiki.yaml` - geo-aware Wikipedia

```yaml
# {host}/wiki/{article}
path: "{article}"
default: "https://en.wikipedia.org/wiki/{article}"
geo:
  DE: "https://de.wikipedia.org/wiki/{article}"
  FR: "https://fr.wikipedia.org/wiki/{article}"
  ES: "https://es.wikipedia.org/wiki/{article}"
  IT: "https://it.wikipedia.org/wiki/{article}"
  PT: "https://pt.wikipedia.org/wiki/{article}"
  NL: "https://nl.wikipedia.org/wiki/{article}"
  PL: "https://pl.wikipedia.org/wiki/{article}"
  JP: "https://ja.wikipedia.org/wiki/{article}"
```

A German user hitting `/wiki/Linux` lands on the German article; everyone else lands on English.

## TLS strategy

golink itself has no TLS code - it speaks plain HTTP on a configurable address (typically a loopback port). TLS termination, the public hostname, and certificate management are entirely the responsibility of whatever reverse proxy fronts it. Any reverse proxy on any host serving any domain works the same way.

## GeoIP database

- **Source:** [DB-IP IP-to-Country Lite](https://db-ip.com/db/lite/ip-to-country) - free, CC BY 4.0, no account or license key required. Direct download from `https://download.db-ip.com/free/dbip-country-lite-YYYY-MM.mmdb.gz`. The same MMDB format MaxMind uses, read by the same Go library (`oschwald/maxminddb-golang`).
- **Self-managed.** golink owns its own database file. On startup, if the local `.mmdb` is missing or older than 30 days, it downloads the latest from DB-IP, gunzips it, and atomically replaces the live file. A goroutine inside golink wakes once a day to check whether a refresh is due. **No external systemd timer, no fetch script, no secrets.**
- **Storage location.** The default path is `${STATE_DIRECTORY:-./}/dbip-country-lite.mmdb`. On the production host this resolves to `/var/lib/golink/dbip-country-lite.mmdb` because systemd sets `STATE_DIRECTORY` from the unit's `StateDirectory=` setting.
- **Failure modes.**
  - Initial download fails (no network on first start): golink starts anyway and serves every request as if the country were unknown - i.e. all redirects fall back to the resolver `default` template. Logs an error. Retries on the next refresh tick.
  - Periodic refresh fails: previous DB stays loaded, error logged, retry on the next tick.
  - DB file corrupted on disk between starts: treated as missing; re-fetch.
- **Attribution.** "This product includes IP geolocation data created by [DB-IP.com](https://db-ip.com)" appears in the README per CC BY 4.0.

## Reload (SIGHUP)

`systemctl reload golink` (or `kill -HUP <pid>`) triggers golink to:

1. Re-scan the resolvers directory and re-parse every `*.yaml` file.
2. Re-open the GeoIP database file.
3. If both succeed, atomically swap the live state.
4. If either fails, the previous state stays live and an error is logged. **Reload never breaks a running service.**

This is the standard Unix idiom (nginx, sshd, postfix all do the same). It avoids exposing an HTTP admin endpoint, which would need authentication and add attack surface for no benefit.

## Logging

Structured JSON logs to stderr; systemd captures them into the journal. Read with `journalctl -u golink`.

Logs are for development and incident diagnosis. There is no metrics endpoint, no Prometheus, no Grafana - this is a personal-use service handling a small number of requests, and the journal is enough.

Per request: `ts`, `remote_ip`, `country`, `prefix`, `path`, `status`, `target`, `latency_us`.

## Errors and status codes

| Situation                                   | Status |
|--------------------------------------------|--------|
| Successful resolution                       | 302    |
| Unknown path prefix                         | 404    |
| Path doesn't match resolver template        | 404    |
| Resolver returns `ErrNotFound`              | 404    |
| Internal error (panic, GeoIP failure mid-request) | 500 |

The body of any non-redirect response is empty. No error pages.

## Extensibility

To add a new redirect type:

1. Write a YAML file matching the schema above.
2. Drop it into the resolvers directory.
3. `systemctl reload golink`.

That's it. No Go code, no rebuild, no deploy.

When the templated resolver isn't expressive enough for some future use case (e.g. branch on time of day, user-agent, query parameters, multi-step lookups), a second resolver type can be added. YAMLs would then declare `type: <name>` to opt in. The default stays `templated`.

## Licensing and attribution

golink itself is **MIT licensed** (Copyright Taḋg Paul). The full text lives in `LICENSE` at the repository root.

The bundled GeoIP data - DB-IP IP-to-Country Lite - is third-party content under **Creative Commons Attribution 4.0 International (CC BY 4.0)**. CC BY 4.0 requires that users of the data:

1. Give appropriate credit to the creator (DB-IP)
2. Provide a link to the license
3. Indicate if changes were made

To meet these requirements, the project carries attribution in three places:

- **`README.md`** - an "Acknowledgements" section with the standard DB-IP attribution and a link to the CC BY 4.0 license text. Visible to anyone who reads the project page.
- **`THIRD-PARTY-NOTICES.md`** at the repository root - formal notice listing every third-party asset (currently just DB-IP), its license, attribution text, and source URL. Conventional for projects bundling or downloading external data.
- **At runtime** - golink prints a one-line attribution to stderr on startup (alongside the version banner), and exposes a `-credits` flag that prints the full third-party notices and exits. Ensures attribution is visible to operators of the running service even if they never look at the source.

Because golink downloads the database at runtime rather than bundling it in the binary, no redistribution of the data file occurs from this repository - but the running service uses the data, and the operator's exposure to it is mediated by golink, so the attribution is part of golink's user-visible surface.

## Out of scope (for v1)

- Authentication / private links
- Click analytics or persistent storage
- Per-link expiry
- Admin UI or API for editing config (config is file-driven, edited by hand or by ops tooling, reloaded via SIGHUP)
- A/B routing or weighted redirects
- Metrics endpoint / Prometheus
- HTTPS termination inside golink

These are deferred. The resolver interface is general enough to host them later if needed.
