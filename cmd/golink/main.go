// ABOUTME: golink entrypoint. v0 hello-world used to prove the deploy pipeline to kepler-452.
// ABOUTME: Real resolver/GeoIP/SIGHUP-reload functionality lands in issue #6.

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
)

// Version is the build identifier surfaced via -version. Overridden at link
// time by `make build` / `make build-linux` via -ldflags.
var Version = "dev"

func main() {
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println(Version)
		return
	}

	addr := resolveListenAddr()

	mux := http.NewServeMux()
	mux.HandleFunc("/", handleHello)

	log.Printf("golink %s listening on %s", Version, addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("golink: server exited: %v", err)
	}
}

// resolveListenAddr derives the bind address from environment variables, in
// the order specified by the kepler-452 deploy contract:
//
//  1. ADDR (full host:port, e.g. "127.0.0.1:18081")
//  2. PORT (port only; bound to 127.0.0.1 explicitly per kepler-452 convention)
//  3. fall back to "127.0.0.1:18081" so the binary is runnable locally
//     without any environment setup
func resolveListenAddr() string {
	if a := os.Getenv("ADDR"); a != "" {
		return a
	}
	if p := os.Getenv("PORT"); p != "" {
		return "127.0.0.1:" + p
	}
	return "127.0.0.1:18081"
}

func handleHello(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "hello from golink %s\n", Version)
	log.Printf("served %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
}
