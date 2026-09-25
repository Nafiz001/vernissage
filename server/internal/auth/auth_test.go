package auth

import (
	"strings"
	"testing"
)

func TestPasswords(t *testing.T) {
	h, err := HashPassword("wheat field with cypresses")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("hash %q isn't PHC argon2id", h)
	}
	if !CheckPassword(h, "wheat field with cypresses") {
		t.Error("right password rejected")
	}
	if CheckPassword(h, "wheat field with cypress") {
		t.Error("wrong password accepted")
	}
	if CheckPassword("", "not a real password") {
		t.Error("the dummy hash let someone in")
	}
	h2, _ := HashPassword("wheat field with cypresses")
	if h == h2 {
		t.Error("two hashes of one password are equal: salt missing")
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter(60, 3)
	for i := range 3 {
		if !l.Allow("a") {
			t.Fatalf("request %d of a 3-burst refused", i+1)
		}
	}
	if l.Allow("a") {
		t.Error("fourth request in a burst of 3 allowed")
	}
	if !l.Allow("b") {
		t.Error("a different key was limited")
	}
}
