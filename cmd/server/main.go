package main

import (
	"log"

	"callcenter-service/internal/config"
	"callcenter-service/internal/db/qstats"
	"callcenter-service/internal/gateway/laravel"
	"callcenter-service/internal/gateway/pbxworker"
	"callcenter-service/internal/handlers"
	"callcenter-service/internal/security/hmacsig"

	"net/http"
	"time"
)

func main() {
	cfg := config.Load()

	db, err := qstats.Connect(cfg.QstatsDSN)
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

	h := handlers.New(qstatsRepo, pbxClient, callbackClient)

	incomingCreds := hmacsig.Credentials{APIKey: cfg.IncomingAPIKey, Secret: cfg.IncomingSecret}
	router := handlers.NewRouter(h, incomingCreds, cfg.IncomingHMACTolerance, cfg.IncomingAuthDisabled)

	if cfg.IncomingAuthDisabled {
		log.Printf("WARNING: INCOMING_AUTH_DISABLED=true — inbound HMAC verification is OFF (local/dev only)")
	}

	addr := ":" + cfg.HTTPPort
	log.Printf("callcenter-service listening on %s (env=%s, pbx-worker mode=%s)", addr, cfg.AppEnv, cfg.PBXWorkerMode)
	if err := router.Run(addr); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
