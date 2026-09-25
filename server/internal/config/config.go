// Package config reads the server's settings from the environment.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	// Addr is where the HTTP server listens.
	Addr string
	// DatabaseURL is a PostgreSQL connection string.
	DatabaseURL string
	// CacheDir holds resized images, deep-zoom tiles and posters.
	CacheDir string
	// CacheBudget is how many bytes of derived images to keep before the
	// least recently used are evicted. Masters are never evicted.
	CacheBudget int64
	// Origins are the web origins allowed to send cookies-bearing writes and
	// open WebSockets. The first one is used to build absolute links.
	Origins []string
	// Secure marks cookies Secure; on for anything served over HTTPS.
	Secure bool
	// Workers is how many background jobs run at once.
	Workers int
	// SyncEvery re-reads the museums' collections on this interval; 0 turns
	// the periodic sync off.
	SyncEvery time.Duration
	// UserAgent identifies the server to the museums' APIs.
	UserAgent string
}

func Load() Config {
	return Config{
		Addr:        env("VERNISSAGE_ADDR", "127.0.0.1:8790"),
		DatabaseURL: env("DATABASE_URL", "postgres://vernissage:vernissage@127.0.0.1:5491/vernissage?sslmode=disable"),
		CacheDir:    env("VERNISSAGE_CACHE", "data/cache"),
		CacheBudget: int64(envInt("VERNISSAGE_CACHE_MB", 2048)) << 20,
		Origins:     strings.Split(env("VERNISSAGE_ORIGINS", "http://localhost:3790,http://127.0.0.1:3790"), ","),
		Secure:      env("VERNISSAGE_SECURE_COOKIES", "false") == "true",
		Workers:     envInt("VERNISSAGE_WORKERS", 6),
		SyncEvery:   envDuration("VERNISSAGE_SYNC_EVERY", 0),
		UserAgent:   env("VERNISSAGE_USER_AGENT", "Vernissage/1.0 (+https://github.com/nafizahmed/vernissage)"),
	}
}

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v, err := strconv.Atoi(env(key, "")); err == nil {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v, err := time.ParseDuration(env(key, "")); err == nil {
		return v
	}
	return def
}
