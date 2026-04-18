// ABOUTME: Analytics event store backed by SQLite. Records one row per HTTP
// ABOUTME: request and exposes query methods for the golink stats CLI.

package analytics

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
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

const createTableSQL = `CREATE TABLE IF NOT EXISTS events (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	ts         TEXT    NOT NULL,
	remote_ip  TEXT    NOT NULL,
	country    TEXT    NOT NULL DEFAULT '',
	prefix     TEXT    NOT NULL DEFAULT '',
	path       TEXT    NOT NULL DEFAULT '',
	status     INTEGER NOT NULL,
	target     TEXT    NOT NULL DEFAULT '',
	referer    TEXT    NOT NULL DEFAULT '',
	user_agent TEXT    NOT NULL DEFAULT ''
)`

// Open opens or creates an analytics database at dbPath with WAL mode enabled.
func Open(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("analytics open: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("analytics WAL mode: %w", err)
	}

	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("analytics create table: %w", err)
	}

	return &Store{db: db}, nil
}

// OpenReadOnly opens an analytics database for read-only queries.
func OpenReadOnly(dbPath string) (*Store, error) {
	dsn := dbPath + "?mode=ro"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("analytics open read-only: %w", err)
	}
	return &Store{db: db}, nil
}

// Record inserts a single analytics event.
func (s *Store) Record(e Event) error {
	_, err := s.db.Exec(
		`INSERT INTO events (ts, remote_ip, country, prefix, path, status, target, referer, user_agent)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.TS.UTC().Format(time.RFC3339Nano),
		e.RemoteIP,
		e.Country,
		e.Prefix,
		e.Path,
		e.Status,
		e.Target,
		e.Referer,
		e.UserAgent,
	)
	return err
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
	query := `SELECT prefix, COUNT(*) as cnt FROM events`
	args := []interface{}{}

	if !since.IsZero() {
		query += ` WHERE ts >= ?`
		args = append(args, since.UTC().Format(time.RFC3339Nano))
	}
	query += ` GROUP BY prefix ORDER BY cnt DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LinkCount
	for rows.Next() {
		var lc LinkCount
		if err := rows.Scan(&lc.Prefix, &lc.Count); err != nil {
			return nil, err
		}
		results = append(results, lc)
	}
	return results, rows.Err()
}

// RecentEvents returns the most recent events in reverse chronological order.
func (s *Store) RecentEvents(limit int) ([]Event, error) {
	rows, err := s.db.Query(
		`SELECT ts, remote_ip, country, prefix, path, status, target, referer, user_agent
		 FROM events ORDER BY ts DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Event
	for rows.Next() {
		var e Event
		var tsStr string
		if err := rows.Scan(&tsStr, &e.RemoteIP, &e.Country, &e.Prefix, &e.Path,
			&e.Status, &e.Target, &e.Referer, &e.UserAgent); err != nil {
			return nil, err
		}
		e.TS, _ = time.Parse(time.RFC3339Nano, tsStr)
		results = append(results, e)
	}
	return results, rows.Err()
}

// LinkDetail returns the total click count and country breakdown for a prefix.
func (s *Store) LinkDetail(prefix string, since time.Time) (int64, []CountryCount, error) {
	// Total count.
	countQuery := `SELECT COUNT(*) FROM events WHERE prefix = ?`
	countArgs := []interface{}{prefix}
	if !since.IsZero() {
		countQuery += ` AND ts >= ?`
		countArgs = append(countArgs, since.UTC().Format(time.RFC3339Nano))
	}

	var total int64
	if err := s.db.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		return 0, nil, err
	}

	// Country breakdown.
	geoQuery := `SELECT country, COUNT(*) as cnt FROM events WHERE prefix = ?`
	geoArgs := []interface{}{prefix}
	if !since.IsZero() {
		geoQuery += ` AND ts >= ?`
		geoArgs = append(geoArgs, since.UTC().Format(time.RFC3339Nano))
	}
	geoQuery += ` GROUP BY country ORDER BY cnt DESC`

	rows, err := s.db.Query(geoQuery, geoArgs...)
	if err != nil {
		return total, nil, err
	}
	defer rows.Close()

	var countries []CountryCount
	for rows.Next() {
		var cc CountryCount
		if err := rows.Scan(&cc.Country, &cc.Count); err != nil {
			return total, nil, err
		}
		countries = append(countries, cc)
	}
	return total, countries, rows.Err()
}

// TopReferers returns referer domains ranked by click count.
// Empty referers are excluded. Domains are extracted from the raw referer URL.
func (s *Store) TopReferers(since time.Time, limit int) ([]RefererCount, error) {
	query := `SELECT referer FROM events WHERE referer != ''`
	args := []interface{}{}
	if !since.IsZero() {
		query += ` AND ts >= ?`
		args = append(args, since.UTC().Format(time.RFC3339Nano))
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Aggregate by domain in Go (avoids SQL string functions).
	domainCounts := make(map[string]int64)
	for rows.Next() {
		var rawReferer string
		if err := rows.Scan(&rawReferer); err != nil {
			return nil, err
		}
		domain := extractDomain(rawReferer)
		if domain != "" {
			domainCounts[domain]++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort by count descending.
	results := make([]RefererCount, 0, len(domainCounts))
	for domain, count := range domainCounts {
		results = append(results, RefererCount{Domain: domain, Count: count})
	}
	sortReferers(results)

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// MissedLinks returns 404 request paths ranked by frequency.
func (s *Store) MissedLinks(since time.Time, limit int) ([]LinkCount, error) {
	query := `SELECT prefix, COUNT(*) as cnt FROM events WHERE status = 404`
	args := []interface{}{}
	if !since.IsZero() {
		query += ` AND ts >= ?`
		args = append(args, since.UTC().Format(time.RFC3339Nano))
	}
	query += ` GROUP BY prefix ORDER BY cnt DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LinkCount
	for rows.Next() {
		var lc LinkCount
		if err := rows.Scan(&lc.Prefix, &lc.Count); err != nil {
			return nil, err
		}
		results = append(results, lc)
	}
	return results, rows.Err()
}

// UniqueVisitors returns unique IP counts alongside total click counts.
// If byPrefix is true, results are broken down by link prefix.
func (s *Store) UniqueVisitors(since time.Time, byPrefix bool) ([]UniqueStats, error) {
	var query string
	args := []interface{}{}

	if byPrefix {
		query = `SELECT prefix, COUNT(DISTINCT remote_ip) as unique_ips, COUNT(*) as total
				 FROM events`
		if !since.IsZero() {
			query += ` WHERE ts >= ?`
			args = append(args, since.UTC().Format(time.RFC3339Nano))
		}
		query += ` GROUP BY prefix ORDER BY total DESC`
	} else {
		query = `SELECT '' as prefix, COUNT(DISTINCT remote_ip) as unique_ips, COUNT(*) as total
				 FROM events`
		if !since.IsZero() {
			query += ` WHERE ts >= ?`
			args = append(args, since.UTC().Format(time.RFC3339Nano))
		}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []UniqueStats
	for rows.Next() {
		var us UniqueStats
		if err := rows.Scan(&us.Prefix, &us.UniqueIPs, &us.TotalClicks); err != nil {
			return nil, err
		}
		results = append(results, us)
	}
	return results, rows.Err()
}

// extractDomain parses a URL and returns just the hostname.
func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return u.Hostname()
}

// sortReferers sorts by count descending (stable).
func sortReferers(refs []RefererCount) {
	for i := 1; i < len(refs); i++ {
		for j := i; j > 0 && refs[j].Count > refs[j-1].Count; j-- {
			refs[j], refs[j-1] = refs[j-1], refs[j]
		}
	}
}

// --- CSV writers ---

// WriteTopLinksCSV writes top-links results as CSV.
func WriteTopLinksCSV(w io.Writer, links []LinkCount) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"prefix", "count"}); err != nil {
		return err
	}
	for _, l := range links {
		if err := cw.Write([]string{l.Prefix, strconv.FormatInt(l.Count, 10)}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteRecentEventsCSV writes recent-events results as CSV.
func WriteRecentEventsCSV(w io.Writer, events []Event) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"ts", "remote_ip", "country", "prefix", "path", "status", "target", "referer", "user_agent"}); err != nil {
		return err
	}
	for _, e := range events {
		if err := cw.Write([]string{
			e.TS.UTC().Format(time.RFC3339),
			e.RemoteIP, e.Country, e.Prefix, e.Path,
			strconv.Itoa(e.Status), e.Target, e.Referer, e.UserAgent,
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteLinkDetailCSV writes link-detail results as CSV.
func WriteLinkDetailCSV(w io.Writer, total int64, countries []CountryCount) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"country", "count"}); err != nil {
		return err
	}
	for _, c := range countries {
		if err := cw.Write([]string{c.Country, strconv.FormatInt(c.Count, 10)}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteTopReferersCSV writes top-referers results as CSV.
func WriteTopReferersCSV(w io.Writer, referers []RefererCount) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"domain", "count"}); err != nil {
		return err
	}
	for _, r := range referers {
		if err := cw.Write([]string{r.Domain, strconv.FormatInt(r.Count, 10)}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// WriteMissedLinksCSV writes missed-links results as CSV.
func WriteMissedLinksCSV(w io.Writer, misses []LinkCount) error {
	return WriteTopLinksCSV(w, misses)
}

// WriteUniqueVisitorsCSV writes unique-visitors results as CSV.
func WriteUniqueVisitorsCSV(w io.Writer, stats []UniqueStats) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"prefix", "unique_ips", "total_clicks"}); err != nil {
		return err
	}
	for _, s := range stats {
		if err := cw.Write([]string{
			s.Prefix,
			strconv.FormatInt(s.UniqueIPs, 10),
			strconv.FormatInt(s.TotalClicks, 10),
		}); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
