// ABOUTME: golink entrypoint. Loads config, initialises GeoIP and resolvers,
// ABOUTME: starts the HTTP server, and handles SIGHUP reload + SIGTERM shutdown.

package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/tigger04/golink/internal/geoip"
	"github.com/tigger04/golink/internal/router"
	"github.com/tigger04/golink/internal/server"
	"gopkg.in/yaml.v3"
)

// Version is the build identifier surfaced via -version. Overridden at link
// time by the build system via -ldflags.
var Version = "dev"

// appConfig is the YAML config file schema.
type appConfig struct {
	ResolversDir string `yaml:"resolvers_dir"`
}

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println(Version)
		os.Exit(0)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	cfg := loadConfig(logger)
	resolversDir := cfg.ResolversDir

	// Load resolvers — refuse to start if any YAML is invalid.
	rtr, err := router.LoadDir(resolversDir)
	if err != nil {
		logger.Error("failed to load resolvers", "dir", resolversDir, "error", err)
		os.Exit(1)
	}
	if len(rtr.Prefixes()) == 0 {
		logger.Error("no resolvers loaded — refusing to start", "dir", resolversDir)
		os.Exit(1)
	}
	logger.Info("resolvers loaded", "prefixes", rtr.Prefixes(), "dir", resolversDir)

	// Initialise GeoIP.
	stateDir := os.Getenv("STATE_DIRECTORY")
	if stateDir == "" {
		stateDir = "."
	}
	geo := geoip.New(geoip.Config{
		Dir:    stateDir,
		Logger: logger,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := geo.Start(ctx); err != nil {
		logger.Error("geoip start failed", "error", err)
		// Non-fatal: service runs without GeoIP.
	}

	// Start HTTP server.
	addr := resolveListenAddr()
	srv := server.New(server.Config{
		Addr:   addr,
		Logger: logger,
	}, rtr, geo)

	logger.Info("golink starting", "version", Version, "addr", addr)

	// Signal handling.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		for sig := range sigCh {
			switch sig {
			case syscall.SIGHUP:
				logger.Info("SIGHUP received, reloading")
				handleReload(srv, geo, resolversDir, logger)
			case syscall.SIGTERM, syscall.SIGINT:
				logger.Info("shutdown signal received", "signal", sig.String())
				shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 25*time.Second)
				defer shutdownCancel()
				if err := srv.Shutdown(shutdownCtx); err != nil {
					logger.Error("shutdown error", "error", err)
				}
				cancel() // stop GeoIP background goroutine
				return
			}
		}
	}()

	if err := srv.ListenAndServe(); err != nil && err.Error() != "http: Server closed" {
		logger.Error("server exited", "error", err)
		os.Exit(1)
	}
}

func handleReload(srv *server.Server, geo *geoip.Service, resolversDir string, logger *slog.Logger) {
	// Try to load new resolvers. Any failure rejects the entire reload.
	newRtr, err := router.LoadDir(resolversDir)
	if err != nil {
		logger.Error("reload failed: resolver load error, keeping previous config", "error", err)
		return
	}
	if len(newRtr.Prefixes()) == 0 {
		logger.Error("reload failed: no valid resolvers, keeping previous config")
		return
	}

	// Try to reload GeoIP.
	if err := geo.Reload(); err != nil {
		logger.Warn("reload: geoip reopen failed, keeping previous database", "error", err)
		// Continue with old GeoIP — resolver reload still takes effect.
	}

	srv.SetState(newRtr, geo)
	logger.Info("reload complete", "prefixes", newRtr.Prefixes())
}

// loadConfig reads the layered YAML config per the kepler-452 deploy contract.
func loadConfig(logger *slog.Logger) appConfig {
	cfg := appConfig{
		ResolversDir: "examples/resolvers",
	}

	// Layer 1: defaults.yaml (if it exists).
	if data, err := os.ReadFile("config/defaults.yaml"); err == nil {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			logger.Warn("config: failed to parse defaults.yaml", "error", err)
		}
	}

	// Layer 2: host-specific config via CONFIG_PATH.
	if configPath := os.Getenv("CONFIG_PATH"); configPath != "" {
		if data, err := os.ReadFile(configPath); err == nil {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				logger.Warn("config: failed to parse host config", "path", configPath, "error", err)
			}
		}
	}

	// Resolve relative paths against working directory.
	if !filepath.IsAbs(cfg.ResolversDir) {
		if wd, err := os.Getwd(); err == nil {
			cfg.ResolversDir = filepath.Join(wd, cfg.ResolversDir)
		}
	}

	return cfg
}

// resolveListenAddr derives the bind address from environment variables, in
// the order specified by the kepler-452 deploy contract:
//
//  1. ADDR (full host:port)
//  2. PORT (port only, bound to 127.0.0.1)
//  3. fallback to 127.0.0.1:18081
func resolveListenAddr() string {
	if a := os.Getenv("ADDR"); a != "" {
		return a
	}
	if p := os.Getenv("PORT"); p != "" {
		return "127.0.0.1:" + p
	}
	return "127.0.0.1:18081"
}
