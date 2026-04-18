// ABOUTME: Analytics event store backed by SQLite. Records one row per HTTP
// ABOUTME: request and exposes query methods for the golink stats CLI.

package analytics

import (
	"database/sql"
	"fmt"
	"io"
	"time"
)

// Store wraps a SQLite database for analytics event storage and querying.
type Store struct {
	db *sql.DB
}

// Event represents a single analytics event (one HTTP request).
type Event struct {
	TS        time.Time
	RemoteIP  string
	Country   string
	Prefix    string
	Path      string
	Status    int
	Target    string
	Referer   string
	UserAgent string
}

// LinkCount is a row in the top-links or misses report.
type LinkCount struct {
	Prefix string
	Path   string
	Count  int64
}

// CountryCount is a row in the geographic breakdown.
type CountryCount struct {
	Country string
	Count   int64
}

// RefererCount is a row in the top-referers report.
type RefererCount struct {
	Domain string
	Count  int64
}

// UniqueStats is a row in the unique-visitors report.
type UniqueStats struct {
	Prefix      string
	UniqueIPs   int64
	TotalClicks int64
}

// Open opens or creates an analytics database at dbPath with WAL mode enabled.
func Open(dbPath string) (*Store, error) {
	return nil, fmt.Errorf("not implemented")
}

// OpenReadOnly opens an analytics database for read-only queries.
func OpenReadOnly(dbPath string) (*Store, error) {
	return nil, fmt.Errorf("not implemented")
}

// Record inserts a single analytics event.
func (s *Store) Record(e Event) error {
	return fmt.Errorf("not implemented")
}

// Close closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// TopLinks returns links ranked by click count, optionally filtered by time.
// A zero-value since means no time filter.
func (s *Store) TopLinks(since time.Time, limit int) ([]LinkCount, error) {
	return nil, fmt.Errorf("not implemented")
}

// RecentEvents returns the most recent events in reverse chronological order.
func (s *Store) RecentEvents(limit int) ([]Event, error) {
	return nil, fmt.Errorf("not implemented")
}

// LinkDetail returns the total click count and country breakdown for a prefix.
func (s *Store) LinkDetail(prefix string, since time.Time) (int64, []CountryCount, error) {
	return 0, nil, fmt.Errorf("not implemented")
}

// TopReferers returns referer domains ranked by click count.
func (s *Store) TopReferers(since time.Time, limit int) ([]RefererCount, error) {
	return nil, fmt.Errorf("not implemented")
}

// MissedLinks returns 404 request paths ranked by frequency.
func (s *Store) MissedLinks(since time.Time, limit int) ([]LinkCount, error) {
	return nil, fmt.Errorf("not implemented")
}

// UniqueVisitors returns unique IP counts alongside total click counts.
// If byPrefix is true, results are broken down by link prefix.
func (s *Store) UniqueVisitors(since time.Time, byPrefix bool) ([]UniqueStats, error) {
	return nil, fmt.Errorf("not implemented")
}

// WriteTopLinksCSV writes top-links results as CSV.
func WriteTopLinksCSV(w io.Writer, links []LinkCount) error {
	return fmt.Errorf("not implemented")
}

// WriteRecentEventsCSV writes recent-events results as CSV.
func WriteRecentEventsCSV(w io.Writer, events []Event) error {
	return fmt.Errorf("not implemented")
}

// WriteLinkDetailCSV writes link-detail results as CSV.
func WriteLinkDetailCSV(w io.Writer, total int64, countries []CountryCount) error {
	return fmt.Errorf("not implemented")
}

// WriteTopReferersCSV writes top-referers results as CSV.
func WriteTopReferersCSV(w io.Writer, referers []RefererCount) error {
	return fmt.Errorf("not implemented")
}

// WriteMissedLinksCSV writes missed-links results as CSV.
func WriteMissedLinksCSV(w io.Writer, misses []LinkCount) error {
	return fmt.Errorf("not implemented")
}

// WriteUniqueVisitorsCSV writes unique-visitors results as CSV.
func WriteUniqueVisitorsCSV(w io.Writer, stats []UniqueStats) error {
	return fmt.Errorf("not implemented")
}
