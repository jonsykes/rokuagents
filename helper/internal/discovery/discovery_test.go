package discovery

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// A synthetic token. The vectors below were computed independently (Python's
// hashlib and hmac), so they also pin what a display has to compute.
const (
	testToken = "00112233445566778899aabbccddeeff"
	testID    = "5947d7c33d783f94"
	testNonce = "0a1b2c3d4e5f6789"
	testURL   = "http://192.168.1.20:8787"
	testMAC   = "74fe2a399359ebb7cea3878bb17d357c"
)

var (
	tv     = netip.MustParseAddrPort("192.168.1.63:50123")
	laptop = netip.MustParseAddr("192.168.1.20")
)

func TestTokenIDAndSignMatchKnownVectors(t *testing.T) {
	if got := TokenID(testToken); got != testID {
		t.Errorf("TokenID = %q, want %q", got, testID)
	}
	if got := Sign(testToken, testNonce, testURL); got != testMAC {
		t.Errorf("Sign = %q, want %q", got, testMAC)
	}
}

func TestTokenStrongEnough(t *testing.T) {
	tests := []struct {
		token string
		want  bool
	}{
		{"", false},
		{"hunter2", false},
		{testToken[:31], false},
		{testToken, true},
		{testToken + testToken, true},
	}
	for _, tt := range tests {
		if got := TokenStrongEnough(tt.token); got != tt.want {
			t.Errorf("TokenStrongEnough(%d chars) = %v, want %v", len(tt.token), got, tt.want)
		}
	}
}

// fakeClock is a settable clock for the rate limit.
type fakeClock struct{ now time.Time }

func (c *fakeClock) Now() time.Time { return c.now }

func newTestResponder(host string) (*Responder, *fakeClock) {
	clock := &fakeClock{now: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	r := New(testToken, host, 8787)
	r.Now = clock.Now
	r.LocalAddrFor = func(remote netip.Addr) (netip.Addr, error) {
		if remote != tv.Addr() {
			return netip.Addr{}, errors.New("unexpected remote " + remote.String())
		}
		return laptop, nil
	}
	return r, clock
}

func TestReply(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		request string
		want    string // "" means no reply
	}{
		{
			name:    "matching token, address from the route",
			request: "rokuagents-discover v1 " + testID + " " + testNonce,
			want:    "rokuagents v1 " + testURL + " " + testMAC,
		},
		{
			name:    "trailing newline and upper-case id are fine",
			request: "rokuagents-discover v1 " + strings.ToUpper(testID) + " " + testNonce + "\n",
			want:    "rokuagents v1 " + testURL + " " + testMAC,
		},
		{
			name:    "fixed host from --addr is advertised as given",
			host:    "192.168.1.30",
			request: "rokuagents-discover v1 " + testID + " " + testNonce,
			want: "rokuagents v1 http://192.168.1.30:8787 " +
				Sign(testToken, testNonce, "http://192.168.1.30:8787"),
		},
		{name: "another token", request: "rokuagents-discover v1 0000000000000000 " + testNonce},
		{name: "token id prefix only", request: "rokuagents-discover v1 5947d7c3 " + testNonce},
		{name: "the raw token instead of its id", request: "rokuagents-discover v1 " + testToken + " " + testNonce},
		{name: "no nonce", request: "rokuagents-discover v1 " + testID},
		{name: "short nonce", request: "rokuagents-discover v1 " + testID + " abc123"},
		{name: "nonce not hex", request: "rokuagents-discover v1 " + testID + " zzzzzzzzzzzzzzzz"},
		{name: "nonce too long", request: "rokuagents-discover v1 " + testID + " " + strings.Repeat("a", 65)},
		{name: "extra field", request: "rokuagents-discover v1 " + testID + " " + testNonce + " more"},
		{name: "other version", request: "rokuagents-discover v2 " + testID + " " + testNonce},
		{name: "a reply is not a request", request: "rokuagents v1 " + testURL + " " + testMAC},
		{name: "empty", request: ""},
		{name: "oversized", request: "rokuagents-discover v1 " + testID + " " + testNonce + strings.Repeat(" ", MaxDatagram)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := newTestResponder(tt.host)
			got, ok := r.Reply([]byte(tt.request), tv)
			if tt.want == "" {
				if ok {
					t.Fatalf("Reply = %q, want no reply", got)
				}
				return
			}
			if !ok || string(got) != tt.want {
				t.Fatalf("Reply = %q, %v; want %q", got, ok, tt.want)
			}
		})
	}
}

func TestReplyWithoutRouteSendsNothing(t *testing.T) {
	r, _ := newTestResponder("")
	r.LocalAddrFor = func(netip.Addr) (netip.Addr, error) { return netip.Addr{}, errors.New("no route") }
	if got, ok := r.Reply([]byte("rokuagents-discover v1 "+testID+" "+testNonce), tv); ok {
		t.Fatalf("Reply = %q, want no reply", got)
	}
}

