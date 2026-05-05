// ABOUTME: AC6.1 tests — short links redirect to the correct destination,
// ABOUTME: invalid paths return 404. Tests RT-6.1 through RT-6.10.

package regression

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tadg-paul/golink/internal/router"
	"github.com/tadg-paul/golink/internal/server"
)

// RT-6.1: Amazon resolver redirects /az/B08N5WRWNW to amazon.com
func TestRedirect_AmazonASIN_RT6_1(t *testing.T) {
	ts := newTestServer(t, nil)
	defer ts.Close()
	client := noRedirectClient()

	resp, err := client.Get(ts.URL + "/az/B08N5WRWNW")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 302 {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc != "https://www.amazon.com/dp/B08N5WRWNW" {
		t.Errorf("Location = %q, want %q", loc, "https://www.amazon.com/dp/B08N5WRWNW")
	}
}

// RT-6.2: GitHub resolver redirects /gh/torvalds/linux to github.com
func TestRedirect_GitHub_RT6_2(t *testing.T) {
	ts := newTestServer(t, nil)
	defer ts.Close()
	client := noRedirectClient()

	resp, err := client.Get(ts.URL + "/gh/torvalds/linux")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 302 {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc != "https://github.com/torvalds/linux" {
		t.Errorf("Location = %q, want %q", loc, "https://github.com/torvalds/linux")
	}
}

// RT-6.3: Wikipedia resolver redirects /wiki/Linux to en.wikipedia.org
func TestRedirect_Wikipedia_RT6_3(t *testing.T) {
	ts := newTestServer(t, nil)
	defer ts.Close()
	client := noRedirectClient()

	resp, err := client.Get(ts.URL + "/wiki/Linux")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 302 {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc != "https://en.wikipedia.org/wiki/Linux" {
		t.Errorf("Location = %q, want %q", loc, "https://en.wikipedia.org/wiki/Linux")
	}
}

// RT-6.4: Redirect response has Location header, empty body, no Content-Type
func TestRedirect_ResponseShape_RT6_4(t *testing.T) {
	ts := newTestServer(t, nil)
	defer ts.Close()
	client := noRedirectClient()

	resp, err := client.Get(ts.URL + "/az/B08N5WRWNW")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 302 {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}
	if resp.Header.Get("Location") == "" {
		t.Error("missing Location header")
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) != 0 {
		t.Errorf("body = %q, want empty", string(body))
	}
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		t.Errorf("Content-Type = %q, want empty", ct)
	}
}

// RT-6.5: Root path / returns 404 with empty body
func TestRedirect_RootPath404_RT6_5(t *testing.T) {
	ts := newTestServer(t, nil)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) != 0 {
		t.Errorf("body = %q, want empty", string(body))
	}
}

// RT-6.6: Unrecognised prefix /unknown/foo returns 404
func TestRedirect_UnknownPrefix404_RT6_6(t *testing.T) {
	ts := newTestServer(t, nil)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/unknown/foo")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// RT-6.7: Prefix with no remaining path /az returns 404
func TestRedirect_PrefixNoPath404_RT6_7(t *testing.T) {
	ts := newTestServer(t, nil)
	defer ts.Close()
	client := noRedirectClient()

	resp, err := client.Get(ts.URL + "/az")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// RT-6.8: Too few segments /gh/torvalds returns 404
func TestRedirect_TooFewSegments404_RT6_8(t *testing.T) {
	ts := newTestServer(t, nil)
	defer ts.Close()
	client := noRedirectClient()

	resp, err := client.Get(ts.URL + "/gh/torvalds")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// RT-6.9: Too many segments /az/B08N5WRWNW/extra returns 404
func TestRedirect_TooManySegments404_RT6_9(t *testing.T) {
	ts := newTestServer(t, nil)
	defer ts.Close()
	client := noRedirectClient()

	resp, err := client.Get(ts.URL + "/az/B08N5WRWNW/extra")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 404 {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

// RT-6.10: End-to-end integration — full server, forwarded header for a known
// country, correct geo-routed 302 Location + correct log line
func TestRedirect_EndToEndIntegration_RT6_10(t *testing.T) {
	// Capture log output.
	var logBuf bytes.Buffer
	origWriter := server.LogWriter
	server.LogWriter = &logBuf
	defer func() { server.LogWriter = origWriter }()

	geo := &stubGeo{mapping: map[string]string{
		"203.0.113.1": "DE",
	}}
	rtr, err := router.LoadDir(filepath.Join(testdataDir(t), "resolvers"))
	if err != nil {
		t.Fatalf("load resolvers: %v", err)
	}
	srv := server.New(server.Config{}, rtr, geo)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := noRedirectClient()
	req, _ := http.NewRequest("GET", ts.URL+"/az/B08N5WRWNW", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.1")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check redirect.
	if resp.StatusCode != 302 {
		t.Errorf("status = %d, want 302", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc != "https://www.amazon.de/dp/B08N5WRWNW" {
		t.Errorf("Location = %q, want %q", loc, "https://www.amazon.de/dp/B08N5WRWNW")
	}

	// Check log line.
	lines := strings.Split(strings.TrimSpace(logBuf.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("no log lines emitted")
	}
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &entry); err != nil {
		t.Fatalf("log line is not valid JSON: %v", err)
	}
	if entry["remote_ip"] != "203.0.113.1" {
		t.Errorf("log remote_ip = %v, want 203.0.113.1", entry["remote_ip"])
	}
	if entry["country"] != "DE" {
		t.Errorf("log country = %v, want DE", entry["country"])
	}
	if entry["target"] != "https://www.amazon.de/dp/B08N5WRWNW" {
		t.Errorf("log target = %v, want https://www.amazon.de/dp/B08N5WRWNW", entry["target"])
	}

	_ = net.IP{} // keep net import used
}
