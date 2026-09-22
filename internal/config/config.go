// Package config loads runtime configuration from environment variables
// (optionally backed by a .env file for local development).
package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// PBXWorkerMode selects which pbx-worker client implementation to use.
type PBXWorkerMode string

const (
	PBXWorkerModeHMAC PBXWorkerMode = "hmac" // v2 PHP worker, HMAC-signed
	PBXWorkerModeJWT  PBXWorkerMode = "jwt"  // v3 Go worker, JWT-authenticated (TODO, not wired yet)
)

type Config struct {
	AppEnv   string
	HTTPPort string

	// Auth for requests coming INTO this service (from Laravel/frontend).
	// ASSUMPTION: reusing the same HMAC pattern as Laravel<->pbx-worker for
	// consistency. Confirm with the user/reference code whether this should
	// instead be JWT or a plain shared secret — see README "Open questions".
	IncomingAuthDisabled  bool // convenience switch for local dev/testing only; must be false in any real deployment
	IncomingAPIKey        string
	IncomingSecret        string
	IncomingHMACTolerance time.Duration

	// End-user authentication (browser -> this service). Separate from the
	// HMAC above, which is service-to-service only; see internal/auth.
	JWTSecret        string
	JWTTTL           time.Duration
	DevAdminPassword string
	DevAgentPassword string

	// Origins allowed to call this service from a browser (the Angular dev
	// server runs on a different port, so same-origin does not apply).
	CORSOrigins []string

	// Softphone: where the browser's SIP user agent connects, and the
	// supervisor feature codes. The PBX is the customer's own; these are
	// deployment settings, not secrets.
	PBXHost         string
	PBXWSPort       int
	PBXWSPath       string
	PBXTransport    string
	SpyMonitorCode  string
	SpyWhisperCode  string
	SpyBargeCode    string
	DevSIPExtension string
	DevSIPPassword  string

	// SIP extension password encryption at rest. See
	// internal/softphone/crypto.go. Generate with: openssl rand -base64 32
	SIPEncryptionKey string

	// qstats direct-read database.
	QstatsDSN string

	// This product's own operational database (call_logs, and eventually
	// sip_extensions) — separate from qstats. See internal/calllog.
	CallCenterDSN string

	// pbx-worker HTTP gateway.
	PBXWorkerMode     PBXWorkerMode
	PBXWorkerBaseURL  string
	PBXWorkerAPIKey   string
	PBXWorkerSecret   string
	PBXWorkerJWTToken string // TODO: v3 auth flow unconfirmed; placeholder only

	// pbx-worker's LIVE control surface (snapshots + AMI commands). Same
	// host and credentials as PBXWorker* above — confirmed real: both
	// PbxReportGateway.php and PbxWorkerGateway.php resolve the same
	// PBX_WORKER_URL/PBX_WORKER_API_KEY/PBX_WORKER_SECRET env vars in
	// heal-crm. Kept as separate config fields (not just reused directly)
	// so a deployment CAN point them at a different host later without
	// this being a surprise.
	PBXControlBaseURL string
	PBXControlAPIKey  string
	PBXControlSecret  string

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

		JWTSecret:        getenv("JWT_SECRET", ""),
		JWTTTL:           getduration("JWT_TTL_SECONDS", 8*time.Hour),
		DevAdminPassword: getenv("DEV_ADMIN_PASSWORD", ""),
		DevAgentPassword: getenv("DEV_AGENT_PASSWORD", ""),

		CORSOrigins: getlist("CORS_ORIGINS", []string{"http://localhost:4200"}),

		PBXHost:         getenv("SOFTPHONE_PBX_HOST", ""),
		PBXWSPort:       getint("SOFTPHONE_PBX_WS_PORT", 8089),
		PBXWSPath:       getenv("SOFTPHONE_PBX_WS_PATH", "/ws"),
		PBXTransport:    getenv("SOFTPHONE_PBX_TRANSPORT", "wss"),
		SpyMonitorCode:  getenv("SOFTPHONE_SPY_MONITOR", "555"),
		SpyWhisperCode:  getenv("SOFTPHONE_SPY_WHISPER", "556"),
		SpyBargeCode:    getenv("SOFTPHONE_SPY_BARGE", "557"),
		DevSIPExtension: getenv("DEV_SIP_EXTENSION", ""),
		DevSIPPassword:  getenv("DEV_SIP_PASSWORD", ""),

		SIPEncryptionKey: getenv("SIP_ENCRYPTION_KEY", ""),

		QstatsDSN:     getenv("QSTATS_DSN", ""),
		CallCenterDSN: getenv("CALLCENTER_DSN", ""),

		PBXWorkerMode:     PBXWorkerMode(getenv("PBX_WORKER_MODE", string(PBXWorkerModeHMAC))),
		PBXWorkerBaseURL:  getenv("PBX_WORKER_BASE_URL", ""),
		PBXWorkerAPIKey:   getenv("PBX_WORKER_API_KEY", ""),
		PBXWorkerSecret:   getenv("PBX_WORKER_SECRET", ""),
		PBXWorkerJWTToken: getenv("PBX_WORKER_JWT_TOKEN", ""),

		// Defaults to the same PBX_WORKER_* values when the _CONTROL_
		// variants are not set, matching the real deployment's one shared
		// credential set. Override only if a deployment ever splits them.
		PBXControlBaseURL: getenv("PBX_CONTROL_BASE_URL", getenv("PBX_WORKER_BASE_URL", "")),
		PBXControlAPIKey:  getenv("PBX_CONTROL_API_KEY", getenv("PBX_WORKER_API_KEY", "")),
		PBXControlSecret:  getenv("PBX_CONTROL_SECRET", getenv("PBX_WORKER_SECRET", "")),

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

func getint(key string, fallback int) int {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getlist(key string, fallback []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
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
