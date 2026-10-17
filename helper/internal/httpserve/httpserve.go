// Package httpserve publishes the helper's latest state frame over HTTP, for
// displays that pull from the network instead of listening on USB serial
// (the Roku channel in roku/). The contract is docs/http-api.md.
//
// A Server implements app.Displays, so the existing Runner drives it exactly
// as it drives serial displays: every frame the Runner broadcasts becomes the
// body of GET /state. The Server never detects sessions itself, never blocks
// a request on detection, and serves nothing but the frame it was given.
// GET /state answers 401 for a wrong or missing bearer token (when a token is
// set) and 503 until the first frame arrives. GET /healthz always answers ok.
package httpserve

import (
	"bytes"
	"crypto/subtle"
	"net"
	"net/http"
	"sync"
	"time"
)

// Server holds the latest frame and serves it. The zero value is not usable;
// call New.
type Server struct {
	token string

	mu     sync.RWMutex
	latest []byte
}

// New returns a Server. An empty token disables authentication, which the
// serve command only allows on loopback addresses.
func New(token string) *Server {
	return &Server{token: token}
}

// Discover is a no-op: HTTP clients find the server, not the other way round.
func (s *Server) Discover(time.Time) {}

// Count is always 1, so the Runner keeps producing frames whether or not a
// client is polling right now.
func (s *Server) Count() int { return 1 }

// Broadcast stores line (one protocol frame, newline optional) as the body of
// the next GET /state. The caller may reuse line afterwards.
func (s *Server) Broadcast(line []byte) {
	frame := bytes.TrimRight(line, "\r\n")
	stored := make([]byte, len(frame))
	copy(stored, frame)
	s.mu.Lock()
	s.latest = stored
	s.mu.Unlock()
}

// Handler returns the routes described in docs/http-api.md.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /state", s.handleState)
	return mux
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if !s.authorized(r) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="managents"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	s.mu.RLock()
	frame := s.latest
	s.mu.RUnlock()
	if frame == nil {
		w.Header().Set("Retry-After", "1")
		http.Error(w, "no state yet", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(frame)
}

func (s *Server) authorized(r *http.Request) bool {
	if s.token == "" {
		return true
	}
	want := []byte("Bearer " + s.token)
	got := []byte(r.Header.Get("Authorization"))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// IsLoopback reports whether a listen address such as "127.0.0.1:8787" or
// "localhost:8787" only accepts connections from this computer. An empty host
// (":8787") listens everywhere and is not loopback.
func IsLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
