// ABOUTME: URL prefix router. Maps the first path segment to a resolver.
// ABOUTME: Immutable after construction; rebuilt on SIGHUP reload.

package router

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tigger04/golink/internal/resolver"
	"github.com/tigger04/golink/internal/resolver/static"
	"github.com/tigger04/golink/internal/resolver/templated"
	"gopkg.in/yaml.v3"
)

// Router holds an immutable prefix → Resolver map.
type Router struct {
	resolvers map[string]resolver.Resolver
}

// New creates a Router from a pre-built resolver map.
func New(resolvers map[string]resolver.Resolver) *Router {
	return &Router{resolvers: resolvers}
}

// Lookup returns the resolver registered for the given prefix, or nil.
func (r *Router) Lookup(prefix string) resolver.Resolver {
	return r.resolvers[prefix]
}

// Prefixes returns the list of registered prefixes (for logging).
func (r *Router) Prefixes() []string {
	out := make([]string, 0, len(r.resolvers))
	for k := range r.resolvers {
		out = append(out, k)
	}
	return out
}

// LoadDir scans a directory for *.yaml files and builds a Router. Each file's
// stem (filename without .yaml) becomes the URL prefix. Returns an error if
// any YAML file fails to parse — the caller decides whether to abort (startup)
// or keep the old state (reload).
func LoadDir(dir string) (*Router, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read resolver directory: %w", err)
	}

	resolvers := make(map[string]resolver.Resolver)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}

		path := filepath.Join(dir, name)
		r, err := loadResolver(path)
		if err != nil {
			return nil, fmt.Errorf("load resolver %s: %w", name, err)
		}

		prefix := strings.TrimSuffix(name, ".yaml")
		resolvers[prefix] = r
	}

	return New(resolvers), nil
}

// typeHint peeks at the type field in a YAML file without fully parsing it.
type typeHint struct {
	Type string `yaml:"type"`
}

// loadResolver reads a YAML file and dispatches to the correct resolver
// implementation based on the type field. Defaults to templated if absent.
func loadResolver(path string) (resolver.Resolver, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read resolver file: %w", err)
	}

	var hint typeHint
	if err := yaml.Unmarshal(data, &hint); err != nil {
		return nil, fmt.Errorf("peek type in %s: %w", path, err)
	}

	switch hint.Type {
	case "static":
		return static.Load(data)
	case "templated", "":
		return templated.Load(data)
	default:
		return nil, fmt.Errorf("unknown resolver type %q in %s", hint.Type, path)
	}
}
