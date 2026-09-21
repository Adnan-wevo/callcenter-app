// Package config loads runtime configuration from environment variables
// (optionally backed by a .env file for local development).
package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// PBXWorkerMode selects which pbx-worker client implementation to use.
type PBXWorkerMode string

const (
	PBXWorkerModeHMAC PBXWorkerMode = "hmac" // v2 PHP worker, HMAC-signed
	PBXWorkerModeJWT   PBXWorkerMode = "jwt"  // v3 Go worker, JWT-authenticated (TODO, not wired yet)
)

type Config struct {
	AppEnv   string
	HTTPPort string

	// Auth for requests coming INTO this service (from Laravel/frontend).
	// ASSUMPTION: reusing the same HMAC pattern as Laravel<->pbx-worker for
	// consistency. Confirm with the user/reference code whether this should
	// instead be JWT or a plain shared secret — see README "Open questions".
	IncomingAuthDisabled bool // convenience switch for local dev/testing only; must be false in any real deployment
	IncomingAPIKey        string
	IncomingSecret         string
	IncomingHMACTolerance time.Duration

	// qstats direct-read database.
	QstatsDSN string

	// pbx-worker HTTP gateway.
	PBXWorkerMode    PBXWorkerMode
	PBXWorkerBaseURL string
	PBXWorkerAPIKey  string
	PBXWorkerSecret  string
	PBXWorkerJWTToken string // TODO: v3 auth flow unconfirmed; placeholder only

	// Outbound webhook back to Laravel (ad-hoc callback attempts).
	LaravelCallbackBaseURL string
	LaravelCallbackPath    string // TODO: exact route unconfirmed, see gateway/laravel
	LaravelCallbackAPIKey  string
	LaravelCallbackSecret  string
}

// Load reads configuration from the environment. If a .env file is present
// in the working directory it is loaded first (existing env vars win).
func Load() Config {
	_ = godotenv.Load() // ok if missing (e.g. running inside docker-compose with env: block)

	return Config{
		AppEnv:   getenv("APP_ENV", "local"),
		HTTPPort: getenv("HTTP_PORT", "8080"),

		IncomingAuthDisabled:  getbool("INCOMING_AUTH_DISABLED", false),
		IncomingAPIKey:        getenv("INCOMING_HMAC_API_KEY", ""),
		IncomingSecret:        getenv("INCOMING_HMAC_SECRET", ""),
		IncomingHMACTolerance: getduration("INCOMING_HMAC_TOLERANCE_SECONDS", 5*time.Minute),

		QstatsDSN: getenv("QSTATS_DSN", ""),

		PBXWorkerMode:     PBXWorkerMode(getenv("PBX_WORKER_MODE", string(PBXWorkerModeHMAC))),
		PBXWorkerBaseURL:  getenv("PBX_WORKER_BASE_URL", ""),
		PBXWorkerAPIKey:   getenv("PBX_WORKER_API_KEY", ""),
		PBXWorkerSecret:   getenv("PBX_WORKER_SECRET", ""),
		PBXWorkerJWTToken: getenv("PBX_WORKER_JWT_TOKEN", ""),

		LaravelCallbackBaseURL: getenv("LARAVEL_CALLBACK_BASE_URL", ""),
		// TODO: confirm real path against Laravel reference code.
		LaravelCallbackPath:   getenv("LARAVEL_CALLBACK_PATH", "/api/internal/call-center/callback-attempts"),
		LaravelCallbackAPIKey: getenv("LARAVEL_CALLBACK_API_KEY", ""),
		LaravelCallbackSecret: getenv("LARAVEL_CALLBACK_SECRET", ""),
	}
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getbool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getduration(key string, fallbackSeconds time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallbackSeconds
	}
	secs, err := strconv.Atoi(v)
	if err != nil {
		return fallbackSeconds
	}
	return time.Duration(secs) * time.Second
}
