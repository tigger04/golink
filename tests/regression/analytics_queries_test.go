// ABOUTME: Tests for analytics query methods and CSV output formatting.
// ABOUTME: Covers AC8.3–AC8.9 (RT-8.10 to RT-8.28).

package regression

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
	"time"

	"github.com/tadg-paul/golink/internal/analytics"
)

// testEvents returns a standard set of events for query tests.
func testEvents() []analytics.Event {
	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	return []analytics.Event{
		{TS: base.Add(1 * time.Hour), RemoteIP: "1.1.1.1", Country: "US", Prefix: "az", Path: "B001", Status: 302, Target: "https://amazon.com/dp/B001", Referer: "https://twitter.com/post/1", UserAgent: "Chrome/1"},
		{TS: base.Add(2 * time.Hour), RemoteIP: "2.2.2.2", Country: "DE", Prefix: "az", Path: "B002", Status: 302, Target: "https://amazon.de/dp/B002", Referer: "https://reddit.com/r/test", UserAgent: "Firefox/1"},
		{TS: base.Add(3 * time.Hour), RemoteIP: "1.1.1.1", Country: "US", Prefix: "az", Path: "B001", Status: 302, Target: "https://amazon.com/dp/B001", Referer: "https://twitter.com/post/2", UserAgent: "Chrome/1"},
		{TS: base.Add(4 * time.Hour), RemoteIP: "3.3.3.3", Country: "FR", Prefix: "gh", Path: "torvalds/linux", Status: 302, Target: "https://github.com/torvalds/linux", Referer: "", UserAgent: "Safari/1"},
		{TS: base.Add(5 * time.Hour), RemoteIP: "4.4.4.4", Country: "IE", Prefix: "wiki", Path: "Go", Status: 302, Target: "https://en.wikipedia.org/wiki/Go", Referer: "https://reddit.com/r/golang", UserAgent: "Chrome/2"},
		{TS: base.Add(6 * time.Hour), RemoteIP: "5.5.5.5", Country: "GB", Prefix: "nonexistent", Path: "foo", Status: 404, Target: "", Referer: "https://twitter.com/post/3", UserAgent: "Bot/1"},
		{TS: base.Add(7 * time.Hour), RemoteIP: "6.6.6.6", Country: "US", Prefix: "also404", Path: "bar", Status: 404, Target: "", Referer: "", UserAgent: "Bot/2"},
		{TS: base.Add(8 * time.Hour), RemoteIP: "1.1.1.1", Country: "US", Prefix: "nonexistent", Path: "foo", Status: 404, Target: "", Referer: "", UserAgent: "Chrome/1"},
	}
}

func seedTestEvents(t *testing.T, store *analytics.Store) {
	t.Helper()
	seedEvents(t, store, testEvents())
}

// --- AC8.3: TopLinks ---

// RT-8.10: Top-links query returns links ranked by descending click count.
//
// Real-user test: operator runs `golink stats top` and sees az at the top
// (3 clicks), then gh and wiki (1 each).
func TestAnalyticsQueries_top_links_ranked_RT8_10(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	links, err := store.TopLinks(time.Time{}, 10)
	if err != nil {
		t.Fatalf("TopLinks: %v", err)
	}
	if len(links) == 0 {
		t.Fatal("expected at least one link")
	}
	// "az" has 3 clicks (most), should be first.
	if links[0].Prefix != "az" {
		t.Errorf("top link: got prefix %q, want %q", links[0].Prefix, "az")
	}
	if links[0].Count != 3 {
		t.Errorf("top link count: got %d, want 3", links[0].Count)
	}

	// Verify descending order.
	for i := 1; i < len(links); i++ {
		if links[i].Count > links[i-1].Count {
			t.Errorf("links not in descending order: [%d].Count=%d > [%d].Count=%d",
				i, links[i].Count, i-1, links[i-1].Count)
		}
	}
}

// RT-8.11: Top-links query with time-range filter excludes events outside the range.
//
// Real-user test: operator runs `golink stats top --last 4h` and only sees
// events from the last 4 hours.
func TestAnalyticsQueries_top_links_time_filter_RT8_11(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	// Filter to only the last 3 events (hours 6, 7, 8 of the test data).
	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	since := base.Add(5*time.Hour + 30*time.Minute)

	links, err := store.TopLinks(since, 10)
	if err != nil {
		t.Fatalf("TopLinks with filter: %v", err)
	}
	// Only events after 5:30 are: hour 6 (nonexistent/404), hour 7 (also404/404),
	// hour 8 (nonexistent/404). These are all 404s.
	// The top-links report should include all statuses (redirects + 404s).
	var total int64
	for _, l := range links {
		total += l.Count
	}
	if total != 3 {
		t.Errorf("filtered total: got %d, want 3", total)
	}
}

