package api

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTickets(t *testing.T) {
	a := newTickets("one secret")
	tk := a.issue("visitor-123", 42)
	if v, uid, ok := a.check(tk); !ok || v != "visitor-123" || uid != 42 {
		t.Fatalf("round trip: %q %d %v", v, uid, ok)
	}
	if _, _, ok := newTickets("another secret").check(tk); ok {
		t.Error("a ticket signed with another key was accepted")
	}
	body, sig, _ := strings.Cut(tk, ".")
	if _, _, ok := a.check(body[:len(body)-2] + "xx." + sig); ok {
		t.Error("a tampered ticket was accepted")
	}
	if _, _, ok := a.check("nonsense"); ok {
		t.Error("nonsense accepted")
	}
}

func TestForwardedFor(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.5:4000"
	if got := forwardedFor(r, 1); got != "10.0.0.5" {
		t.Errorf("no header: %s", got)
	}
	// The client forged the first entry; Vercel added the browser's
	// address, Render added Vercel's.
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 203.0.113.9, 76.76.21.1")
	if got := forwardedFor(r, 2); got != "203.0.113.9" {
		t.Errorf("two hops: %s", got)
	}
	if got := forwardedFor(r, 1); got != "76.76.21.1" {
		t.Errorf("one hop: %s", got)
	}
	if got := forwardedFor(r, 9); got != "6.6.6.6" {
		t.Errorf("more hops than entries: %s", got)
	}
}
