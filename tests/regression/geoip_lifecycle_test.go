// ABOUTME: AC6.5 tests — GeoIP database self-management lifecycle.
// ABOUTME: Tests RT-6.27 through RT-6.30.

package regression

import (
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tigger04/golink/internal/geoip"
)

// serveGzippedFile creates a test HTTP server that serves a gzipped copy of
// the given file. Returns the server (caller must close) and the URL.
func serveGzippedFile(t *testing.T, path string) *httptest.Server {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		gz := gzip.NewWriter(w)
		_, _ = gz.Write(data)
		_ = gz.Close()
	}))
}

// createMinimalMMDB creates a minimal file that the geoip service can attempt
// to open. For lifecycle tests we primarily care about download/skip/refresh
// logic, not actual lookups.
func createMinimalMMDB(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "dbip-country-lite.mmdb")
	// Write a small but non-empty file. The geoip.Service will fail to open
	// it as a valid MMDB, which is fine — the lifecycle tests check whether
	// download is initiated, not whether lookups succeed.
	if err := os.WriteFile(path, []byte("not-a-real-mmdb-but-thats-ok"), 0644); err != nil {
		t.Fatalf("write test mmdb: %v", err)
	}
	return path
}

// RT-6.27: No database file at startup → download initiated
func TestGeoIPLifecycle_DownloadWhenMissing_RT6_27(t *testing.T) {
	dir := t.TempDir()
	downloaded := false

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloaded = true
		w.Header().Set("Content-Type", "application/gzip")
		gz := gzip.NewWriter(w)
		// Write some bytes (not a real MMDB, but tests download logic).
		_, _ = gz.Write([]byte("fake-mmdb-data"))
		_ = gz.Close()
	}))
	defer mockServer.Close()

	svc := geoip.New(geoip.Config{
		Dir:         dir,
		DownloadURL: mockServer.URL + "/test.mmdb.gz",
		MaxAge:      30 * 24 * time.Hour,
		HTTPClient:  mockServer.Client(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = svc.Start(ctx)
	defer func() { _ = svc.Close() }()

	if !downloaded {
		t.Error("expected download when no database file exists")
	}
}

// RT-6.28: Fresh database (< 30 days) at startup → no download
func TestGeoIPLifecycle_SkipWhenFresh_RT6_28(t *testing.T) {
	dir := t.TempDir()
	createMinimalMMDB(t, dir)
	downloaded := false

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloaded = true
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	svc := geoip.New(geoip.Config{
		Dir:         dir,
		DownloadURL: mockServer.URL + "/test.mmdb.gz",
		MaxAge:      30 * 24 * time.Hour,
		HTTPClient:  mockServer.Client(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = svc.Start(ctx)
	defer func() { _ = svc.Close() }()

	if downloaded {
		t.Error("should not download when database is fresh")
	}
}

// RT-6.29: Stale database (> 30 days) at startup → refresh initiated
func TestGeoIPLifecycle_RefreshWhenStale_RT6_29(t *testing.T) {
	dir := t.TempDir()
	path := createMinimalMMDB(t, dir)
	// Set mtime to 31 days ago.
	staleTime := time.Now().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(path, staleTime, staleTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	downloaded := false
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloaded = true
		w.Header().Set("Content-Type", "application/gzip")
		gz := gzip.NewWriter(w)
		_, _ = gz.Write([]byte("refreshed-mmdb-data"))
		_ = gz.Close()
	}))
	defer mockServer.Close()

	svc := geoip.New(geoip.Config{
		Dir:         dir,
		DownloadURL: mockServer.URL + "/test.mmdb.gz",
		MaxAge:      30 * 24 * time.Hour,
		HTTPClient:  mockServer.Client(),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = svc.Start(ctx)
	defer func() { _ = svc.Close() }()

	if !downloaded {
		t.Error("expected refresh when database is stale")
	}
}

// RT-6.30: Daily background tick fires with stale database → refresh initiated
func TestGeoIPLifecycle_BackgroundRefresh_RT6_30(t *testing.T) {
	dir := t.TempDir()
	createMinimalMMDB(t, dir)

	downloaded := make(chan bool, 1)
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloaded <- true
		w.Header().Set("Content-Type", "application/gzip")
		gz := gzip.NewWriter(w)
		_, _ = gz.Write([]byte("background-refresh-data"))
		_ = gz.Close()
	}))
	defer mockServer.Close()

	// Use a very short check interval and a Now function that makes the
	// file appear stale after the first tick.
	callCount := 0
	svc := geoip.New(geoip.Config{
		Dir:           dir,
		DownloadURL:   mockServer.URL + "/test.mmdb.gz",
		MaxAge:        1 * time.Second,
		CheckInterval: 100 * time.Millisecond,
		HTTPClient:    mockServer.Client(),
		Now: func() time.Time {
			callCount++
			if callCount <= 1 {
				return time.Now() // fresh for startup
			}
			// After startup, report time far in the future so the file appears stale.
			return time.Now().Add(48 * time.Hour)
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = svc.Start(ctx)
	defer func() { _ = svc.Close() }()

	select {
	case <-downloaded:
		// Background refresh triggered.
	case <-time.After(5 * time.Second):
		t.Error("background refresh did not trigger within 5 seconds")
	}
}
