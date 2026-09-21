package pbxworker

import (
	"fmt"
	"net/http"
	"time"

	"callcenter-service/internal/config"
	"callcenter-service/internal/security/hmacsig"
)

// New picks the ReportsClient implementation to use based on cfg.PBXWorkerMode,
// so callers (handlers) never need to know which worker version is live.
func New(cfg config.Config) (ReportsClient, error) {
	httpClient := &http.Client{Timeout: 15 * time.Second}

	switch cfg.PBXWorkerMode {
	case config.PBXWorkerModeHMAC, "":
		return NewHMACClient(httpClient, cfg.PBXWorkerBaseURL, hmacsig.Credentials{
			APIKey: cfg.PBXWorkerAPIKey,
			Secret: cfg.PBXWorkerSecret,
		}), nil
	case config.PBXWorkerModeJWT:
		return NewJWTClient(httpClient, cfg.PBXWorkerBaseURL, cfg.PBXWorkerJWTToken), nil
	default:
		return nil, fmt.Errorf("pbxworker: unknown PBX_WORKER_MODE %q", cfg.PBXWorkerMode)
	}
}
