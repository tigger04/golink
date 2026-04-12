// ABOUTME: Self-managed GeoIP service wrapping DB-IP IP-to-Country Lite.
// ABOUTME: Downloads on first start if missing/stale, refreshes daily in the
// ABOUTME: background, degrades gracefully if unavailable.

package geoip

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

const (
	// DefaultMaxAge is how old the database can be before a refresh is needed.
	DefaultMaxAge = 30 * 24 * time.Hour // 30 days

	// DefaultCheckInterval is how often the background goroutine checks staleness.
	DefaultCheckInterval = 24 * time.Hour

	dbFilename = "dbip-country-lite.mmdb"
)

// countryRecord is the MMDB lookup result structure.
type countryRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

// Config configures the GeoIP service.
type Config struct {
	// Dir is the directory to store the database file. Defaults to ".".
	Dir string

	// DownloadURL overrides the DB-IP download URL (for testing).
	// If empty, the current month's URL is derived automatically.
	DownloadURL string

	// MaxAge is the maximum age before a refresh is triggered. Defaults to 30 days.
	MaxAge time.Duration

	// CheckInterval is how often the background goroutine checks. Defaults to 24h.
	CheckInterval time.Duration

	// HTTPClient overrides the HTTP client for downloads (for testing).
	HTTPClient *http.Client

	// Logger for status messages.
	Logger *slog.Logger

	// Now overrides time.Now for testing staleness checks.
	Now func() time.Time
}

// Service manages the GeoIP database lifecycle and provides lookups.
type Service struct {
	cfg    Config
	dbPath string

	mu     sync.RWMutex
	reader *maxminddb.Reader

	cancel context.CancelFunc
	done   chan struct{}
}

// New creates a GeoIP service. Call Start to download/open the database
// and begin background refresh.
func New(cfg Config) *Service {
	if cfg.Dir == "" {
		cfg.Dir = "."
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = DefaultMaxAge
	}
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = DefaultCheckInterval
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 60 * time.Second}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}

	return &Service{
		cfg:    cfg,
		dbPath: filepath.Join(cfg.Dir, dbFilename),
		done:   make(chan struct{}),
	}
}

// Start opens the database (downloading if needed) and starts background refresh.
func (s *Service) Start(ctx context.Context) error {
	if s.isStale() {
		if err := s.download(); err != nil {
			s.cfg.Logger.Error("geoip: initial download failed, continuing without GeoIP", "error", err)
		}
	}

	if err := s.openDB(); err != nil {
		s.cfg.Logger.Warn("geoip: could not open database, lookups will return empty", "error", err)
	}

	bgCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	go s.backgroundRefresh(bgCtx)
	return nil
}

// CountryCode returns the ISO 3166-1 alpha-2 country code for the given IP,
// or "" if the lookup fails or the database is unavailable.
func (s *Service) CountryCode(ip net.IP) string {
	s.mu.RLock()
	r := s.reader
	s.mu.RUnlock()

	if r == nil {
		return ""
	}

	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return ""
	}

	var record countryRecord
	if err := r.Lookup(addr).Decode(&record); err != nil {
		return ""
	}
	return record.Country.ISOCode
}

// Reload re-opens the database file from disk. Used during SIGHUP.
func (s *Service) Reload() error {
	return s.openDB()
}

// Close stops the background goroutine and closes the database.
func (s *Service) Close() error {
	if s.cancel != nil {
		s.cancel()
		<-s.done
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reader != nil {
		return s.reader.Close()
	}
	return nil
}

func (s *Service) openDB() error {
	reader, err := maxminddb.Open(s.dbPath)
	if err != nil {
		return fmt.Errorf("open mmdb: %w", err)
	}

	s.mu.Lock()
	old := s.reader
	s.reader = reader
	s.mu.Unlock()

	if old != nil {
		// Close old reader after a delay to let in-flight requests drain.
		go func() {
			time.Sleep(30 * time.Second)
			_ = old.Close()
		}()
	}
	return nil
}

func (s *Service) isStale() bool {
	info, err := os.Stat(s.dbPath)
	if err != nil {
		return true // missing or unreadable
	}
	return s.cfg.Now().Sub(info.ModTime()) > s.cfg.MaxAge
}

func (s *Service) download() error {
	url := s.cfg.DownloadURL
	if url == "" {
		now := s.cfg.Now()
		url = fmt.Sprintf("https://download.db-ip.com/free/dbip-country-lite-%d-%02d.mmdb.gz",
			now.Year(), now.Month())
	}

	s.cfg.Logger.Info("geoip: downloading database", "url", url)

	resp, err := s.cfg.HTTPClient.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("download: gunzip: %w", err)
	}
	defer func() { _ = gz.Close() }()

	// Write to a temp file, then atomic rename.
	tmp, err := os.CreateTemp(s.cfg.Dir, "geoip-*.mmdb.tmp")
	if err != nil {
		return fmt.Errorf("download: create temp: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := io.Copy(tmp, gz); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("download: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("download: close: %w", err)
	}

	if err := os.Rename(tmpPath, s.dbPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("download: rename: %w", err)
	}

	s.cfg.Logger.Info("geoip: database updated", "path", s.dbPath)
	return nil
}

func (s *Service) backgroundRefresh(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(s.cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if s.isStale() {
				if err := s.download(); err != nil {
					s.cfg.Logger.Error("geoip: background refresh failed", "error", err)
					continue
				}
				if err := s.openDB(); err != nil {
					s.cfg.Logger.Error("geoip: reopen after refresh failed", "error", err)
				}
			}
		}
	}
}
