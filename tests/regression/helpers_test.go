// ABOUTME: Shared test helpers for regression tests. Provides functions to
// ABOUTME: build test servers with configurable resolvers and GeoIP stubs.

package regression

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/tadg-paul/golink/internal/router"
	"github.com/tadg-paul/golink/internal/server"
)

// testdataDir returns the absolute path to tests/testdata/.
func testdataDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine testdata path")
	}
	return filepath.Join(filepath.Dir(file), "..", "testdata")
}

// stubGeo is a GeoIP stub that returns a fixed country code based on IP.
type stubGeo struct {
	mapping map[string]string // IP string → country code
}

func (s *stubGeo) CountryCode(ip net.IP) string {
	if s == nil || s.mapping == nil {
		return ""
	}
	return s.mapping[ip.String()]
}

// newTestServer creates an httptest.Server backed by the test resolvers
// and an optional GeoIP stub. Returns the server and a cleanup function.
func newTestServer(t *testing.T, geo server.GeoLookup) *httptest.Server {
	t.Helper()
	rtr, err := router.LoadDir(filepath.Join(testdataDir(t), "resolvers"))
	if err != nil {
		t.Fatalf("load test resolvers: %v", err)
	}
	srv := server.New(server.Config{}, rtr, geo)
	return httptest.NewServer(srv.Handler())
}

// noRedirectClient returns an HTTP client that does not follow redirects.
func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// execCommand creates an exec.Cmd for a binary with the given args.
func execCommand(t *testing.T, binary string, args ...string) *exec.Cmd {
	t.Helper()
	return exec.Command(binary, args...)
}
