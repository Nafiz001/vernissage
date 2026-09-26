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
	// Cloudinary, a cloud name, hands picture resizing to Cloudinary's
	// fetch CDN instead of this server: /img redirects there, and deep zoom
	// falls back to one large picture. For hosts with little CPU and no disk.
	Cloudinary string
	// Secret signs live-room tickets. Empty means a random one per process.
	Secret string
	// ProxyHops is how many proxies in front of the server append to
	// X-Forwarded-For: 1 behind Caddy, 2 behind Vercel and Render.
	ProxyHops int
	// PixelBudget caps the pixels being decoded at once.
	PixelBudget int64
}

func Load() Config {
	return Config{
		Addr:        env("VERNISSAGE_ADDR", addrFromPort()),
		DatabaseURL: env("DATABASE_URL", "postgres://vernissage:vernissage@127.0.0.1:5491/vernissage?sslmode=disable"),
		CacheDir:    env("VERNISSAGE_CACHE", "data/cache"),
		CacheBudget: int64(envInt("VERNISSAGE_CACHE_MB", 2048)) << 20,
		Origins:     strings.Split(env("VERNISSAGE_ORIGINS", "http://localhost:3790,http://127.0.0.1:3790"), ","),
		Secure:      env("VERNISSAGE_SECURE_COOKIES", "false") == "true",
		Workers:     envInt("VERNISSAGE_WORKERS", 6),
		SyncEvery:   envDuration("VERNISSAGE_SYNC_EVERY", 0),
		UserAgent:   env("VERNISSAGE_USER_AGENT", "Vernissage/1.0 (+https://github.com/Nafiz001/vernissage)"),
		Cloudinary:  env("VERNISSAGE_CLOUDINARY", ""),
		Secret:      env("VERNISSAGE_SECRET", ""),
		ProxyHops:   max(1, envInt("VERNISSAGE_PROXY_HOPS", 1)),
		PixelBudget: int64(max(4, envInt("VERNISSAGE_PIXEL_BUDGET_MP", 64))) << 20,
	}
}

// addrFromPort listens where a platform like Render says to, through
// PORT, or on localhost for development.
func addrFromPort() string {
	if p := os.Getenv("PORT"); p != "" {
		return "0.0.0.0:" + p
	}
	return "127.0.0.1:8790"
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
