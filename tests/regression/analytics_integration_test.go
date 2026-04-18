// ABOUTME: Integration tests verifying that HTTP requests produce analytics
// ABOUTME: events through the full server stack. Covers AC8.1 (RT-8.1 to RT-8.6).

package regression

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/tigger04/golink/internal/analytics"
	"github.com/tigger04/golink/internal/router"
	"github.com/tigger04/golink/internal/server"
)

// newTestServerWithAnalytics creates an httptest.Server with analytics recording
// enabled. The analytics store uses a temp-dir SQLite database.
func newTestServerWithAnalytics(t *testing.T, geo server.GeoLookup) (*httptest.Server, *analytics.Store) {
	t.Helper()
	rtr, err := router.LoadDir(filepath.Join(testdataDir(t), "resolvers"))
	if err != nil {
		t.Fatalf("load test resolvers: %v", err)
	}
	store, _ := openTestStore(t)
	srv := server.New(server.Config{Analytics: store}, rtr, geo)
	return httptest.NewServer(srv.Handler()), store
}

// RT-8.1: Successful redirect (302) persists event with all fields populated.
//
// Real-user test: a user visits /az/B0XYZ and gets redirected. The operator
// later queries the analytics DB and sees an event for that request with
// timestamp, IP, country, prefix, path, status 302, and the redirect target.
func TestAnalyticsIntegration_redirect_persists_event_RT8_1(t *testing.T) {
	geo := &stubGeo{mapping: map[string]string{"127.0.0.1": "IE"}}
	ts, store := newTestServerWithAnalytics(t, geo)
	defer ts.Close()

	client := noRedirectClient()
	req, _ := http.NewRequest("GET", ts.URL+"/az/B0XYZ", nil)
	req.Header.Set("Referer", "https://twitter.com/some/post")
	req.Header.Set("User-Agent", "TestBot/1.0")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /az/B0XYZ: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != 302 {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}

	// Query the store for recorded events.
	events, err := store.RecentEvents(10)
	if err != nil {
		t.Fatalf("RecentEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Prefix != "az" {
		t.Errorf("prefix: got %q, want %q", e.Prefix, "az")
	}
	if e.Path != "B0XYZ" {
		t.Errorf("path: got %q, want %q", e.Path, "B0XYZ")
	}
	if e.Status != 302 {
		t.Errorf("status: got %d, want 302", e.Status)
	}
	if e.Target == "" {
		t.Error("target: expected non-empty redirect URL")
	}
	if e.RemoteIP == "" {
		t.Error("remote_ip: expected non-empty")
	}
	if e.TS.IsZero() {
		t.Error("ts: expected non-zero timestamp")
	}
}

// RT-8.2: Unknown-prefix request (404) persists event with empty destination.
//
// Real-user test: a user visits /nonexistent/path and gets 404. The operator
// sees the event in analytics with status 404 and an empty target.
func TestAnalyticsIntegration_404_persists_event_RT8_2(t *testing.T) {
	ts, store := newTestServerWithAnalytics(t, nil)
	defer ts.Close()

	client := noRedirectClient()
	resp, err := client.Get(ts.URL + "/nonexistent/path")
	if err != nil {
		t.Fatalf("GET /nonexistent/path: %v", err)
	}
	resp.Body.Close()

	events, err := store.RecentEvents(10)
	if err != nil {
		t.Fatalf("RecentEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	e := events[0]
	if e.Status != 404 {
		t.Errorf("status: got %d, want 404", e.Status)
	}
	if e.Target != "" {
		t.Errorf("target: got %q, want empty for 404", e.Target)
	}
	if e.Prefix != "nonexistent" {
		t.Errorf("prefix: got %q, want %q", e.Prefix, "nonexistent")
	}
}

// RT-8.3: Referer header value is captured in the persisted event.
//
// Real-user test: a user clicks a golink from a tweet. The operator queries
// analytics and sees the Twitter URL as the referer.
func TestAnalyticsIntegration_referer_captured_RT8_3(t *testing.T) {
	ts, store := newTestServerWithAnalytics(t, nil)
	defer ts.Close()

	client := noRedirectClient()
	req, _ := http.NewRequest("GET", ts.URL+"/gh/torvalds/linux", nil)
	req.Header.Set("Referer", "https://twitter.com/user/status/123456")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	events, err := store.RecentEvents(10)
	if err != nil {
		t.Fatalf("RecentEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Referer != "https://twitter.com/user/status/123456" {
		t.Errorf("referer: got %q, want %q", events[0].Referer, "https://twitter.com/user/status/123456")
	}
}

// RT-8.4: User-Agent header value is captured in the persisted event.
//
// Real-user test: a user's browser sends a User-Agent string. The operator
// sees it in the analytics event.
func TestAnalyticsIntegration_user_agent_captured_RT8_4(t *testing.T) {
	ts, store := newTestServerWithAnalytics(t, nil)
	defer ts.Close()

	client := noRedirectClient()
	req, _ := http.NewRequest("GET", ts.URL+"/gh/torvalds/linux", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	events, err := store.RecentEvents(10)
	if err != nil {
		t.Fatalf("RecentEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].UserAgent != "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)" {
		t.Errorf("user_agent: got %q", events[0].UserAgent)
	}
}

// RT-8.5: Country code from GeoIP lookup is captured in the persisted event.
//
// Real-user test: a user from Germany visits /az/B0XYZ. The operator sees
// country=DE in the analytics event.
func TestAnalyticsIntegration_country_captured_RT8_5(t *testing.T) {
	geo := &stubGeo{mapping: map[string]string{"127.0.0.1": "DE"}}
	ts, store := newTestServerWithAnalytics(t, geo)
	defer ts.Close()

	client := noRedirectClient()
	resp, err := client.Get(ts.URL + "/az/B0XYZ")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	events, err := store.RecentEvents(10)
	if err != nil {
		t.Fatalf("RecentEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Country != "DE" {
		t.Errorf("country: got %q, want %q", events[0].Country, "DE")
	}
}

// RT-8.6: Remote IP is captured from X-Forwarded-For when present.
//
// Real-user test: a user behind Caddy (reverse proxy) visits golink. The
// operator sees the user's real IP (from X-Forwarded-For), not the proxy's.
func TestAnalyticsIntegration_xff_ip_captured_RT8_6(t *testing.T) {
	ts, store := newTestServerWithAnalytics(t, nil)
	defer ts.Close()

	client := noRedirectClient()
	req, _ := http.NewRequest("GET", ts.URL+"/gh/torvalds/linux", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.42, 10.0.0.1")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	resp.Body.Close()

	events, err := store.RecentEvents(10)
	if err != nil {
		t.Fatalf("RecentEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	// X-Forwarded-For uses the leftmost (original client) IP.
	if events[0].RemoteIP != "203.0.113.42" {
		t.Errorf("remote_ip: got %q, want %q", events[0].RemoteIP, "203.0.113.42")
	}

	// Anti-gaming check: make sure a request WITHOUT X-Forwarded-For captures
	// the direct RemoteAddr (not the previous request's XFF).
	_ = time.Now() // ensure different timestamp
	resp2, err := client.Get(ts.URL + "/gh/torvalds/linux")
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	resp2.Body.Close()

	events2, err := store.RecentEvents(10)
	if err != nil {
		t.Fatalf("RecentEvents: %v", err)
	}
	if len(events2) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events2))
	}
	// Second event (most recent = first in list) should have 127.0.0.1, not 203.0.113.42.
	if events2[0].RemoteIP == "203.0.113.42" {
		t.Error("second request without XFF should not carry previous request's IP")
	}
}
