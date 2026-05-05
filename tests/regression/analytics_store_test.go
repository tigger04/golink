// ABOUTME: Tests for analytics store provisioning, persistence, and WAL mode.
// ABOUTME: Covers AC8.2 (RT-8.7 to RT-8.9).

package regression

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tadg-paul/golink/internal/analytics"
)

// openTestStore creates a temporary analytics store for testing.
// Returns the store and the path to the DB file.
func openTestStore(t *testing.T) (*analytics.Store, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "analytics.db")
	store, err := analytics.Open(dbPath)
	if err != nil {
		t.Fatalf("analytics.Open(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { store.Close() })
	return store, dbPath
}

// seedEvents inserts a batch of events into the store for testing.
func seedEvents(t *testing.T, store *analytics.Store, events []analytics.Event) {
	t.Helper()
	for _, e := range events {
		if err := store.Record(e); err != nil {
			t.Fatalf("Record: %v", err)
		}
	}
}

// RT-8.7: Fresh start with no existing database creates it automatically.
//
// Real-user test: an operator starts golink on a fresh server with no prior
// analytics data. The analytics DB file is created automatically without
// manual intervention.
func TestAnalyticsStore_fresh_start_creates_database_RT8_7(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "analytics.db")

	// Verify file does not exist before Open.
	if _, err := os.Stat(dbPath); !os.IsNotExist(err) {
		t.Fatalf("expected DB file not to exist before Open, got: %v", err)
	}

	store, err := analytics.Open(dbPath)
	if err != nil {
		t.Fatalf("analytics.Open: %v", err)
	}
	defer store.Close()

	// Verify file now exists.
	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("expected DB file to exist after Open, got: %v", err)
	}
	if info.Size() == 0 {
		t.Error("expected DB file to have non-zero size")
	}

	// Verify the table exists by recording an event.
	err = store.Record(analytics.Event{
		TS:       time.Now(),
		RemoteIP: "1.2.3.4",
		Status:   302,
	})
	if err != nil {
		t.Fatalf("Record after fresh Open: %v", err)
	}
}

// RT-8.8: Existing database with prior events is reused without data loss.
//
// Real-user test: an operator restarts golink. Previously recorded analytics
// events are still present in the database after restart.
func TestAnalyticsStore_existing_database_reused_RT8_8(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "analytics.db")

	// First open: create DB and insert an event.
	store1, err := analytics.Open(dbPath)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	err = store1.Record(analytics.Event{
		TS:       time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		RemoteIP: "10.0.0.1",
		Prefix:   "az",
		Path:     "B0XYZ",
		Status:   302,
		Target:   "https://www.amazon.com/dp/B0XYZ",
	})
	if err != nil {
		t.Fatalf("Record on first open: %v", err)
	}
	store1.Close()

	// Second open: verify the event is still there.
	store2, err := analytics.Open(dbPath)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer store2.Close()

	events, err := store2.RecentEvents(10)
	if err != nil {
		t.Fatalf("RecentEvents after reopen: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event after reopen, got %d", len(events))
	}
	if events[0].Prefix != "az" || events[0].Path != "B0XYZ" {
		t.Errorf("event data mismatch after reopen: prefix=%q path=%q", events[0].Prefix, events[0].Path)
	}
}

// RT-8.9: The database journal mode is WAL.
//
// Real-user test: the operator inspects the analytics database and confirms
// it uses WAL journal mode for concurrent read/write safety.
func TestAnalyticsStore_wal_mode_RT8_9(t *testing.T) {
	store, dbPath := openTestStore(t)

	// Open a second read-only connection to verify WAL mode via PRAGMA.
	roStore, err := analytics.OpenReadOnly(dbPath)
	if err != nil {
		t.Fatalf("OpenReadOnly: %v", err)
	}
	defer roStore.Close()

	// The Store doesn't expose the raw DB, so we verify WAL by checking
	// that a WAL file exists on disk (SQLite creates <db>-wal in WAL mode).
	// We need at least one write to trigger WAL file creation.
	err = store.Record(analytics.Event{
		TS:       time.Now(),
		RemoteIP: "1.2.3.4",
		Status:   200,
	})
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	walPath := dbPath + "-wal"
	if _, err := os.Stat(walPath); os.IsNotExist(err) {
		t.Error("expected WAL file to exist, indicating WAL journal mode")
	}
}
