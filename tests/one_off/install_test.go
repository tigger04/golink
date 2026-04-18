// ABOUTME: One-off tests for make install/uninstall targets.
// ABOUTME: Covers AC8.11 (OT-8.1 to OT-8.2).
//
// These tests invoke Makefile targets and are therefore OTs, not RTs.
// Run via: make test-one-off ISSUE=8

package one_off

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func projectRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine project root")
	}
	// tests/one_off/install_test.go -> project root is two dirs up.
	return filepath.Join(filepath.Dir(file), "..", "..")
}

// OT-8.1: make install creates symlinks at ~/.local/bin/golink and ~/.local/bin/goreport.
//
// Real-user test: operator runs `make install` and can then run `golink` and
// `goreport` from any directory.
func TestMakeInstall_creates_symlinks_OT8_1(t *testing.T) {
	root := projectRoot(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	golinkLink := filepath.Join(home, ".local", "bin", "golink")
	goreportLink := filepath.Join(home, ".local", "bin", "goreport")

	cmd := exec.Command("make", "install")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make install failed: %v\n%s", err, out)
	}

	// Verify golink symlink.
	if info, err := os.Lstat(golinkLink); err != nil {
		t.Errorf("golink symlink not found: %v", err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("golink at %s is not a symlink", golinkLink)
	}

	// Verify goreport symlink.
	if info, err := os.Lstat(goreportLink); err != nil {
		t.Errorf("goreport symlink not found: %v", err)
	} else if info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("goreport at %s is not a symlink", goreportLink)
	}
}

// OT-8.2: make uninstall removes both symlinks.
//
// Real-user test: operator runs `make uninstall` and the golink/goreport
// commands are no longer available.
func TestMakeUninstall_removes_symlinks_OT8_2(t *testing.T) {
	root := projectRoot(t)
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}

	golinkLink := filepath.Join(home, ".local", "bin", "golink")
	goreportLink := filepath.Join(home, ".local", "bin", "goreport")

	// First install, then uninstall.
	install := exec.Command("make", "install")
	install.Dir = root
	if out, err := install.CombinedOutput(); err != nil {
		t.Fatalf("make install (setup): %v\n%s", err, out)
	}

	uninstall := exec.Command("make", "uninstall")
	uninstall.Dir = root
	if out, err := uninstall.CombinedOutput(); err != nil {
		t.Fatalf("make uninstall: %v\n%s", err, out)
	}

	// Verify both symlinks are gone.
	if _, err := os.Lstat(golinkLink); !os.IsNotExist(err) {
		t.Errorf("golink symlink still exists after uninstall")
	}
	if _, err := os.Lstat(goreportLink); !os.IsNotExist(err) {
		t.Errorf("goreport symlink still exists after uninstall")
	}
}
