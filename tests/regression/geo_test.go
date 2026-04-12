// ABOUTME: AC6.3 tests — location-aware routing based on client IP geolocation.
// ABOUTME: Tests RT-6.18 through RT-6.22.

package regression

import (
	"net/http"
	"testing"
)

// RT-6.18: German IP on Amazon resolver → amazon.de
func TestGeo_GermanAmazon_RT6_18(t *testing.T) {
	geo := &stubGeo{mapping: map[string]string{"203.0.113.1": "DE"}}
	ts := newTestServer(t, geo)
	defer ts.Close()
	client := noRedirectClient()

	req, _ := http.NewRequest("GET", ts.URL+"/az/B08N5WRWNW", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.1")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loc := resp.Header.Get("Location")
	if loc != "https://www.amazon.de/dp/B08N5WRWNW" {
		t.Errorf("Location = %q, want amazon.de", loc)
	}
}

// RT-6.19: Unmapped country on Amazon resolver → amazon.com (default)
func TestGeo_UnmappedCountryDefault_RT6_19(t *testing.T) {
	geo := &stubGeo{mapping: map[string]string{"203.0.113.2": "ZZ"}}
	ts := newTestServer(t, geo)
	defer ts.Close()
	client := noRedirectClient()

	req, _ := http.NewRequest("GET", ts.URL+"/az/B08N5WRWNW", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.2")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loc := resp.Header.Get("Location")
	if loc != "https://www.amazon.com/dp/B08N5WRWNW" {
		t.Errorf("Location = %q, want amazon.com (default)", loc)
	}
}

// RT-6.20: French IP on Wikipedia resolver → fr.wikipedia.org
func TestGeo_FrenchWikipedia_RT6_20(t *testing.T) {
	geo := &stubGeo{mapping: map[string]string{"203.0.113.3": "FR"}}
	ts := newTestServer(t, geo)
	defer ts.Close()
	client := noRedirectClient()

	req, _ := http.NewRequest("GET", ts.URL+"/wiki/Linux", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.3")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loc := resp.Header.Get("Location")
	if loc != "https://fr.wikipedia.org/wiki/Linux" {
		t.Errorf("Location = %q, want fr.wikipedia.org", loc)
	}
}

// RT-6.21: GitHub resolver (no geo block) uses default regardless of country
func TestGeo_NoGeoBlockIgnoresCountry_RT6_21(t *testing.T) {
	geo := &stubGeo{mapping: map[string]string{"203.0.113.4": "DE"}}
	ts := newTestServer(t, geo)
	defer ts.Close()
	client := noRedirectClient()

	req, _ := http.NewRequest("GET", ts.URL+"/gh/torvalds/linux", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.4")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loc := resp.Header.Get("Location")
	if loc != "https://github.com/torvalds/linux" {
		t.Errorf("Location = %q, want github.com (no geo)", loc)
	}
}

// RT-6.22: Unknown IP (empty country) → default template
func TestGeo_UnknownIPDefault_RT6_22(t *testing.T) {
	geo := &stubGeo{mapping: map[string]string{}} // no mapping for any IP
	ts := newTestServer(t, geo)
	defer ts.Close()
	client := noRedirectClient()

	resp, err := client.Get(ts.URL + "/az/B08N5WRWNW")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loc := resp.Header.Get("Location")
	if loc != "https://www.amazon.com/dp/B08N5WRWNW" {
		t.Errorf("Location = %q, want amazon.com (default)", loc)
	}
}
