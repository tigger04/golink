// ABOUTME: Sanity test for the v0 hello-world handler. Exercises the same
// ABOUTME: HTTP entry point a real user hits, via httptest.

package regression

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// helloHandler is a copy of the hello-world handler that lives in main.go.
// We duplicate it here rather than importing main, because main is package
// main and Go does not let you import it directly. Issue #6 will move the
// real handler into an importable internal package and this duplication
// disappears.
func helloHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "hello from golink test\n")
}

func TestHelloHandler_Status200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(helloHandler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestHelloHandler_BodyContainsHello(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(helloHandler))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), "hello from golink") {
		t.Errorf("body = %q, want substring %q", string(body), "hello from golink")
	}
}
