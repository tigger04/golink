// ABOUTME: Core resolver types and interface. Defines the contract that all
// ABOUTME: resolver implementations (e.g. templated) must satisfy.

package resolver

import (
	"errors"
	"net"
)

// ErrNotFound is the sentinel for "I understood the request but have nothing
// for it." The server maps this to HTTP 404.
var ErrNotFound = errors.New("not found")

// Request carries the information a resolver needs to produce a redirect URL.
type Request struct {
	Path        string // path segments after the prefix, joined with "/"
	CountryCode string // ISO 3166-1 alpha-2, "" if unknown
	RemoteIP    net.IP
}

// Result is a successful resolution: a URL to redirect to and an HTTP status.
type Result struct {
	URL  string
	Code int // always 302 for now
}

// Resolver resolves a request into a redirect result.
type Resolver interface {
	Resolve(Request) (Result, error)
}
