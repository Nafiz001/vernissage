// Package auth hashes passwords, mints session tokens and rate-limits.
package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

// Argon2id at OWASP's recommended minimum: 19 MiB, two passes, one lane.
const (
	memoryKiB = 19 * 1024
	passes    = 2
	lanes     = 1
	keyLen    = 32
	saltLen   = 16
)

// hashing bounds concurrent password hashes, each of which takes 19 MiB.
var hashing = make(chan struct{}, 4)

// HashPassword returns a PHC-format argon2id hash.
func HashPassword(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hashing <- struct{}{}
	key := argon2.IDKey([]byte(password), salt, passes, memoryKiB, lanes, keyLen)
	<-hashing
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoryKiB, passes, lanes, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// dummy is checked against when there's no such user, so a wrong email
// takes as long as a wrong password and doesn't reveal who has an account.
var dummy, _ = HashPassword("not a real password")

// CheckPassword reports whether password matches hash, in constant time.
// An empty hash checks against a dummy and always fails.
func CheckPassword(hash, password string) bool {
	real := hash != ""
	if !real {
		hash = dummy
	}
	var version, m, t int
	var p uint8
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false
	}
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(parts[4])
	want, err2 := base64.RawStdEncoding.DecodeString(parts[5])
	if err1 != nil || err2 != nil {
		return false
	}
	hashing <- struct{}{}
	got := argon2.IDKey([]byte(password), salt, uint32(t), uint32(m), p, uint32(len(want)))
	<-hashing
	return subtle.ConstantTimeCompare(got, want) == 1 && real
}

// NewToken is 256 random bits, URL-safe.
func NewToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// ValidatePassword explains what's wrong with a new password, if anything.
func ValidatePassword(p string) error {
	switch {
	case len(p) < 8:
		return errors.New("Use at least 8 characters for your password.")
	case len(p) > 200:
		return errors.New("Use at most 200 characters for your password.")
	}
	return nil
}

// Limiter is a token bucket per key: rate tokens a second, up to burst.
type Limiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	buckets map[string]*bucket
	swept   time.Time
}

type bucket struct {
	tokens float64
	at     time.Time
}

func NewLimiter(perMinute, burst int) *Limiter {
	return &Limiter{rate: float64(perMinute) / 60, burst: float64(burst), buckets: map[string]*bucket{}, swept: time.Now()}
}

// Allow takes a token for key if one is available.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	if now.Sub(l.swept) > 10*time.Minute {
		// Forget full buckets; they'd be refilled anyway.
		for k, b := range l.buckets {
			if b.tokens+now.Sub(b.at).Seconds()*l.rate >= l.burst {
				delete(l.buckets, k)
			}
		}
		l.swept = now
	}
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.burst, at: now}
		l.buckets[key] = b
	}
	b.tokens = min(l.burst, b.tokens+now.Sub(b.at).Seconds()*l.rate)
	b.at = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
