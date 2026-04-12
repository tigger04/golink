// ABOUTME: AC6.7 tests — structured JSON request logging to stderr.
// ABOUTME: Tests RT-6.41 through RT-6.43.

package regression

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tigger04/golink/internal/router"
	"github.com/tigger04/golink/internal/server"
)

// RT-6.41: Successful request emits valid JSON log line on stderr
func TestLogging_ValidJSON_RT6_41(t *testing.T) {
	var logBuf bytes.Buffer
	origWriter := server.LogWriter
	server.LogWriter = &logBuf
	defer func() { server.LogWriter = origWriter }()

	rtr, err := router.LoadDir(filepath.Join(testdataDir(t), "resolvers"))
	if err != nil {
		t.Fatalf("load resolvers: %v", err)
	}
	srv := server.New(server.Config{}, rtr, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := noRedirectClient()
	resp, err := client.Get(ts.URL + "/az/B08N5WRWNW")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	_ = resp.Body.Close()

	lines := strings.Split(strings.TrimSpace(logBuf.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("no log lines emitted")
	}
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("log line is not valid JSON: %v\nline: %s", err, lines[0])
	}
}

// RT-6.42: Log line contains all required fields with correct values
func TestLogging_AllFields_RT6_42(t *testing.T) {
	var logBuf bytes.Buffer
	origWriter := server.LogWriter
	server.LogWriter = &logBuf
	defer func() { server.LogWriter = origWriter }()

	rtr, err := router.LoadDir(filepath.Join(testdataDir(t), "resolvers"))
	if err != nil {
		t.Fatalf("load resolvers: %v", err)
	}
	srv := server.New(server.Config{}, rtr, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := noRedirectClient()
	resp, err := client.Get(ts.URL + "/az/B08N5WRWNW")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	_ = resp.Body.Close()

	lines := strings.Split(strings.TrimSpace(logBuf.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("no log lines")
	}
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("not JSON: %v", err)
	}

	required := []string{"ts", "remote_ip", "country", "prefix", "path", "status", "target", "latency_us"}
	for _, field := range required {
		if _, ok := entry[field]; !ok {
			t.Errorf("missing required field: %s", field)
		}
	}

	if entry["prefix"] != "az" {
		t.Errorf("prefix = %v, want az", entry["prefix"])
	}
	if entry["path"] != "B08N5WRWNW" {
		t.Errorf("path = %v, want B08N5WRWNW", entry["path"])
	}
	if status, ok := entry["status"].(float64); !ok || status != 302 {
		t.Errorf("status = %v, want 302", entry["status"])
	}
	if entry["target"] != "https://www.amazon.com/dp/B08N5WRWNW" {
		t.Errorf("target = %v, want amazon.com URL", entry["target"])
	}
}

// RT-6.43: 404 response emits structured log line with status=404 and target absent
func TestLogging_404Logged_RT6_43(t *testing.T) {
	var logBuf bytes.Buffer
	origWriter := server.LogWriter
	server.LogWriter = &logBuf
	defer func() { server.LogWriter = origWriter }()

	rtr, err := router.LoadDir(filepath.Join(testdataDir(t), "resolvers"))
	if err != nil {
		t.Fatalf("load resolvers: %v", err)
	}
	srv := server.New(server.Config{}, rtr, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/nonexistent/foo")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	_ = resp.Body.Close()

	lines := strings.Split(strings.TrimSpace(logBuf.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("no log lines")
	}
	var entry map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &entry); err != nil {
		t.Fatalf("not JSON: %v", err)
	}

	if status, ok := entry["status"].(float64); !ok || status != 404 {
		t.Errorf("status = %v, want 404", entry["status"])
	}
	if target, ok := entry["target"].(string); ok && target != "" {
		t.Errorf("target = %q, want empty for 404", target)
	}
}
