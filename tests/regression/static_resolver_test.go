// ABOUTME: Tests for the static (lookup) resolver. Verifies exact-match
// ABOUTME: routing, default fallback, and 404 for unknown slugs.

package regression

import (
	"path/filepath"
	"testing"

	"github.com/tigger04/golink/internal/resolver"
	"github.com/tigger04/golink/internal/resolver/static"
	"github.com/tigger04/golink/internal/router"
)

// Static resolver unit tests — Load from YAML bytes.

func TestStaticResolver_ExactMatch(t *testing.T) {
	yaml := `
type: static
routes:
  platys: "https://example.com/platys-page"
  draft: "https://example.com/draft-page"
default: "https://example.com/fallback"
`
	res, err := static.Load([]byte(yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	result, err := res.Resolve(resolver.Request{Path: "platys"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.URL != "https://example.com/platys-page" {
		t.Errorf("URL = %q, want %q", result.URL, "https://example.com/platys-page")
	}
	if result.Code != 302 {
		t.Errorf("Code = %d, want 302", result.Code)
	}
}

func TestStaticResolver_MultipleRoutes(t *testing.T) {
	yaml := `
type: static
routes:
  platys: "https://example.com/platys"
  draft: "https://example.com/draft"
default: "https://example.com/fallback"
`
	res, err := static.Load([]byte(yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	for _, tc := range []struct {
		path string
		want string
	}{
		{"platys", "https://example.com/platys"},
		{"draft", "https://example.com/draft"},
	} {
		result, err := res.Resolve(resolver.Request{Path: tc.path})
		if err != nil {
			t.Errorf("Resolve(%q): %v", tc.path, err)
			continue
		}
		if result.URL != tc.want {
			t.Errorf("Resolve(%q) = %q, want %q", tc.path, result.URL, tc.want)
		}
	}
}

func TestStaticResolver_DefaultFallback(t *testing.T) {
	yaml := `
type: static
routes:
  platys: "https://example.com/platys"
default: "https://example.com/fallback"
`
	res, err := static.Load([]byte(yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	result, err := res.Resolve(resolver.Request{Path: "unknown-slug"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.URL != "https://example.com/fallback" {
		t.Errorf("URL = %q, want %q", result.URL, "https://example.com/fallback")
	}
}

func TestStaticResolver_NoDefaultReturnsNotFound(t *testing.T) {
	yaml := `
type: static
routes:
  platys: "https://example.com/platys"
`
	res, err := static.Load([]byte(yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	_, err = res.Resolve(resolver.Request{Path: "unknown-slug"})
	if err != resolver.ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestStaticResolver_EmptyPathReturnsNotFound(t *testing.T) {
	yaml := `
type: static
routes:
  platys: "https://example.com/platys"
default: "https://example.com/fallback"
`
	res, err := static.Load([]byte(yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	_, err = res.Resolve(resolver.Request{Path: ""})
	if err != resolver.ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound for empty path", err)
	}
}

func TestStaticResolver_LeadingTrailingSlashesStripped(t *testing.T) {
	yaml := `
type: static
routes:
  platys: "https://example.com/platys"
`
	res, err := static.Load([]byte(yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	result, err := res.Resolve(resolver.Request{Path: "/platys/"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.URL != "https://example.com/platys" {
		t.Errorf("URL = %q, want %q", result.URL, "https://example.com/platys")
	}
}

func TestStaticResolver_NoRoutesMissingDefault(t *testing.T) {
	yaml := `
type: static
`
	_, err := static.Load([]byte(yaml))
	if err == nil {
		t.Error("Load succeeded with no routes and no default, want error")
	}
}

func TestStaticResolver_EmptyRoutesWithDefault(t *testing.T) {
	yaml := `
type: static
default: "https://example.com/catchall"
`
	res, err := static.Load([]byte(yaml))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	result, err := res.Resolve(resolver.Request{Path: "anything"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result.URL != "https://example.com/catchall" {
		t.Errorf("URL = %q, want %q", result.URL, "https://example.com/catchall")
	}
}

// Integration: LoadDir dispatches static and templated resolvers from mixed directory.

func TestLoadDir_MixedResolverTypes(t *testing.T) {
	rtr, err := router.LoadDir(filepath.Join(testdataDir(t), "resolvers"))
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	// Static resolver should be loaded.
	if rtr.Lookup("links") == nil {
		t.Error("static resolver 'links' not registered")
	}

	// Templated resolvers should still work.
	for _, name := range []string{"az", "gh", "wiki"} {
		if rtr.Lookup(name) == nil {
			t.Errorf("templated resolver %q not registered", name)
		}
	}
}

func TestLoadDir_StaticResolverIntegration(t *testing.T) {
	ts := newTestServer(t, nil)
	defer ts.Close()
	client := noRedirectClient()

	resp, err := client.Get(ts.URL + "/links/docs")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loc := resp.Header.Get("Location")
	want := "https://example.com/documentation"
	if loc != want {
		t.Errorf("Location = %q, want %q", loc, want)
	}
}
