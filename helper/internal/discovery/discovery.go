// Package discovery lets a network display find `managents serve` on the LAN
// when the computer's address changes, instead of needing the address baked
// in again. The exchange is described in docs/http-api.md#discovery.
//
// A display broadcasts one short UDP datagram to the serve port:
//
//	rokuagents-discover v1 <token id> <nonce>
//
// where the token id is the first 16 hex characters of sha256(token) and the
// nonce is fresh random hex. The Responder answers, unicast to the sender and
// only when the token id matches its own token, with
//
//	rokuagents v1 <base url> <mac>
//
// where mac is the first 32 hex characters of HMAC-SHA256(token,
// "<nonce> <base url>"). The token stays the trust anchor both ways: the
// helper never answers a display that does not hold its token, and a display
// can reject a reply from anything that does not hold it either, so a rogue
// host on the LAN cannot redirect the display (and its bearer token) to
// itself. Requests that do not match are dropped without a reply, so the
// port reveals nothing to scanners.
package discovery

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

const (
	// RequestPrefix starts every discovery request.
	RequestPrefix = "rokuagents-discover v1"
	// ReplyPrefix starts every discovery reply.
	ReplyPrefix = "rokuagents v1"
	// MaxDatagram is the largest request the Responder reads. Anything longer
	// is not a discovery request.
	MaxDatagram = 512
	// RepliesPerSecond caps how often the Responder answers, so a burst of
	// requests (a display sends to more than one broadcast address) or a
	// flood cannot turn it into a noisy sender.
	RepliesPerSecond = 10
	// MinTokenLength is the shortest token discovery runs with. Every LAN
	// member sees the token id in the display's broadcasts, and a reply is
	// keyed with the token too, so both are offline guessing oracles for a
	// weak token. 32 characters is what `openssl rand -hex 16` and the menu
	// bar app produce: 128 random bits, far beyond guessing.
	MinTokenLength = 32
)

// TokenStrongEnough reports whether token is long enough to expose its id
// and signatures on the LAN. It checks length only; the docs say to generate
// the token at random.
func TokenStrongEnough(token string) bool {
	return len(token) >= MinTokenLength
}

// TokenID is the public name of a token in discovery requests: the first 16
// hex characters of its SHA-256. It identifies the token without revealing it.
func TokenID(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:8])
}

// Sign returns the mac a reply carries for nonce and baseURL: the first 32
// hex characters of HMAC-SHA256 keyed with the token.
func Sign(token, nonce, baseURL string) string {
	m := hmac.New(sha256.New, []byte(token))
	m.Write([]byte(nonce + " " + baseURL))
	return hex.EncodeToString(m.Sum(nil)[:16])
}

// Responder answers discovery requests for one token. The zero value is not
// usable; call New. A Responder is not safe for concurrent use; Serve calls
// Reply from one goroutine.
type Responder struct {
	token string
	id    string
	port  int
	host  string

	// LocalAddrFor returns this computer's address on the route to remote.
	// It is only used when New was given no host. Tests replace it.
	LocalAddrFor func(remote netip.Addr) (netip.Addr, error)
	// Now is the clock for the reply rate limit. Tests replace it.
	Now func() time.Time
	// Logger receives debug lines about replies; nil discards them.
	Logger *slog.Logger

	windowStart time.Time
	replies     int
}

// New returns a Responder that advertises http://host:port. An empty host
// advertises whichever local address routes to the asker, which is what a
// server listening on every interface wants.
func New(token, host string, port int) *Responder {
	return &Responder{
		token:        token,
		id:           TokenID(token),
		port:         port,
		host:         host,
		LocalAddrFor: RouteLocalAddr,
		Now:          time.Now,
	}
}

// Reply returns the datagram to send back to from for request, and false
// when the request gets no answer: malformed, for another token, or over the
// rate limit.
func (r *Responder) Reply(request []byte, from netip.AddrPort) ([]byte, bool) {
	nonce, ok := r.parse(request)
	if !ok {
		return nil, false
	}
	host := r.host
	if host == "" {
		local, err := r.LocalAddrFor(from.Addr().Unmap())
		if err != nil || !local.IsValid() {
			r.debug("no local address for discovery reply", "err", err)
			return nil, false
		}
		host = local.Unmap().String()
	}
	if !r.allow() {
		r.debug("discovery reply rate limited")
		return nil, false
	}
	baseURL := "http://" + net.JoinHostPort(host, strconv.Itoa(r.port))
	r.debug("answering discovery request", "url", baseURL)
	return []byte(ReplyPrefix + " " + baseURL + " " + Sign(r.token, nonce, baseURL)), true
}

// parse returns the nonce of a well-formed request for this token.
func (r *Responder) parse(request []byte) (string, bool) {
	if len(request) > MaxDatagram {
		return "", false
	}
	rest, ok := strings.CutPrefix(strings.TrimRight(string(request), "\r\n"), RequestPrefix+" ")
	if !ok {
		return "", false
	}
	id, nonce, ok := strings.Cut(rest, " ")
	if !ok || !isHex(nonce, 16, 64) {
		return "", false
	}
	if subtle.ConstantTimeCompare([]byte(strings.ToLower(id)), []byte(r.id)) != 1 {
		return "", false
	}
	return nonce, true
}

func (r *Responder) allow() bool {
	now := r.Now()
	if now.Sub(r.windowStart) >= time.Second || now.Before(r.windowStart) {
		r.windowStart = now
		r.replies = 0
	}
	if r.replies >= RepliesPerSecond {
		return false
	}
	r.replies++
	return true
}

func (r *Responder) debug(msg string, args ...any) {
	if r.Logger != nil {
		r.Logger.Debug(msg, args...)
	}
}

// Serve answers requests arriving on conn until ctx ends or conn fails, and
// closes conn when it returns. It returns nil when ctx ended.
func (r *Responder) Serve(ctx context.Context, conn net.PacketConn) error {
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	defer conn.Close()

	buf := make([]byte, MaxDatagram+1)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}
		from, ok := addrPort(addr)
		if !ok {
			continue
		}
		reply, ok := r.Reply(buf[:n], from)
		if !ok {
			continue
		}
		if _, err := conn.WriteTo(reply, addr); err != nil {
			r.debug("discovery reply failed", "err", err)
		}
	}
}

// RouteLocalAddr returns the local address the system would send from to
// reach remote. Connecting a UDP socket only consults the routing table; it
// sends nothing.
func RouteLocalAddr(remote netip.Addr) (netip.Addr, error) {
	conn, err := net.DialUDP("udp", nil, net.UDPAddrFromAddrPort(netip.AddrPortFrom(remote, 9)))
	if err != nil {
		return netip.Addr{}, err
	}
	defer conn.Close()
	local, ok := addrPort(conn.LocalAddr())
	if !ok {
		return netip.Addr{}, errors.New("local address is not an IP address")
	}
	return local.Addr().Unmap(), nil
}

func addrPort(addr net.Addr) (netip.AddrPort, bool) {
	udp, ok := addr.(*net.UDPAddr)
	if !ok {
		return netip.AddrPort{}, false
	}
	ap := udp.AddrPort()
	return ap, ap.IsValid()
}

func isHex(s string, minLen, maxLen int) bool {
	if len(s) < minLen || len(s) > maxLen {
		return false
	}
	for _, c := range s {
		if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
			return false
		}
	}
	return true
}