func TestReplyIsRateLimited(t *testing.T) {
	r, clock := newTestResponder("")
	request := []byte("rokuagents-discover v1 " + testID + " " + testNonce)
	for i := range RepliesPerSecond {
		if _, ok := r.Reply(request, tv); !ok {
			t.Fatalf("reply %d refused, want %d per second", i+1, RepliesPerSecond)
		}
	}
	if _, ok := r.Reply(request, tv); ok {
		t.Fatal("reply over the limit was sent")
	}
	clock.now = clock.now.Add(999 * time.Millisecond)
	if _, ok := r.Reply(request, tv); ok {
		t.Fatal("reply inside the same second was sent")
	}
	clock.now = clock.now.Add(time.Millisecond)
	if _, ok := r.Reply(request, tv); !ok {
		t.Fatal("reply in the next second was refused")
	}
	// Ignored requests do not use up the budget.
	r2, _ := newTestResponder("")
	for range 3 * RepliesPerSecond {
		r2.Reply([]byte("rokuagents-discover v1 0000000000000000 "+testNonce), tv)
	}
	if _, ok := r2.Reply(request, tv); !ok {
		t.Fatal("ignored requests used up the reply budget")
	}
}

// fakeConn is a net.PacketConn fed from a channel that records what it sends.
type fakeConn struct {
	in     chan packet
	wrote  chan struct{}
	mu     sync.Mutex
	sent   []packet
	closed chan struct{}
	once   sync.Once
}

type packet struct {
	data []byte
	addr net.Addr
}

func newFakeConn() *fakeConn {
	return &fakeConn{in: make(chan packet, 8), wrote: make(chan struct{}, 8), closed: make(chan struct{})}
}

func (c *fakeConn) ReadFrom(b []byte) (int, net.Addr, error) {
	select {
	case p := <-c.in:
		return copy(b, p.data), p.addr, nil
	case <-c.closed:
		return 0, nil, net.ErrClosed
	}
}

func (c *fakeConn) WriteTo(b []byte, addr net.Addr) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sent = append(c.sent, packet{data: append([]byte(nil), b...), addr: addr})
	c.wrote <- struct{}{}
	return len(b), nil
}

func (c *fakeConn) Close() error {
	c.once.Do(func() { close(c.closed) })
	return nil
}

func (c *fakeConn) LocalAddr() net.Addr              { return &net.UDPAddr{Port: 8787} }
func (c *fakeConn) SetDeadline(time.Time) error      { return nil }
func (c *fakeConn) SetReadDeadline(time.Time) error  { return nil }
func (c *fakeConn) SetWriteDeadline(time.Time) error { return nil }

func TestServeAnswersOnlyMatchingRequestsAndStopsWithContext(t *testing.T) {
	r, _ := newTestResponder("")
	conn := newFakeConn()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- r.Serve(ctx, conn) }()

	from := net.UDPAddrFromAddrPort(tv)
	conn.in <- packet{data: []byte("hello"), addr: from}
	conn.in <- packet{data: []byte("rokuagents-discover v1 0000000000000000 " + testNonce), addr: from}
	conn.in <- packet{data: []byte("rokuagents-discover v1 " + testID + " " + testNonce), addr: from}
	conn.in <- packet{data: []byte("rokuagents-discover v1 " + testID + " " + testNonce), addr: &net.IPAddr{}}
	// A final request acts as a barrier: once it is answered, everything
	// before it has been handled.
	conn.in <- packet{data: []byte("rokuagents-discover v1 " + testID + " ffffffffffffffff"), addr: from}

	deadline := time.After(5 * time.Second)
	for i := range 2 {
		select {
		case <-conn.wrote:
		case <-deadline:
			t.Fatalf("got %d replies, want 2", i)
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Serve = %v, want nil after cancel", err)
	}

	conn.mu.Lock()
	defer conn.mu.Unlock()
	if len(conn.sent) != 2 {
		t.Fatalf("sent %d replies, want 2", len(conn.sent))
	}
	want := "rokuagents v1 " + testURL + " " + testMAC
	if got := string(conn.sent[0].data); got != want {
		t.Errorf("first reply = %q, want %q", got, want)
	}
	if got := conn.sent[0].addr.String(); got != tv.String() {
		t.Errorf("reply went to %s, want %s", got, tv)
	}
	select {
	case <-conn.closed:
	default:
		t.Error("Serve returned without closing the conn")
	}
}

func TestServeReturnsReadErrors(t *testing.T) {
	r, _ := newTestResponder("")
	conn := &failingConn{fakeConn: newFakeConn(), err: errors.New("boom")}
	if err := r.Serve(context.Background(), conn); err == nil || err.Error() != "boom" {
		t.Fatalf("Serve = %v, want boom", err)
	}
}

type failingConn struct {
	*fakeConn
	err error
}

func (c *failingConn) ReadFrom([]byte) (int, net.Addr, error) { return 0, nil, c.err }

func TestRouteLocalAddrForLoopback(t *testing.T) {
	got, err := RouteLocalAddr(netip.MustParseAddr("127.0.0.1"))
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsLoopback() {
		t.Fatalf("RouteLocalAddr(127.0.0.1) = %s, want a loopback address", got)
	}
}

func TestServeOverRealUDP(t *testing.T) {
	server, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("no UDP on loopback: %v", err)
	}
	port := server.LocalAddr().(*net.UDPAddr).Port
	r := New(testToken, "", port)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = r.Serve(ctx, server) }()

	client, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if _, err := client.WriteTo([]byte("rokuagents-discover v1 "+testID+" "+testNonce), server.LocalAddr()); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(5 * time.Second))
	buf := make([]byte, MaxDatagram)
	n, _, err := client.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	url := "http://127.0.0.1:" + strconv.Itoa(port)
	want := "rokuagents v1 " + url + " " + Sign(testToken, testNonce, url)
	if got := string(buf[:n]); got != want {
		t.Fatalf("reply = %q, want %q", got, want)
	}
}