// RT-8.12: Top-links query with no events returns an empty result.
//
// Real-user test: operator runs `golink stats top` on a freshly deployed
// golink with no traffic. Sees an empty report, not an error.
func TestAnalyticsQueries_top_links_empty_RT8_12(t *testing.T) {
	store, _ := openTestStore(t)

	links, err := store.TopLinks(time.Time{}, 10)
	if err != nil {
		t.Fatalf("TopLinks on empty DB: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("expected empty result, got %d links", len(links))
	}
}

// --- AC8.4: RecentEvents ---

// RT-8.13: Recent-events query returns events in reverse chronological order.
//
// Real-user test: operator runs `golink stats recent` and sees the most
// recent event first.
func TestAnalyticsQueries_recent_events_order_RT8_13(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	events, err := store.RecentEvents(10)
	if err != nil {
		t.Fatalf("RecentEvents: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	// Verify reverse chronological order.
	for i := 1; i < len(events); i++ {
		if events[i].TS.After(events[i-1].TS) {
			t.Errorf("events not in reverse chronological order: [%d]=%v after [%d]=%v",
				i, events[i].TS, i-1, events[i-1].TS)
		}
	}

	// Most recent event should be the last one seeded (hour 8).
	if events[0].Prefix != "nonexistent" || events[0].RemoteIP != "1.1.1.1" {
		t.Errorf("most recent event: prefix=%q ip=%q, expected nonexistent/1.1.1.1",
			events[0].Prefix, events[0].RemoteIP)
	}
}

// RT-8.14: Recent-events query respects a configurable limit on the number of rows.
//
// Real-user test: operator runs `golink stats recent --limit 3` and sees
// exactly 3 events.
func TestAnalyticsQueries_recent_events_limit_RT8_14(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	events, err := store.RecentEvents(3)
	if err != nil {
		t.Fatalf("RecentEvents: %v", err)
	}
	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d", len(events))
	}
}

// --- AC8.5: LinkDetail ---

// RT-8.15: Per-link query returns total click count for the specified prefix.
//
// Real-user test: operator runs `golink stats link az` and sees "3 clicks".
func TestAnalyticsQueries_link_detail_count_RT8_15(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	total, _, err := store.LinkDetail("az", time.Time{})
	if err != nil {
		t.Fatalf("LinkDetail: %v", err)
	}
	if total != 3 {
		t.Errorf("total clicks for 'az': got %d, want 3", total)
	}
}

// RT-8.16: Per-link query includes country-level breakdown with counts.
//
// Real-user test: operator runs `golink stats link az` and sees
// US: 2, DE: 1.
func TestAnalyticsQueries_link_detail_countries_RT8_16(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	_, countries, err := store.LinkDetail("az", time.Time{})
	if err != nil {
		t.Fatalf("LinkDetail: %v", err)
	}
	if len(countries) == 0 {
		t.Fatal("expected country breakdown")
	}

	// Build a map for easy lookup.
	countryMap := make(map[string]int64)
	for _, c := range countries {
		countryMap[c.Country] = c.Count
	}

	if countryMap["US"] != 2 {
		t.Errorf("US clicks: got %d, want 2", countryMap["US"])
	}
	if countryMap["DE"] != 1 {
		t.Errorf("DE clicks: got %d, want 1", countryMap["DE"])
	}
}

// RT-8.17: Per-link query with time-range filter excludes events outside the range.
//
// Real-user test: operator runs `golink stats link az --last 2h` and sees
// only the click from hour 3 (1 click), not the earlier ones.
func TestAnalyticsQueries_link_detail_time_filter_RT8_17(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	since := base.Add(2*time.Hour + 30*time.Minute)

	total, _, err := store.LinkDetail("az", since)
	if err != nil {
		t.Fatalf("LinkDetail with filter: %v", err)
	}
	// Only the event at hour 3 qualifies (hour 1 and 2 are before the cutoff).
	if total != 1 {
		t.Errorf("filtered total for 'az': got %d, want 1", total)
	}
}

// --- AC8.6: TopReferers ---

// RT-8.18: Top-referers query returns referer domains ranked by descending click count.
//
// Real-user test: operator runs `golink stats referers` and sees
// twitter.com at the top.
func TestAnalyticsQueries_top_referers_ranked_RT8_18(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	referers, err := store.TopReferers(time.Time{}, 10)
	if err != nil {
		t.Fatalf("TopReferers: %v", err)
	}
	if len(referers) == 0 {
		t.Fatal("expected at least one referer")
	}
	// twitter.com has 3 referrals (posts 1, 2, 3), reddit.com has 2.
	if referers[0].Domain != "twitter.com" {
		t.Errorf("top referer: got %q, want %q", referers[0].Domain, "twitter.com")
	}
	if referers[0].Count != 3 {
		t.Errorf("top referer count: got %d, want 3", referers[0].Count)
	}
}

// RT-8.19: Top-referers query with time-range filter excludes events outside the range.
//
// Real-user test: operator runs `golink stats referers --last 3h` and sees
// only recent referers.
func TestAnalyticsQueries_top_referers_time_filter_RT8_19(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	since := base.Add(5*time.Hour + 30*time.Minute)

	referers, err := store.TopReferers(since, 10)
	if err != nil {
		t.Fatalf("TopReferers with filter: %v", err)
	}
	// Only event at hour 6 (twitter.com/post/3) has a referer in this window.
	var total int64
	for _, r := range referers {
		total += r.Count
	}
	if total != 1 {
		t.Errorf("filtered referer total: got %d, want 1", total)
	}
}

// RT-8.20: Referers with different paths but the same domain are grouped together.
//
// Real-user test: operator sees "twitter.com: 3" not three separate entries
// for /post/1, /post/2, /post/3.
func TestAnalyticsQueries_referers_grouped_by_domain_RT8_20(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	referers, err := store.TopReferers(time.Time{}, 10)
	if err != nil {
		t.Fatalf("TopReferers: %v", err)
	}

	// Count how many entries have twitter.com as domain.
	twitterCount := 0
	for _, r := range referers {
		if r.Domain == "twitter.com" {
			twitterCount++
		}
	}
	if twitterCount != 1 {
		t.Errorf("expected exactly 1 twitter.com entry (grouped), got %d", twitterCount)
	}
}

// --- AC8.7: MissedLinks ---

// RT-8.21: Misses query returns 404 paths ranked by descending frequency.
//
// Real-user test: operator runs `golink stats misses` and sees the most
// frequently missed path first.
func TestAnalyticsQueries_misses_ranked_RT8_21(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	misses, err := store.MissedLinks(time.Time{}, 10)
	if err != nil {
		t.Fatalf("MissedLinks: %v", err)
	}
	if len(misses) == 0 {
		t.Fatal("expected at least one miss")
	}
	// "nonexistent" has 2 hits (hours 6 and 8), "also404" has 1.
	if misses[0].Prefix != "nonexistent" {
		t.Errorf("top miss: got prefix %q, want %q", misses[0].Prefix, "nonexistent")
	}
	if misses[0].Count != 2 {
		t.Errorf("top miss count: got %d, want 2", misses[0].Count)
	}
}

// RT-8.22: Misses query excludes successful redirects (non-404 status codes).
//
// Real-user test: operator runs `golink stats misses` and does NOT see
// /az or /gh in the output.
func TestAnalyticsQueries_misses_excludes_redirects_RT8_22(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	misses, err := store.MissedLinks(time.Time{}, 100)
	if err != nil {
		t.Fatalf("MissedLinks: %v", err)
	}
	for _, m := range misses {
		if m.Prefix == "az" || m.Prefix == "gh" || m.Prefix == "wiki" {
			t.Errorf("misses should not include successful prefix %q", m.Prefix)
		}
	}
}

// RT-8.23: Misses query with time-range filter excludes events outside the range.
//
// Real-user test: operator runs `golink stats misses --last 2h` and sees
// only the most recent 404.
func TestAnalyticsQueries_misses_time_filter_RT8_23(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	since := base.Add(7*time.Hour + 30*time.Minute)

	misses, err := store.MissedLinks(since, 10)
	if err != nil {
		t.Fatalf("MissedLinks with filter: %v", err)
	}
	// Only the event at hour 8 (nonexistent/foo, 404) qualifies.
	var total int64
	for _, m := range misses {
		total += m.Count
	}
	if total != 1 {
		t.Errorf("filtered misses total: got %d, want 1", total)
	}
}

// --- AC8.8: UniqueVisitors ---

// RT-8.24: Unique-visitors query returns distinct IP count and total click count.
//
// Real-user test: operator runs `golink stats unique` and sees
// 6 unique IPs, 8 total clicks.
func TestAnalyticsQueries_unique_visitors_overall_RT8_24(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	stats, err := store.UniqueVisitors(time.Time{}, false)
	if err != nil {
		t.Fatalf("UniqueVisitors: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 overall row, got %d", len(stats))
	}
	if stats[0].UniqueIPs != 6 {
		t.Errorf("unique IPs: got %d, want 6", stats[0].UniqueIPs)
	}
	if stats[0].TotalClicks != 8 {
		t.Errorf("total clicks: got %d, want 8", stats[0].TotalClicks)
	}
}

// RT-8.25: Unique-visitors query breaks down by link prefix when requested.
//
// Real-user test: operator runs `golink stats unique --by-prefix` and sees
// per-prefix breakdowns (e.g. az: 2 unique / 3 clicks).
func TestAnalyticsQueries_unique_visitors_by_prefix_RT8_25(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	stats, err := store.UniqueVisitors(time.Time{}, true)
	if err != nil {
		t.Fatalf("UniqueVisitors by prefix: %v", err)
	}
	if len(stats) < 2 {
		t.Fatalf("expected multiple prefix rows, got %d", len(stats))
	}

	// Build a map for easy lookup.
	m := make(map[string]analytics.UniqueStats)
	for _, s := range stats {
		m[s.Prefix] = s
	}

	az, ok := m["az"]
	if !ok {
		t.Fatal("expected 'az' prefix in results")
	}
	if az.UniqueIPs != 2 {
		t.Errorf("az unique IPs: got %d, want 2", az.UniqueIPs)
	}
	if az.TotalClicks != 3 {
		t.Errorf("az total clicks: got %d, want 3", az.TotalClicks)
	}
}

// RT-8.26: Unique-visitors query with time-range filter excludes events outside the range.
//
// Real-user test: operator runs `golink stats unique --last 3h` and sees
// only recent unique visitors.
func TestAnalyticsQueries_unique_visitors_time_filter_RT8_26(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	base := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	since := base.Add(5*time.Hour + 30*time.Minute)

	stats, err := store.UniqueVisitors(since, false)
	if err != nil {
		t.Fatalf("UniqueVisitors with filter: %v", err)
	}
	if len(stats) != 1 {
		t.Fatalf("expected 1 overall row, got %d", len(stats))
	}
	// Events after 5:30: hour 6 (5.5.5.5), hour 7 (6.6.6.6), hour 8 (1.1.1.1) = 3 unique, 3 clicks
	if stats[0].UniqueIPs != 3 {
		t.Errorf("filtered unique IPs: got %d, want 3", stats[0].UniqueIPs)
	}
	if stats[0].TotalClicks != 3 {
		t.Errorf("filtered total clicks: got %d, want 3", stats[0].TotalClicks)
	}
}

// --- AC8.9: CSV Output ---

// RT-8.27: CSV output produces valid CSV with a header row.
//
// Real-user test: operator runs `golink stats top --csv` and pipes to
// another tool. The output starts with a header row and is valid CSV.
func TestAnalyticsQueries_csv_has_header_RT8_27(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	links, err := store.TopLinks(time.Time{}, 10)
	if err != nil {
		t.Fatalf("TopLinks: %v", err)
	}

	var buf bytes.Buffer
	if err := analytics.WriteTopLinksCSV(&buf, links); err != nil {
		t.Fatalf("WriteTopLinksCSV: %v", err)
	}

	// Parse as CSV.
	reader := csv.NewReader(strings.NewReader(buf.String()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("CSV parse error: %v", err)
	}
	if len(records) == 0 {
		t.Fatal("expected at least a header row")
	}

	// First row is the header.
	header := records[0]
	if len(header) < 2 {
		t.Errorf("header has %d columns, expected at least 2", len(header))
	}
	// Header should contain recognisable column names.
	headerStr := strings.Join(header, ",")
	if !strings.Contains(strings.ToLower(headerStr), "prefix") {
		t.Errorf("header missing 'prefix' column: %v", header)
	}
	if !strings.Contains(strings.ToLower(headerStr), "count") {
		t.Errorf("header missing 'count' column: %v", header)
	}
}

// RT-8.28: CSV output contains the same data as the default tabular output.
//
// Real-user test: operator compares `golink stats top` and `golink stats top --csv`
// and sees the same data in both. The CSV data rows match the query results.
func TestAnalyticsQueries_csv_matches_data_RT8_28(t *testing.T) {
	store, _ := openTestStore(t)
	seedTestEvents(t, store)

	links, err := store.TopLinks(time.Time{}, 10)
	if err != nil {
		t.Fatalf("TopLinks: %v", err)
	}

	var buf bytes.Buffer
	if err := analytics.WriteTopLinksCSV(&buf, links); err != nil {
		t.Fatalf("WriteTopLinksCSV: %v", err)
	}

	reader := csv.NewReader(strings.NewReader(buf.String()))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("CSV parse error: %v", err)
	}

	// Subtract 1 for header row.
	dataRows := len(records) - 1
	if dataRows != len(links) {
		t.Errorf("CSV data rows: got %d, want %d", dataRows, len(links))
	}

	// Verify first data row matches the top link.
	if len(links) > 0 && dataRows > 0 {
		// The first data row should contain the top link's prefix.
		row := records[1]
		found := false
		for _, col := range row {
			if col == links[0].Prefix {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("first CSV data row %v does not contain top prefix %q", row, links[0].Prefix)
		}
	}
}
