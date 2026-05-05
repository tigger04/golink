// ABOUTME: AC6.4 tests — reverse proxy awareness. The service correctly
// ABOUTME: extracts the real client IP from forwarded headers. Tests RT-6.23
// ABOUTME: through RT-6.26.

package regression

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tadg-paul/golink/internal/router"
	"github.com/tadg-paul/golink/internal/server"
)

// RT-6.23: Request with X-Forwarded-For → forwarded IP used for geolocation
func TestProxy_ForwardedIPUsed_RT6_23(t *testing.T) {
	geo := &stubGeo{mapping: map[string]string{"198.51.100.1": "FR"}}
	ts := newTestServer(t, geo)
	defer ts.Close()
	client := noRedirectClient()

	req, _ := http.NewRequest("GET", ts.URL+"/az/B08N5WRWNW", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.1")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loc := resp.Header.Get("Location")
	if loc != "https://www.amazon.fr/dp/B08N5WRWNW" {
		t.Errorf("Location = %q, want amazon.fr (forwarded IP geo)", loc)
	}
}

// RT-6.24: Request without forwarded header → RemoteAddr used
func TestProxy_NoForwardedHeaderUsesRemote_RT6_24(t *testing.T) {
	// Without X-Forwarded-For, the test client's IP (127.0.0.1) is used.
	// 127.0.0.1 is not in the geo stub, so we get the default.
	geo := &stubGeo{mapping: map[string]string{"198.51.100.1": "DE"}}
	ts := newTestServer(t, geo)
	defer ts.Close()
	client := noRedirectClient()

	// No X-Forwarded-For header.
	resp, err := client.Get(ts.URL + "/az/B08N5WRWNW")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loc := resp.Header.Get("Location")
	if loc != "https://www.amazon.com/dp/B08N5WRWNW" {
		t.Errorf("Location = %q, want amazon.com (default, no forwarded header)", loc)
	}
}

// RT-6.25: Multi-hop X-Forwarded-For → leftmost (client) IP extracted
func TestProxy_MultiHopLeftmostIP_RT6_25(t *testing.T) {
	geo := &stubGeo{mapping: map[string]string{
		"198.51.100.1": "DE",
		"10.0.0.1":     "US",
	}}
	ts := newTestServer(t, geo)
	defer ts.Close()
	client := noRedirectClient()

	req, _ := http.NewRequest("GET", ts.URL+"/az/B08N5WRWNW", nil)
	// Client is 198.51.100.1 (DE), went through two proxies.
	req.Header.Set("X-Forwarded-For", "198.51.100.1, 10.0.0.1, 10.0.0.2")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loc := resp.Header.Get("Location")
	if loc != "https://www.amazon.de/dp/B08N5WRWNW" {
		t.Errorf("Location = %q, want amazon.de (leftmost IP)", loc)
	}
}

// RT-6.26: Log line remote_ip reflects real client IP, not proxy address
func TestProxy_LogReflectsRealIP_RT6_26(t *testing.T) {
	var logBuf bytes.Buffer
	origWriter := server.LogWriter
	server.LogWriter = &logBuf
	defer func() { server.LogWriter = origWriter }()

	geo := &stubGeo{mapping: map[string]string{"198.51.100.5": "DE"}}
	rtr, err := router.LoadDir(filepath.Join(testdataDir(t), "resolvers"))
	if err != nil {
		t.Fatalf("load resolvers: %v", err)
	}
	srv := server.New(server.Config{}, rtr, geo)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := noRedirectClient()
	req, _ := http.NewRequest("GET", ts.URL+"/az/B08N5WRWNW", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.5")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	_ = resp.Body.Close()

	lines := strings.Split(strings.TrimSpace(logBuf.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("no log lines")
	}
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("log not valid JSON: %v", err)
	}
	if entry["remote_ip"] != "198.51.100.5" {
		t.Errorf("log remote_ip = %v, want 198.51.100.5", entry["remote_ip"])
	}
}
