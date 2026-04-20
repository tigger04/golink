// ABOUTME: Static (lookup) resolver. Maps exact path slugs to fixed destination
// ABOUTME: URLs via a YAML routes table. No template substitution.

package static

import (
	"fmt"
	"os"
	"strings"

	"github.com/tigger04/golink/internal/resolver"
	"gopkg.in/yaml.v3"
)

// Config is the on-disk YAML schema for a static resolver.
type Config struct {
	Type    string            `yaml:"type"`
	Routes  map[string]string `yaml:"routes"`
	Default string            `yaml:"default"`
}

// Resolver is a static lookup resolver instance.
type Resolver struct {
	routes map[string]string
	dflt   string
}

// LoadFile reads a YAML file and returns a configured Resolver.
func LoadFile(path string) (*Resolver, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read resolver file: %w", err)
	}
	return Load(data)
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

// Resolve looks up the request path in the routes table. Returns the matched
// URL, falls back to the default if set, or returns ErrNotFound.
func (r *Resolver) Resolve(req resolver.Request) (resolver.Result, error) {
	path := strings.Trim(req.Path, "/")
	if path == "" {
		return resolver.Result{}, resolver.ErrNotFound
	}

	if url, ok := r.routes[path]; ok {
		return resolver.Result{URL: url, Code: 302}, nil
	}

	if r.dflt != "" {
		return resolver.Result{URL: r.dflt, Code: 302}, nil
	}

	return resolver.Result{}, resolver.ErrNotFound
}
