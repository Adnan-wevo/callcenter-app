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
// product or the softphone cannot register at all.
//
// # Credentials
//
// A SIP password is a live credential — anyone holding it can place calls
// billed to the customer. It is therefore:
//
//   - served only over an authenticated route, to the agent it belongs to
//   - never logged
//   - never included in any list/index response, only in this one bootstrap
//
// Storing it recoverably is unavoidable: SIP digest auth needs the plaintext
// (or the HA1 hash) to compute a response, so it cannot be one-way hashed
// the way a login password is. That is a property of SIP, not a shortcut —
// but it does mean this store deserves encryption at rest before the product
// ships. See the TODO below.
package softphone

import (
	"errors"
	"strings"
	"sync"
)

var ErrNoExtension = errors.New("softphone: no SIP extension assigned to this user")

// Extension is one SIP account an agent may register as.
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

	// The extension this session registers as: the agent's default.
	Extension   string   `json:"extension"`
	DisplayName string   `json:"display_name"`
	Password    string   `json:"password"`
	Queues      []string `json:"queues"`

	IsSupervisor bool            `json:"is_supervisor"`
	Supervisor   SupervisorCodes `json:"supervisor"`

	// Every extension this agent may register as, so a supervisor with more
	// than one can switch without signing out. Passwords are omitted.
	Extensions []Extension `json:"extensions"`
}

// Store maps a user to the SIP extensions they may register as.
//
// TODO: this is in-memory and seeded at startup, which matches the rest of
// the local store (see internal/auth). Before the product ships it needs a
// real backing table with the passwords ENCRYPTED AT REST — see the package
// doc for why hashing is not an option.
type Store struct {
	mu     sync.RWMutex
	byUser map[string][]Extension
}

func NewStore() *Store {
	return &Store{byUser: make(map[string][]Extension)}
}

// Assign replaces the extensions available to a user.
func (s *Store) Assign(userID string, extensions []Extension) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byUser[userID] = extensions
}

// ForUser returns the user's extensions, default first.
func (s *Store) ForUser(userID string) ([]Extension, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ext, ok := s.byUser[userID]
	return ext, ok && len(ext) > 0
}

// Service builds the bootstrap payload.
type Service struct {
	store      *Store
	pbx        PBX
	supervisor SupervisorCodes
}

func NewService(store *Store, pbx PBX, supervisor SupervisorCodes) *Service {
	return &Service{store: store, pbx: pbx, supervisor: supervisor}
}

// BootstrapFor assembles the payload for one user.
func (s *Service) BootstrapFor(userID string) (*Bootstrap, error) {
	extensions, ok := s.store.ForUser(userID)
	if !ok {
		return nil, ErrNoExtension
	}

	// Default first, falling back to the first row: an agent with extensions
	// but none marked default should still get a phone rather than an error.
	primary := extensions[0]
	for _, e := range extensions {
		if e.IsDefault {
			primary = e
			break
		}
	}

	// The listed extensions carry no passwords — only the one being
	// registered is disclosed, and only to its owner.
	listed := make([]Extension, 0, len(extensions))
	for _, e := range extensions {
		e.Password = ""
		if e.Queues == nil {
			e.Queues = []string{}
		}
		listed = append(listed, e)
	}

	queues := primary.Queues
	if queues == nil {
		queues = []string{}
	}

	return &Bootstrap{
		Server:       s.pbx.Server,
		Port:         s.pbx.Port,
		WSPath:       s.pbx.WSPath,
		Transport:    s.pbx.Transport,
		Extension:    primary.Extension,
		DisplayName:  displayName(primary),
		Password:     primary.Password,
		Queues:       queues,
		IsSupervisor: primary.IsSupervisor,
		Supervisor:   s.supervisor,
		Extensions:   listed,
	}, nil
}

func displayName(e Extension) string {
	if name := strings.TrimSpace(e.DisplayName); name != "" {
		return name
	}
	return e.Extension
}
