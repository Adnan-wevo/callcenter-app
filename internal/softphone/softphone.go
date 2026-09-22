// Package softphone supplies the browser softphone with what it needs to
// register against the PBX.
//
// # What this is, and what it is not
//
// The actual calling is done IN THE BROWSER: JsSIP registers a SIP user
// agent over a secure WebSocket to Asterisk and WebRTC carries the audio.
// This package never touches a call. It answers one question — "what
// credentials and PBX coordinates does this signed-in agent use?" — which is
// exactly what heal-crm's Livewire component does today before handing off
// to the same JsSIP library.
//
// # Why the product owns this
//
// In heal-crm these rows live in the monolith's own tables. This service is
// being packaged as a standalone product, so a customer deployment has no
// heal-crm database to read: the extension directory has to belong to the
// product or the softphone cannot register at all. See repository.go for
// the real, MySQL-backed store (migrations/callcenter/002_sip_extensions.sql),
// which replaced an in-memory placeholder this package started with.
//
// # Credentials
//
// A SIP password is a live credential — anyone holding it can place calls
// billed to the customer. It is therefore:
//
//   - served only over an authenticated route, to the agent it belongs to
//   - never logged
//   - never included in any list/index response, only in this one bootstrap
//   - encrypted at rest (crypto.go, AES-256-GCM) — recoverably, not hashed:
//     SIP digest auth needs the plaintext, unlike a login password.
package softphone

import (
	"context"
	"errors"
	"strings"
)

var (
	ErrNoExtension = errors.New("softphone: no SIP extension assigned to this user")
)

// Extension is one SIP account an agent may register as.
//
// The real schema (see repository.go) allows AT MOST ONE per user — the
// slice-shaped API here (ExtensionFor/BootstrapFor's Extensions field)
// exists for compatibility with a client written against heal-crm's own
// aspirational "several extensions, switch between them" design, which its
// own database does not actually allow (see the migration's own doc
// comment for the discrepancy). It will always hold zero or one item.
type Extension struct {
	Extension    string   `json:"extension"`
	DisplayName  string   `json:"display_name"`
	Password     string   `json:"-"` // never serialised in a list
	IsDefault    bool     `json:"is_default"`
	IsSupervisor bool     `json:"is_supervisor"`
	Queues       []string `json:"queues"`
}

// PBX is where the browser's SIP user agent connects.
type PBX struct {
	Server    string `json:"server"`
	Port      int    `json:"port"`
	WSPath    string `json:"ws_path"`
	Transport string `json:"transport"`
}

// SupervisorCodes are the feature codes for listening in on an agent. They
// are dial strings, not permissions — holding the code does not grant the
// right to use it; the PBX dialplan enforces that.
type SupervisorCodes struct {
	SpyMonitor string `json:"spy_monitor"`
	SpyWhisper string `json:"spy_whisper"`
	SpyBarge   string `json:"spy_barge"`
}

// Bootstrap is everything the browser needs to register and place calls.
// Field names mirror heal-crm's own payload so the port is a like-for-like
// swap rather than a new contract to learn.
type Bootstrap struct {
	Server    string `json:"server"`
	Port      int    `json:"port"`
	WSPath    string `json:"ws_path"`
	Transport string `json:"transport"`

	Extension   string   `json:"extension"`
	DisplayName string   `json:"display_name"`
	Password    string   `json:"password"`
	Queues      []string `json:"queues"`

	IsSupervisor bool            `json:"is_supervisor"`
	Supervisor   SupervisorCodes `json:"supervisor"`

	// See Extension's own doc comment for why this is a list of at most one.
	Extensions []Extension `json:"extensions"`
}

// Service builds the bootstrap payload from the real SIP extension
// directory.
type Service struct {
	repo       *Repository
	pbx        PBX
	supervisor SupervisorCodes
}

func NewService(repo *Repository, pbx PBX, supervisor SupervisorCodes) *Service {
	return &Service{repo: repo, pbx: pbx, supervisor: supervisor}
}

// ExtensionFor returns just the extension NUMBER (never the password) for
// a user's SIP account, for callers that need to stamp a call log row with
// "who handled this" without needing the full bootstrap payload.
func (s *Service) ExtensionFor(ctx context.Context, userID string) (string, bool) {
	ext, ok, err := s.repo.ForUser(ctx, userID)
	if err != nil || !ok {
		return "", false
	}
	return ext.Extension, true
}

// BootstrapFor assembles the payload for one user.
func (s *Service) BootstrapFor(ctx context.Context, userID string) (*Bootstrap, error) {
	ext, ok, err := s.repo.ForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrNoExtension
	}

	listed := ext
	listed.Password = ""
	if listed.Queues == nil {
		listed.Queues = []string{}
	}

	queues := ext.Queues
	if queues == nil {
		queues = []string{}
	}

	return &Bootstrap{
		Server:       s.pbx.Server,
		Port:         s.pbx.Port,
		WSPath:       s.pbx.WSPath,
		Transport:    s.pbx.Transport,
		Extension:    ext.Extension,
		DisplayName:  displayName(ext),
		Password:     ext.Password,
		Queues:       queues,
		IsSupervisor: ext.IsSupervisor,
		Supervisor:   s.supervisor,
		Extensions:   []Extension{listed},
	}, nil
}

func displayName(e Extension) string {
	if name := strings.TrimSpace(e.DisplayName); name != "" {
		return name
	}
	return e.Extension
}
