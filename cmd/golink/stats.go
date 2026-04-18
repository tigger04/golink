// ABOUTME: Stats subcommand for querying the analytics database.
// ABOUTME: Invoked via `golink stats <report> [flags]`.

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/tigger04/golink/internal/analytics"
)

func runStats(args []string) {
	fs := flag.NewFlagSet("stats", flag.ExitOnError)
	lastFlag := fs.String("last", "", "time range filter (e.g. 24h, 7d, 30d)")
	csvFlag := fs.Bool("csv", false, "output as CSV")
	limitFlag := fs.Int("limit", 20, "maximum number of rows")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: golink stats <report> [flags]\n\n")
		fmt.Fprintf(os.Stderr, "Reports:\n")
		fmt.Fprintf(os.Stderr, "  top               Top links by click count\n")
		fmt.Fprintf(os.Stderr, "  recent            Recent events (reverse chronological)\n")
		fmt.Fprintf(os.Stderr, "  link <prefix>     Click count + geo breakdown for a link\n")
		fmt.Fprintf(os.Stderr, "  referers          Top referer domains\n")
		fmt.Fprintf(os.Stderr, "  misses            404 paths ranked by frequency\n")
		fmt.Fprintf(os.Stderr, "  unique            Unique visitors vs total clicks\n")
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		fs.PrintDefaults()
	}

	if len(args) == 0 {
		fs.Usage()
		os.Exit(2)
	}

	report := args[0]
	if err := fs.Parse(args[1:]); err != nil {
		os.Exit(2)
	}

	// Parse the time range.
	var since time.Time
	if *lastFlag != "" {
		d, err := parseDuration(*lastFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "invalid --last value %q: %v\n", *lastFlag, err)
			os.Exit(2)
		}
		since = time.Now().Add(-d)
	}

	// Open the analytics database read-only.
	stateDir := os.Getenv("STATE_DIRECTORY")
	if stateDir == "" {
		stateDir = "."
	}
	dbPath := filepath.Join(stateDir, "analytics.db")
	store, err := analytics.OpenReadOnly(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open analytics database at %s: %v\n", dbPath, err)
		os.Exit(1)
	}
	defer store.Close()

	switch report {
	case "top":
		statsTop(store, since, *limitFlag, *csvFlag)
	case "recent":
		statsRecent(store, *limitFlag, *csvFlag)
	case "link":
		prefix := fs.Arg(0)
		if prefix == "" {
			fmt.Fprintf(os.Stderr, "Usage: golink stats link <prefix> [flags]\n")
			os.Exit(2)
		}
		statsLink(store, prefix, since, *csvFlag)
	case "referers":
		statsReferers(store, since, *limitFlag, *csvFlag)
	case "misses":
		statsMisses(store, since, *limitFlag, *csvFlag)
	case "unique":
		statsUnique(store, since, *csvFlag)
	default:
		fmt.Fprintf(os.Stderr, "unknown report: %s\n", report)
		fs.Usage()
		os.Exit(2)
	}
}

func statsTop(store *analytics.Store, since time.Time, limit int, csvOut bool) {
	links, err := store.TopLinks(since, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query error: %v\n", err)
		os.Exit(1)
	}
	if csvOut {
		if err := analytics.WriteTopLinksCSV(os.Stdout, links); err != nil {
			fmt.Fprintf(os.Stderr, "csv error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "PREFIX\tCOUNT\n")
	for _, l := range links {
		fmt.Fprintf(tw, "%s\t%d\n", l.Prefix, l.Count)
	}
	tw.Flush()
}

func statsRecent(store *analytics.Store, limit int, csvOut bool) {
	events, err := store.RecentEvents(limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query error: %v\n", err)
		os.Exit(1)
	}
	if csvOut {
		if err := analytics.WriteRecentEventsCSV(os.Stdout, events); err != nil {
			fmt.Fprintf(os.Stderr, "csv error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "TIME\tIP\tCOUNTRY\tPREFIX\tPATH\tSTATUS\tTARGET\n")
	for _, e := range events {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%s\n",
			e.TS.UTC().Format("2006-01-02 15:04:05"),
			e.RemoteIP, e.Country, e.Prefix, e.Path, e.Status, e.Target)
	}
	tw.Flush()
}

func statsLink(store *analytics.Store, prefix string, since time.Time, csvOut bool) {
	total, countries, err := store.LinkDetail(prefix, since)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query error: %v\n", err)
		os.Exit(1)
	}
	if csvOut {
		if err := analytics.WriteLinkDetailCSV(os.Stdout, total, countries); err != nil {
			fmt.Fprintf(os.Stderr, "csv error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	fmt.Printf("Link: /%s\n", prefix)
	fmt.Printf("Total clicks: %d\n\n", total)
	if len(countries) > 0 {
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(tw, "COUNTRY\tCOUNT\n")
		for _, c := range countries {
			country := c.Country
			if country == "" {
				country = "(unknown)"
			}
			fmt.Fprintf(tw, "%s\t%d\n", country, c.Count)
		}
		tw.Flush()
	}
}

func statsReferers(store *analytics.Store, since time.Time, limit int, csvOut bool) {
	referers, err := store.TopReferers(since, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query error: %v\n", err)
		os.Exit(1)
	}
	if csvOut {
		if err := analytics.WriteTopReferersCSV(os.Stdout, referers); err != nil {
			fmt.Fprintf(os.Stderr, "csv error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "DOMAIN\tCOUNT\n")
	for _, r := range referers {
		fmt.Fprintf(tw, "%s\t%d\n", r.Domain, r.Count)
	}
	tw.Flush()
}

func statsMisses(store *analytics.Store, since time.Time, limit int, csvOut bool) {
	misses, err := store.MissedLinks(since, limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query error: %v\n", err)
		os.Exit(1)
	}
	if csvOut {
		if err := analytics.WriteMissedLinksCSV(os.Stdout, misses); err != nil {
			fmt.Fprintf(os.Stderr, "csv error: %v\n", err)
			os.Exit(1)
		}
		return
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "PREFIX\tCOUNT\n")
	for _, m := range misses {
		fmt.Fprintf(tw, "%s\t%d\n", m.Prefix, m.Count)
	}
	tw.Flush()
}

func statsUnique(store *analytics.Store, since time.Time, csvOut bool) {
	stats, err := store.UniqueVisitors(since, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query error: %v\n", err)
		os.Exit(1)
	}

	// Also get overall totals.
	overall, err := store.UniqueVisitors(since, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "query error: %v\n", err)
		os.Exit(1)
	}

	if csvOut {
		// Combine overall + per-prefix.
		all := append(overall, stats...)
		if err := analytics.WriteUniqueVisitorsCSV(os.Stdout, all); err != nil {
			fmt.Fprintf(os.Stderr, "csv error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if len(overall) > 0 {
		fmt.Printf("Overall: %d unique visitors, %d total clicks\n\n",
			overall[0].UniqueIPs, overall[0].TotalClicks)
	}

	if len(stats) > 0 {
		tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintf(tw, "PREFIX\tUNIQUE IPS\tTOTAL CLICKS\n")
		for _, s := range stats {
			fmt.Fprintf(tw, "%s\t%d\t%d\n", s.Prefix, s.UniqueIPs, s.TotalClicks)
		}
		tw.Flush()
	}
}

// parseDuration extends time.ParseDuration with support for "d" (days).
func parseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid days: %w", err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}
