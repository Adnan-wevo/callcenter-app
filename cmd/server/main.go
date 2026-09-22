package main

import (
	"log"
	"net/http"
	"time"

	"github.com/jmoiron/sqlx"

	"callcenter-service/internal/auth"
	"callcenter-service/internal/calllog"
	"callcenter-service/internal/config"
	"callcenter-service/internal/db/qstats"
	"callcenter-service/internal/gateway/laravel"
	"callcenter-service/internal/gateway/pbxcontrol"
	"callcenter-service/internal/gateway/pbxworker"
	"callcenter-service/internal/handlers"
	"callcenter-service/internal/security/hmacsig"
	"callcenter-service/internal/softphone"
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

	db, err := connectWithRetry(30, 2*time.Second, func() (*sqlx.DB, error) { return qstats.Connect(cfg.QstatsDSN) })
	if err != nil {
		log.Fatalf("qstats connect: %v", err)
	}
	defer db.Close()
	qstatsRepo := qstats.NewRepository(db)

	// This product's own operational database (call_logs). See
	// internal/calllog's own doc comment for why this is separate from
	// qstats above.
	ccDB, err := connectWithRetry(30, 2*time.Second, func() (*sqlx.DB, error) { return calllog.Connect(cfg.CallCenterDSN) })
	if err != nil {
		log.Fatalf("callcenter db connect: %v", err)
	}
	defer ccDB.Close()
	callLogRepo := calllog.NewRepository(ccDB)

	pbxClient, err := pbxworker.New(cfg)
	if err != nil {
		log.Fatalf("pbxworker client: %v", err)
	}

	// pbx-worker's LIVE control surface (snapshots + AMI commands) —
	// distinct from pbxClient above, which is the HISTORICAL reporting
	// client. See internal/gateway/pbxcontrol's own doc comment.
	pbxControlClient := pbxcontrol.NewClient(
		&http.Client{Timeout: 15 * time.Second},
		cfg.PBXControlBaseURL,
		hmacsig.Credentials{APIKey: cfg.PBXControlAPIKey, Secret: cfg.PBXControlSecret},
	)

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

	// The SIP extension directory. Seeded for local use, for the same reason
	// the user store is: the product owns this data (a customer deployment has
	// no heal-crm to read it from), but the real backing table is not built
	// yet. See internal/softphone.
	extensionStore := softphone.NewStore()
	if cfg.DevSIPExtension != "" {
		for _, id := range []string{
			"00000000-0000-0000-0000-000000000001",
			"00000000-0000-0000-0000-000000000002",
		} {
			extensionStore.Assign(id, []softphone.Extension{{
				Extension:   cfg.DevSIPExtension,
				DisplayName: cfg.DevSIPExtension,
				Password:    cfg.DevSIPPassword,
				IsDefault:   true,
				Queues:      []string{},
			}})
		}
		log.Printf("softphone: seeded dev extension %s", cfg.DevSIPExtension)
	} else {
		log.Printf("softphone: no DEV_SIP_EXTENSION set — the phone will report no extension assigned")
	}

	softphoneSvc := softphone.NewService(
		extensionStore,
		softphone.PBX{
			Server:    cfg.PBXHost,
			Port:      cfg.PBXWSPort,
			WSPath:    cfg.PBXWSPath,
			Transport: cfg.PBXTransport,
		},
		softphone.SupervisorCodes{
			SpyMonitor: cfg.SpyMonitorCode,
			SpyWhisper: cfg.SpyWhisperCode,
			SpyBarge:   cfg.SpyBargeCode,
		},
	)

	h := handlers.New(qstatsRepo, pbxClient, callbackClient, authSvc, softphoneSvc, callLogRepo, pbxControlClient)
	router := handlers.NewRouter(h, authSvc, cfg.CORSOrigins)

	addr := ":" + cfg.HTTPPort
	log.Printf("callcenter-service listening on %s (env=%s, pbx-worker mode=%s)", addr, cfg.AppEnv, cfg.PBXWorkerMode)
	log.Printf("CORS origins allowed: %v", cfg.CORSOrigins)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// connectWithRetry waits for a database to accept connections instead of
// dying on the first refusal.
//
// A compose healthcheck is not enough on its own: MySQL's usual `mysqladmin
// ping -h localhost` succeeds over the unix socket while the TCP listener is
// still coming up, so a dependent service can be released to start and still
// be refused. Retrying here is also the behaviour wanted in a real
// deployment, where a database can be restarted underneath a running estate.
// connect is qstats.Connect or calllog.Connect, called until it succeeds or
// attempts runs out.
func connectWithRetry(attempts int, wait time.Duration, connect func() (*sqlx.DB, error)) (*sqlx.DB, error) {
	var lastErr error
	for i := 1; i <= attempts; i++ {
		db, err := connect()
		if err == nil {
			return db, nil
		}
		lastErr = err
		log.Printf("database not ready (attempt %d/%d): %v", i, attempts, err)
		time.Sleep(wait)
	}
	return nil, lastErr
}
