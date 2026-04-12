// ABOUTME: AC6.8 tests (RT portion) — README attribution and -version flag.
// ABOUTME: Tests RT-6.44 through RT-6.46.

package regression

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// projectRoot returns the repository root directory.
func projectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine project root")
	}
	// tests/regression/attribution_test.go → ../../
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// RT-6.44: README contains DB-IP attribution with link to db-ip.com
func TestAttribution_ReadmeDBIP_RT6_44(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join(projectRoot(t), "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	content := string(readme)
	if !strings.Contains(content, "db-ip.com") && !strings.Contains(content, "DB-IP.com") {
		t.Error("README.md does not contain DB-IP attribution link")
	}
	if !strings.Contains(content, "DB-IP") {
		t.Error("README.md does not mention DB-IP")
	}
}

// RT-6.45: README references CC BY 4.0 licence
func TestAttribution_ReadmeCCBY4_RT6_45(t *testing.T) {
	readme, err := os.ReadFile(filepath.Join(projectRoot(t), "README.md"))
	if err != nil {
		t.Fatalf("read README.md: %v", err)
	}
	content := string(readme)
	if !strings.Contains(content, "Creative Commons Attribution 4.0") &&
		!strings.Contains(content, "CC BY 4.0") &&
		!strings.Contains(content, "creativecommons.org/licenses/by/4.0") {
		t.Error("README.md does not reference CC BY 4.0 licence")
	}
}

// RT-6.46: -version prints version and exits with code 0
func TestAttribution_VersionFlag_RT6_46(t *testing.T) {
	binary := filepath.Join(projectRoot(t), "bin", "golink")
	if _, err := os.Stat(binary); os.IsNotExist(err) {
		t.Skip("binary not built; run 'make build' first")
	}

	cmd := execCommand(t, binary, "-version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("-version failed: %v", err)
	}
	version := strings.TrimSpace(string(out))
	if version == "" {
		t.Error("-version produced empty output")
	}
}
