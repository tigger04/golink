// ABOUTME: Tests for the static (lookup) resolver. Verifies exact-match
// ABOUTME: routing, default fallback, and 404 for unknown slugs.

package regression

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/tigger04/golink/internal/resolver"
	"github.com/tigger04/golink/internal/resolver/static"
	"github.com/tigger04/golink/internal/router"
	"github.com/tigger04/golink/internal/server"
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

// Internal dispatch tests — routes starting with "/" resolve through another resolver.

func TestStaticResolver_InternalDispatchToTemplated(t *testing.T) {
	// End-to-end via HTTP: /links/gocode → internal /gh/golang/go → https://github.com/golang/go
	ts := newTestServer(t, nil)
	defer ts.Close()
	client := noRedirectClient()

	resp, err := client.Get(ts.URL + "/links/gocode")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loc := resp.Header.Get("Location")
	want := "https://github.com/golang/go"
	if loc != want {
		t.Errorf("Location = %q, want %q", loc, want)
	}
	if resp.StatusCode != 302 {
		t.Errorf("Status = %d, want 302", resp.StatusCode)
	}
}

func TestStaticResolver_InternalDispatchGeoAware(t *testing.T) {
	// Internal route through az resolver should respect geo lookup.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "buy.yaml"), `
type: static
routes:
  book: /az/1234567890
`)
	writeFile(t, filepath.Join(dir, "az.yaml"), `
type: templated
path: "{asin}"
default: "https://www.amazon.com/dp/{asin}"
geo:
  DE: "https://www.amazon.de/dp/{asin}"
`)
	rtr, err := router.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	geo := &stubGeo{mapping: map[string]string{"1.2.3.4": "DE"}}
	srv := server.New(server.Config{}, rtr, geo)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := noRedirectClient()

	req, _ := http.NewRequest("GET", ts.URL+"/buy/book", nil)
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loc := resp.Header.Get("Location")
	want := "https://www.amazon.de/dp/1234567890"
	if loc != want {
		t.Errorf("Location = %q, want %q", loc, want)
	}
}

func TestStaticResolver_InternalDispatchSelfReference(t *testing.T) {
	// A static resolver that references itself should fail gracefully.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "loop.yaml"), `
type: static
routes:
  bad: /loop/other
`)
	rtr, err := router.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	srv := server.New(server.Config{}, rtr, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := noRedirectClient()

	resp, err := client.Get(ts.URL + "/loop/bad")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 500 {
		t.Errorf("Status = %d, want 500 for self-referencing route", resp.StatusCode)
	}
}

func TestStaticResolver_InternalDispatchUnknownPrefix(t *testing.T) {
	// A route referencing a non-existent resolver should 404.
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "broken.yaml"), `
type: static
routes:
  bad: /nonexistent/something
`)
	rtr, err := router.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	srv := server.New(server.Config{}, rtr, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := noRedirectClient()

	resp, err := client.Get(ts.URL + "/broken/bad")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 404 {
		t.Errorf("Status = %d, want 404 for unknown internal prefix", resp.StatusCode)
	}
}

func TestStaticResolver_AbsoluteURLUnaffectedByRouter(t *testing.T) {
	// Absolute URLs should still work exactly as before, no internal dispatch.
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
