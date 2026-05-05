// ABOUTME: AC6.2 tests — redirect behaviour is data-driven via YAML files.
// ABOUTME: Tests RT-6.11 through RT-6.17.

package regression

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tadg-paul/golink/internal/router"
)

// RT-6.11: Multiple YAML files all register as routable prefixes
func TestResolver_MultipleFilesLoaded_RT6_11(t *testing.T) {
	rtr, err := router.LoadDir(filepath.Join(testdataDir(t), "resolvers"))
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	prefixes := rtr.Prefixes()
	if len(prefixes) < 3 {
		t.Errorf("loaded %d prefixes, want at least 3", len(prefixes))
	}
}

// RT-6.12: URL prefix matches the YAML filename stem
func TestResolver_FilenameStemIsPrefix_RT6_12(t *testing.T) {
	rtr, err := router.LoadDir(filepath.Join(testdataDir(t), "resolvers"))
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	for _, name := range []string{"az", "gh", "wiki"} {
		if rtr.Lookup(name) == nil {
			t.Errorf("prefix %q not registered", name)
		}
	}
}

// RT-6.13: Non-YAML files in the directory are ignored
func TestResolver_NonYAMLIgnored_RT6_13(t *testing.T) {
	dir := t.TempDir()
	// Write a valid YAML and a non-YAML file.
	writeFile(t, filepath.Join(dir, "test.yaml"), `
path: "{id}"
default: "https://example.com/{id}"
`)
	writeFile(t, filepath.Join(dir, "README.md"), "# not a resolver")
	writeFile(t, filepath.Join(dir, "notes.txt"), "some notes")

	rtr, err := router.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(rtr.Prefixes()) != 1 {
		t.Errorf("loaded %d prefixes, want 1", len(rtr.Prefixes()))
	}
}

// RT-6.14: Path variables are extracted and substituted into the redirect URL
func TestResolver_VariableSubstitution_RT6_14(t *testing.T) {
	ts := newTestServer(t, nil)
	defer ts.Close()
	client := noRedirectClient()

	resp, err := client.Get(ts.URL + "/gh/golang/go")
	if err != nil {
		t.Fatalf("GET failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	loc := resp.Header.Get("Location")
	want := "https://github.com/golang/go"
	if loc != want {
		t.Errorf("Location = %q, want %q", loc, want)
	}
}

// RT-6.15: All YAMLs invalid at startup → refuses to start
func TestResolver_AllInvalidRefusesStart_RT6_15(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "bad.yaml"), "not: valid: yaml: {{{}}")

	_, err := router.LoadDir(dir)
	if err == nil {
		t.Error("LoadDir succeeded with all-invalid YAMLs, want error")
	}
}

// RT-6.16: Empty resolver directory → refuses to start (zero resolvers)
func TestResolver_EmptyDirRefusesStart_RT6_16(t *testing.T) {
	dir := t.TempDir()

	rtr, err := router.LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v (unexpected error for empty dir)", err)
	}
	// The router loads fine but has zero prefixes. The main function
	// checks this and exits — we verify the count here.
	if len(rtr.Prefixes()) != 0 {
		t.Errorf("loaded %d prefixes from empty dir, want 0", len(rtr.Prefixes()))
	}
}

// RT-6.17: Missing resolver directory → refuses to start
func TestResolver_MissingDirRefusesStart_RT6_17(t *testing.T) {
	_, err := router.LoadDir("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("LoadDir succeeded with missing dir, want error")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
