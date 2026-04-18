// ABOUTME: Tests for the goreport convenience script's dry-run output.
// ABOUTME: Covers AC8.10 (RT-8.29 to RT-8.32).

package regression

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// goreportScript returns the absolute path to scripts/goreport.
func goreportScript(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine script path")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "scripts", "goreport")
}

// runGoreport executes goreport with --dry-run and returns stdout + exit code.
func runGoreport(t *testing.T, args ...string) (string, int) {
	t.Helper()
	script := goreportScript(t)
	fullArgs := append([]string{"--dry-run"}, args...)
	cmd := exec.Command(script, fullArgs...)
	cmd.Env = append(os.Environ(), "PATH="+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("exec goreport: %v", err)
		}
	}
	return string(out), exitCode
}

// RT-8.29: goreport with stats subcommand constructs the correct SSH command.
//
// Real-user test: operator runs `goreport light-hugger stats top --last 7d`
// and the script SSHes to light-hugger and runs `sudo golink stats top --last 7d`.
func TestGoreport_stats_command_RT8_29(t *testing.T) {
	out, code := runGoreport(t, "light-hugger", "stats", "top", "--last", "7d")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, out)
	}
	// Dry-run should print the SSH command it would execute.
	if !strings.Contains(out, "ssh") {
		t.Errorf("expected SSH command in output, got: %s", out)
	}
	if !strings.Contains(out, "light-hugger") {
		t.Errorf("expected host 'light-hugger' in command, got: %s", out)
	}
	if !strings.Contains(out, "golink stats top") {
		t.Errorf("expected 'golink stats top' in command, got: %s", out)
	}
	if !strings.Contains(out, "--last") || !strings.Contains(out, "7d") {
		t.Errorf("expected '--last 7d' in command, got: %s", out)
	}
}

// RT-8.30: goreport with logs subcommand constructs an SSH command that tails journalctl.
//
// Real-user test: operator runs `goreport light-hugger logs` and sees a live
// log tail from the server.
func TestGoreport_logs_command_RT8_30(t *testing.T) {
	out, code := runGoreport(t, "light-hugger", "logs")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, out)
	}
	if !strings.Contains(out, "journalctl") {
		t.Errorf("expected 'journalctl' in command, got: %s", out)
	}
	if !strings.Contains(out, "golink") {
		t.Errorf("expected 'golink' service name in command, got: %s", out)
	}
	if !strings.Contains(out, "-f") {
		t.Errorf("expected '-f' (follow) in command, got: %s", out)
	}
}

// RT-8.31: goreport with status subcommand constructs an SSH command that runs
// systemctl status and recent journal.
//
// Real-user test: operator runs `goreport light-hugger status` and sees
// systemd status plus recent log lines.
func TestGoreport_status_command_RT8_31(t *testing.T) {
	out, code := runGoreport(t, "light-hugger", "status")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d: %s", code, out)
	}
	if !strings.Contains(out, "systemctl") {
		t.Errorf("expected 'systemctl' in command, got: %s", out)
	}
	if !strings.Contains(out, "journalctl") {
		t.Errorf("expected 'journalctl' in command, got: %s", out)
	}
}

// RT-8.32: goreport with no arguments or missing host prints usage and exits non-zero.
//
// Real-user test: operator runs `goreport` with no arguments and gets a
// usage message, not a cryptic error.
func TestGoreport_no_args_usage_RT8_32(t *testing.T) {
	out, code := runGoreport(t)
	if code == 0 {
		t.Errorf("expected non-zero exit for no args, got 0")
	}
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "usage") {
		t.Errorf("expected 'usage' in output, got: %s", out)
	}
}
