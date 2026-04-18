// ABOUTME: HTTP server with structured JSON logging, X-Forwarded-For extraction,
// ABOUTME: and graceful shutdown. The handler dispatches to the router and writes
// ABOUTME: 302 redirects or 404 responses.

package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tigger04/golink/internal/analytics"
	"github.com/tigger04/golink/internal/resolver"
	"github.com/tigger04/golink/internal/router"
)

// GeoLookup abstracts the GeoIP service for testability.
type GeoLookup interface {
	CountryCode(ip net.IP) string
}

// Config holds the server configuration.
type Config struct {
	Addr      string
	Logger    *slog.Logger
	Analytics *analytics.Store
}

// Server is the golink HTTP server.
type Server struct {
	cfg    Config
	http   *http.Server
	rtr    *router.Router
	geo    GeoLookup
	logger *slog.Logger
}

// New creates a server. The router and geo lookup can be swapped via SetState.
func New(cfg Config, rtr *router.Router, geo GeoLookup) *Server {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	s := &Server{
		cfg:    cfg,
		rtr:    rtr,
		geo:    geo,
		logger: cfg.Logger,
	}
	s.http = &http.Server{
		Addr:    cfg.Addr,
		Handler: s,
	}
	return s
}

// SetState atomically swaps the router and GeoIP service. Used during SIGHUP reload.
func (s *Server) SetState(rtr *router.Router, geo GeoLookup) {
	s.rtr = rtr
	s.geo = geo
}

// Router returns the current router (for reload logic).
func (s *Server) Router() *router.Router {
	return s.rtr
}

// Geo returns the current GeoIP lookup (for reload logic).
func (s *Server) Geo() GeoLookup {
	return s.geo
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	return s.http.ListenAndServe()
}

// Serve accepts connections on a listener (for testing).
func (s *Server) Serve(ln net.Listener) error {
	return s.http.Serve(ln)
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

// Handler returns the HTTP handler (for httptest).
func (s *Server) Handler() http.Handler {
	return s
}

// ServeHTTP is the main request handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Snapshot the current state for this request.
	rtr := s.rtr
	geo := s.geo

	clientIP := extractClientIP(r)
	prefix, remaining := splitPath(r.URL.Path)
	referer := r.Header.Get("Referer")
	userAgent := r.Header.Get("User-Agent")

	entry := logEntry{
		RemoteIP: clientIP.String(),
		Prefix:   prefix,
		Path:     remaining,
	}

	if prefix == "" {
		entry.Status = 404
		writeNotFound(w)
		s.emitLog(entry, start)
		s.recordAnalytics(start, entry, referer, userAgent)
		return
	}

	res := rtr.Lookup(prefix)
	if res == nil {
		entry.Status = 404
		writeNotFound(w)
		s.emitLog(entry, start)
		s.recordAnalytics(start, entry, referer, userAgent)
		return
	}

	var countryCode string
	if geo != nil {
		countryCode = geo.CountryCode(clientIP)
	}
	entry.Country = countryCode

	result, err := res.Resolve(resolver.Request{
		Path:        remaining,
		CountryCode: countryCode,
		RemoteIP:    clientIP,
	})
	if err != nil {
		if err == resolver.ErrNotFound {
			entry.Status = 404
			writeNotFound(w)
		} else {
			entry.Status = 500
			w.WriteHeader(http.StatusInternalServerError)
		}
		s.emitLog(entry, start)
		s.recordAnalytics(start, entry, referer, userAgent)
		return
	}

	entry.Status = result.Code
	entry.Target = result.URL
	w.Header().Set("Location", result.URL)
	w.WriteHeader(result.Code)
	s.emitLog(entry, start)
	s.recordAnalytics(start, entry, referer, userAgent)
}

// recordAnalytics writes an event to the analytics store if one is configured.
// Errors are logged but never affect the HTTP response.
func (s *Server) recordAnalytics(ts time.Time, entry logEntry, referer, userAgent string) {
	if s.cfg.Analytics == nil {
		return
	}
	err := s.cfg.Analytics.Record(analytics.Event{
		TS:        ts,
		RemoteIP:  entry.RemoteIP,
		Country:   entry.Country,
		Prefix:    entry.Prefix,
		Path:      entry.Path,
		Status:    entry.Status,
		Target:    entry.Target,
		Referer:   referer,
		UserAgent: userAgent,
	})
	if err != nil {
		s.logger.Error("analytics record failed", "error", err)
	}
}

// logEntry is the structured log format for each request.
type logEntry struct {
	TS        string `json:"ts"`
	RemoteIP  string `json:"remote_ip"`
	Country   string `json:"country"`
	Prefix    string `json:"prefix"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	Target    string `json:"target"`
	LatencyUS int64  `json:"latency_us"`
}

// LogWriter is the destination for raw JSON log lines. Defaults to os.Stderr.
var LogWriter io.Writer = os.Stderr

func (s *Server) emitLog(entry logEntry, start time.Time) {
	entry.TS = start.UTC().Format(time.RFC3339Nano)
	entry.LatencyUS = time.Since(start).Microseconds()

	line, err := json.Marshal(entry)
	if err != nil {
		s.logger.Error("failed to marshal log entry", "error", err)
		return
	}
	line = append(line, '\n')
	_, _ = LogWriter.Write(line)
}

func writeNotFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
}

// extractClientIP reads the real client IP from X-Forwarded-For (leftmost)
// or falls back to RemoteAddr.
func extractClientIP(r *http.Request) net.IP {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the leftmost IP (the original client).
		parts := strings.SplitN(xff, ",", 2)
		ip := net.ParseIP(strings.TrimSpace(parts[0]))
		if ip != nil {
			return ip
		}
	}

	// Fall back to RemoteAddr.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return nil
	}
	return net.ParseIP(host)
}

// splitPath extracts the first path segment as the prefix and the rest as
// the remaining path. Leading and trailing slashes are trimmed.
func splitPath(path string) (prefix, remaining string) {
	path = strings.Trim(path, "/")
	if path == "" {
		return "", ""
	}

	idx := strings.IndexByte(path, '/')
	if idx < 0 {
		return path, ""
	}
	return path[:idx], path[idx+1:]
}
