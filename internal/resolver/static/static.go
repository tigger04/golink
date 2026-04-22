// ABOUTME: Static (lookup) resolver. Maps exact path slugs to fixed destination
// ABOUTME: URLs via a YAML routes table. Routes starting with "/" are resolved
// ABOUTME: internally through the router (e.g. /az/1234 dispatches to the az resolver).

package static

import (
	"fmt"
	"strings"

	"github.com/tigger04/golink/internal/resolver"
	"gopkg.in/yaml.v3"
)

// InternalLookup resolves an internal path through the router. Accepts a
// prefix, remaining path, and the original request (for geo/IP context).
type InternalLookup func(prefix, remaining string, req resolver.Request) (resolver.Result, error)

// Config is the on-disk YAML schema for a static resolver.
type Config struct {
	Routes  map[string]string `yaml:"routes"`
	Default string            `yaml:"default"`
}

// Resolver is a static lookup resolver instance.
type Resolver struct {
	prefix string
	routes map[string]string
	dflt   string
	lookup InternalLookup
}

// Load parses YAML bytes into a Resolver.
func Load(data []byte) (*Resolver, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	if len(cfg.Routes) == 0 && cfg.Default == "" {
		return nil, fmt.Errorf("static resolver requires at least one route or a default")
	}

	return &Resolver{
		routes: cfg.Routes,
		dflt:   cfg.Default,
	}, nil
}

// SetRouter wires up internal dispatch. Called by the router after all
// resolvers are constructed. prefix is this resolver's own URL prefix.
func (r *Resolver) SetRouter(prefix string, lookup InternalLookup) {
	r.prefix = prefix
	r.lookup = lookup
}

// Resolve looks up the request path in the routes table. If the matched URL
// starts with "/", it is resolved internally through the router. Otherwise
// returns the URL directly. Falls back to default, or returns ErrNotFound.
func (r *Resolver) Resolve(req resolver.Request) (resolver.Result, error) {
	path := strings.Trim(req.Path, "/")
	if path == "" {
		return resolver.Result{}, resolver.ErrNotFound
	}

	if url, ok := r.routes[path]; ok {
		return r.resolve(url, req)
	}

	if r.dflt != "" {
		return r.resolve(r.dflt, req)
	}

	return resolver.Result{}, resolver.ErrNotFound
}

// resolve returns the URL directly if absolute, or dispatches internally if
// it starts with "/".
func (r *Resolver) resolve(url string, req resolver.Request) (resolver.Result, error) {
	if !strings.HasPrefix(url, "/") {
		return resolver.Result{URL: url, Code: 302}, nil
	}

	// Internal route: split into prefix + remaining.
	trimmed := strings.TrimPrefix(url, "/")
	targetPrefix, remaining := splitFirst(trimmed)

	if targetPrefix == r.prefix {
		return resolver.Result{}, fmt.Errorf("static resolver %q: self-referencing route %q", r.prefix, url)
	}

	if r.lookup == nil {
		return resolver.Result{}, fmt.Errorf("static resolver: internal route %q but no router configured", url)
	}

	return r.lookup(targetPrefix, remaining, req)
}

// splitFirst splits on the first "/" and returns prefix and remainder.
func splitFirst(path string) (string, string) {
	idx := strings.IndexByte(path, '/')
	if idx < 0 {
		return path, ""
	}
	return path[:idx], path[idx+1:]
}
