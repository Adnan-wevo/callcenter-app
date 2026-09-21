package main

import (
	"log"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"

	"callcenter-service/internal/auth"
	"callcenter-service/internal/config"
	"callcenter-service/internal/db/qstats"
	"callcenter-service/internal/gateway/laravel"
	"callcenter-service/internal/gateway/pbxworker"
	"callcenter-service/internal/handlers"
	"callcenter-service/internal/security/hmacsig"
)

func main() {
	cfg := config.Load()

	if cfg.JWTSecret == "" {
		// Refusing to start is deliberate. A generated-at-startup fallback
		// would work perfectly in development and then silently invalidate
		// every session on each deploy — and two replicas would reject each
		// other's tokens.
		log.Fatal("JWT_SECRET is not set — refusing to start without a signing key")
	}
	if cfg.DevAdminPassword == "" || cfg.DevAgentPassword == "" {
		log.Fatal("DEV_ADMIN_PASSWORD and DEV_AGENT_PASSWORD must be set to seed the local user store")
	}

	db, err := connectWithRetry(cfg.QstatsDSN, 30, 2*time.Second)
	if err != nil {
		log.Fatalf("qstats connect: %v", err)
	}
	defer db.Close()
	qstatsRepo := qstats.NewRepository(db)

	pbxClient, err := pbxworker.New(cfg)
	if err != nil {
		log.Fatalf("pbxworker client: %v", err)
	}

	callbackClient := laravel.NewHMACCallbackClient(
		&http.Client{Timeout: 15 * time.Second},
		cfg.LaravelCallbackBaseURL,
		cfg.LaravelCallbackPath,
		hmacsig.Credentials{APIKey: cfg.LaravelCallbackAPIKey, Secret: cfg.LaravelCallbackSecret},
	)

	// Hashing the seeded passwords is deliberately expensive (PBKDF2 at
	// 600k iterations), so this costs about a second at boot.
	userStore, err := auth.NewDevStore(cfg.DevAdminPassword, cfg.DevAgentPassword)
	if err != nil {
		log.Fatalf("seed user store: %v", err)
	}
	authSvc := auth.NewService(userStore, cfg.JWTSecret, cfg.JWTTTL)

	h := handlers.New(qstatsRepo, pbxClient, callbackClient, authSvc)
	router := handlers.NewRouter(h, authSvc, cfg.CORSOrigins)

	addr := ":" + cfg.HTTPPort
	log.Printf("callcenter-service listening on %s (env=%s, pbx-worker mode=%s)", addr, cfg.AppEnv, cfg.PBXWorkerMode)
	log.Printf("CORS origins allowed: %v", cfg.CORSOrigins)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// connectWithRetry waits for the database to accept connections instead of
// dying on the first refusal.
//
// A compose healthcheck is not enough on its own: MySQL's usual `mysqladmin
// ping -h localhost` succeeds over the unix socket while the TCP listener is
// still coming up, so a dependent service can be released to start and still
// be refused. Retrying here is also the behaviour wanted in a real
// deployment, where a database can be restarted underneath a running estate.
func connectWithRetry(dsn string, attempts int, wait time.Duration) (*sqlx.DB, error) {
	var lastErr error
	for i := 1; i <= attempts; i++ {
		db, err := qstats.Connect(dsn)
		if err == nil {
			return db, nil
		}
		lastErr = err
		log.Printf("qstats not ready (attempt %d/%d): %v", i, attempts, err)
		time.Sleep(wait)
	}
	return nil, lastErr
}
