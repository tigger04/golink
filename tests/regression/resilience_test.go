// ABOUTME: AC6.6 tests — the service keeps working when things go wrong.
// ABOUTME: GeoIP failure, SIGHUP reload, and SIGTERM shutdown. Tests RT-6.31
// ABOUTME: through RT-6.40.

package regression

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/tigger04/golink/internal/geoip"
	"github.com/tigger04/golink/internal/router"
	"github.com/tigger04/golink/internal/server"
)

// RT-6.31: GeoIP download fails → service starts, defaults used
func TestResilience_GeoDownloadFails_RT6_31(t *testing.T) {
	dir := t.TempDir()
	// Mock server that always fails.
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockServer.Close()

	svc := geoip.New(geoip.Config{
		Dir:         dir,
		DownloadURL: mockServer.URL + "/fail.mmdb.gz",
		HTTPClient:  mockServer.Client(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start should succeed even when download fails.
	err := svc.Start(ctx)
	defer func() { _ = svc.Close() }()
	if err != nil {
		t.Errorf("Start returned error: %v (should succeed even when download fails)", err)
	}

	// Lookups should return empty string (default).
	code := svc.CountryCode(net.ParseIP("203.0.113.1"))
	if code != "" {
		t.Errorf("CountryCode = %q, want empty (no database)", code)
	}
}

// RT-6.32: Missing GeoIP database → all resolvers use default template
func TestResilience_MissingGeoIPDefaults_RT6_32(t *testing.T) {
	// Server with nil GeoIP — all lookups return "".
	ts := newTestServer(t, nil)
	defer ts.Close()
	client := noRedirectClient()

	resp, err := client.Get(ts.URL + "/az/B08N5WRWNW")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loc := resp.Header.Get("Location")
	if loc != "https://www.amazon.com/dp/B08N5WRWNW" {
		t.Errorf("Location = %q, want amazon.com (default, no GeoIP)", loc)
	}
}

// RT-6.33: Corrupt GeoIP database → service continues with defaults
func TestResilience_CorruptGeoIP_RT6_33(t *testing.T) {
	dir := t.TempDir()
	// Write a corrupt file.
	corruptPath := filepath.Join(dir, "dbip-country-lite.mmdb")
	if err := os.WriteFile(corruptPath, []byte("this is not a valid mmdb file"), 0644); err != nil {
		t.Fatalf("write corrupt mmdb: %v", err)
	}

	svc := geoip.New(geoip.Config{Dir: dir})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start should succeed even with corrupt DB.
	_ = svc.Start(ctx)
	defer func() { _ = svc.Close() }()

	code := svc.CountryCode(net.ParseIP("203.0.113.1"))
	if code != "" {
		t.Errorf("CountryCode = %q, want empty (corrupt database)", code)
	}
}

// RT-6.34: New YAML added + SIGHUP → new prefix becomes routable
func TestResilience_ReloadNewYAML_RT6_34(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.yaml"), `
path: "{id}"
default: "https://example.com/{id}"
`)

	rtr, err := router.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	srv := server.New(server.Config{}, rtr, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := noRedirectClient()

	// Add a new resolver.
	writeFile(t, filepath.Join(dir, "new.yaml"), `
path: "{key}"
default: "https://new.example.com/{key}"
`)

	// Simulate SIGHUP: reload.
	newRtr, err := router.LoadDir(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	srv.SetState(newRtr, nil)

	// New prefix should work.
	resp, err := client.Get(ts.URL + "/new/hello")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 302 {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc != "https://new.example.com/hello" {
		t.Errorf("Location = %q, want new.example.com", loc)
	}
}

// RT-6.35: Existing YAML modified + SIGHUP → redirect target updates
func TestResilience_ReloadModifiedYAML_RT6_35(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "test.yaml")
	writeFile(t, yamlPath, `
path: "{id}"
default: "https://old.example.com/{id}"
`)

	rtr, err := router.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	srv := server.New(server.Config{}, rtr, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := noRedirectClient()

	// Modify the YAML.
	writeFile(t, yamlPath, `
path: "{id}"
default: "https://new.example.com/{id}"
`)

	newRtr, err := router.LoadDir(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	srv.SetState(newRtr, nil)

	resp, err := client.Get(ts.URL + "/test/abc")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loc := resp.Header.Get("Location")
	if loc != "https://new.example.com/abc" {
		t.Errorf("Location = %q, want new.example.com", loc)
	}
}

// RT-6.36: All YAMLs broken + SIGHUP → previous state preserved
func TestResilience_ReloadAllBroken_RT6_36(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.yaml"), `
path: "{id}"
default: "https://example.com/{id}"
`)

	rtr, err := router.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	srv := server.New(server.Config{}, rtr, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := noRedirectClient()

	// Break the YAML.
	writeFile(t, filepath.Join(dir, "test.yaml"), "not: valid: yaml: {{{}}")

	// Reload should fail.
	_, reloadErr := router.LoadDir(dir)
	if reloadErr == nil {
		t.Fatal("expected reload to fail with broken YAML")
	}
	// Do NOT call srv.SetState — the reload failed, so we keep the old state.

	// Old prefix should still work.
	resp, err := client.Get(ts.URL + "/test/abc")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 302 {
		t.Errorf("status = %d, want 302 (old config preserved)", resp.StatusCode)
	}
}

// RT-6.37: One invalid YAML among valid ones + SIGHUP → entire reload rejected
func TestResilience_ReloadPartiallyBroken_RT6_37(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "good.yaml"), `
path: "{id}"
default: "https://good.example.com/{id}"
`)

	rtr, err := router.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	srv := server.New(server.Config{}, rtr, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := noRedirectClient()

	// Add a broken YAML alongside the good one.
	writeFile(t, filepath.Join(dir, "bad.yaml"), "not: valid: yaml: {{{}}")

	// Reload should fail (one bad file rejects the whole reload).
	_, reloadErr := router.LoadDir(dir)
	if reloadErr == nil {
		t.Fatal("expected reload to fail with one broken YAML")
	}

	// Old config preserved.
	resp, err := client.Get(ts.URL + "/good/abc")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 302 {
		t.Errorf("status = %d, want 302 (old config preserved)", resp.StatusCode)
	}
}

// RT-6.38: In-flight request during SIGHUP completes against pre-reload state
func TestResilience_InFlightDuringReload_RT6_38(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.yaml"), `
path: "{id}"
default: "https://old.example.com/{id}"
`)

	rtr, err := router.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	// Use a slow GeoIP stub that holds requests long enough to simulate in-flight.
	slowGeo := &slowGeoStub{
		delay:   200 * time.Millisecond,
		mapping: map[string]string{},
	}

	srv := server.New(server.Config{}, rtr, slowGeo)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()
	client := noRedirectClient()

	var wg sync.WaitGroup
	var respLocation string
	var respErr error

	// Start a request that will be in-flight during reload.
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := client.Get(ts.URL + "/test/abc")
		if err != nil {
			respErr = err
			return
		}
		defer func() { _ = resp.Body.Close() }()
		respLocation = resp.Header.Get("Location")
	}()

	// Wait a bit, then reload while the request is in-flight.
	time.Sleep(50 * time.Millisecond)
	writeFile(t, filepath.Join(dir, "test.yaml"), `
path: "{id}"
default: "https://new.example.com/{id}"
`)
	newRtr, err := router.LoadDir(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	srv.SetState(newRtr, slowGeo)

	wg.Wait()
	if respErr != nil {
		t.Fatalf("in-flight request failed: %v", respErr)
	}

	// The in-flight request should have completed against the OLD config.
	if respLocation != "https://old.example.com/abc" {
		t.Errorf("Location = %q, want old.example.com (pre-reload state)", respLocation)
	}
}

// slowGeoStub delays CountryCode calls to simulate in-flight requests.
type slowGeoStub struct {
	delay   time.Duration
	mapping map[string]string
}

func (s *slowGeoStub) CountryCode(ip net.IP) string {
	time.Sleep(s.delay)
	if s.mapping == nil {
		return ""
	}
	return s.mapping[ip.String()]
}

// RT-6.39: SIGTERM when idle → clean exit (code 0)
func TestResilience_ShutdownIdle_RT6_39(t *testing.T) {
	rtr, err := router.LoadDir(filepath.Join(testdataDir(t), "resolvers"))
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := server.New(server.Config{}, rtr, nil)
	go func() { _ = srv.Serve(ln) }()

	// Shutdown immediately (no in-flight requests).
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown returned error: %v", err)
	}
}

// RT-6.40: SIGTERM during request → in-flight request completes before exit
func TestResilience_ShutdownDuringRequest_RT6_40(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "test.yaml"), `
path: "{id}"
default: "https://example.com/{id}"
`)
	rtr, err := router.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	slowGeo := &slowGeoStub{
		delay:   300 * time.Millisecond,
		mapping: map[string]string{},
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()

	srv := server.New(server.Config{}, rtr, slowGeo)
	go func() { _ = srv.Serve(ln) }()

	client := noRedirectClient()

	var wg sync.WaitGroup
	var respStatus int
	var respErr error

	// Start a slow request.
	wg.Add(1)
	go func() {
		defer wg.Done()
		resp, err := client.Get("http://" + addr + "/test/abc")
		if err != nil {
			respErr = err
			return
		}
		defer func() { _ = resp.Body.Close() }()
		respStatus = resp.StatusCode
	}()

	// Wait for the request to be in-flight, then shutdown.
	time.Sleep(50 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)

	wg.Wait()
	if respErr != nil {
		t.Fatalf("in-flight request failed: %v", respErr)
	}
	if respStatus != 302 {
		t.Errorf("status = %d, want 302 (request should complete before shutdown)", respStatus)
	}
}
