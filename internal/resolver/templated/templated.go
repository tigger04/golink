// ABOUTME: YAML-driven templated resolver. Matches path segments against a
// ABOUTME: template, optionally branches on country code, and substitutes
// ABOUTME: captured variables into a URL template.

package templated

import (
	"fmt"
	"os"
	"strings"

	"github.com/tigger04/golink/internal/resolver"
	"gopkg.in/yaml.v3"
)

// Config is the on-disk YAML schema for a templated resolver.
type Config struct {
	Type    string            `yaml:"type"`
	Path    string            `yaml:"path"`
	Default string            `yaml:"default"`
	Geo     map[string]string `yaml:"geo"`
}

// Resolver is a single templated resolver instance, built from a parsed YAML.
type Resolver struct {
	segments []string          // template path segments, e.g. ["{user}", "{repo}"]
	vars     []string          // variable names extracted from segments
	dflt     string            // default URL template
	geo      map[string]string // country code → URL template
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
	if cfg.Default == "" {
		return nil, fmt.Errorf("resolver YAML missing required field: default")
	}

	var segments []string
	if cfg.Path != "" {
		segments = strings.Split(cfg.Path, "/")
	}
	vars := make([]string, 0, len(segments))
	for _, seg := range segments {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			vars = append(vars, seg[1:len(seg)-1])
		}
	}

	return &Resolver{
		segments: segments,
		vars:     vars,
		dflt:     cfg.Default,
		geo:      cfg.Geo,
	}, nil
}

// Resolve matches the request path against the template, looks up a
// country-specific URL if available, substitutes captured variables, and
// returns the redirect target.
func (r *Resolver) Resolve(req resolver.Request) (resolver.Result, error) {
	captured, ok := r.match(req.Path)
	if !ok {
		return resolver.Result{}, resolver.ErrNotFound
	}

	tmpl := r.dflt
	if req.CountryCode != "" && r.geo != nil {
		if geoTmpl, exists := r.geo[strings.ToUpper(req.CountryCode)]; exists {
			tmpl = geoTmpl
		}
	}

	url := tmpl
	for name, value := range captured {
		url = strings.ReplaceAll(url, "{"+name+"}", value)
	}

	return resolver.Result{URL: url, Code: 302}, nil
}

// match splits the request path on "/" and checks it has exactly the right
// number of segments. Returns captured variable values keyed by name.
func (r *Resolver) match(path string) (map[string]string, bool) {
	// Trim leading/trailing slashes and split.
	path = strings.Trim(path, "/")
	if path == "" {
		if len(r.segments) == 0 {
			return map[string]string{}, true
		}
		return nil, false
	}

	parts := strings.Split(path, "/")
	if len(parts) != len(r.segments) {
		return nil, false
	}

	captured := make(map[string]string, len(r.vars))
	for i, seg := range r.segments {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			name := seg[1 : len(seg)-1]
			captured[name] = parts[i]
		} else if parts[i] != seg {
			return nil, false
		}
	}
	return captured, true
}
