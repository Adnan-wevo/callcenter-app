// Package auth owns end-user authentication and authority for this service.
//
// # Why this exists at all
//
// Before this, the service had only HMAC — which is service-to-service
// (Laravel <-> this service <-> pbx-worker). A browser cannot sign requests
// that way and must never hold the shared secret, so there was no way for a
// person to use this service at all. See docs/extraction-plan.md §4.2.
//
// # The model, and why the token carries no authority
//
// Login issues a JWT carrying IDENTITY ONLY — subject and lifetime. What the
// caller may do is resolved separately, on every request, from this package's
// store. A token carrying permissions keeps granting them after a grant is
// revoked, until it expires; a token carrying only a subject cannot.
//
// # Scope: this is the LOCAL store
//
// Users are seeded in-process (see NewDevStore) so the service can be run and
// used end to end without the monolith. Where the real user directory and the
// real permission catalogue should come from — Laravel issuing the token, or
// this service reading the CRM database — is open decision D2 in
// docs/extraction-plan.md and is NOT settled by this file. The Store
// interface is the seam that decision plugs into.
package auth

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"callcenter-service/internal/security/jwtsig"
	"callcenter-service/internal/security/pwhash"
)

// The CallCenter permission vocabulary, taken from the real Laravel policies
// (Modules/CallCenter/app/Livewire/*/Index.php `$this->authorize(...)`).
// Keeping the identifiers identical is what will let authority eventually be
// served by Laravel without the frontend changing.
const (
	PermDashboard          = "call-center.dashboard.index"
	PermAnsweredIndex      = "call-center.answered-calls.index"
	PermAnsweredExport     = "call-center.answered-calls.export"
	PermUnansweredIndex    = "call-center.unanswered-calls.index"
	PermUnansweredExport   = "call-center.unanswered-calls.export"
	PermUnansweredCallback = "call-center.unanswered-calls.callback"
	PermCallSearchIndex    = "call-center.call-search.index"
	PermCallSearchExport   = "call-center.call-search.export"
	PermAgentPerformance   = "call-center.agent-performance.index"
	PermDistribution       = "call-center.distribution.index"

	// PermSoftphoneSupervise gates redirect/pickup/spy — the same name and
	// the same boundary as heal-crm's own
	// `Route::middleware('can:softphone.supervise')` in
	// Modules/SoftPhone/routes/api.php. Queue login/logout/pause are NOT
	// behind this permission there either — any agent controls their own
	// queue membership, only supervisor actions over OTHER agents' calls
	// are gated.
	PermSoftphoneSupervise = "softphone.supervise"

	// SIP extension admin — same four names as
	// Modules/SoftPhone/app/Livewire/SipExtensions/Index.php's own
	// $this->authorize(...) calls.
	PermSIPExtensionsIndex   = "call-center.sip-extensions.index"
	PermSIPExtensionsStore   = "call-center.sip-extensions.store"
	PermSIPExtensionsUpdate  = "call-center.sip-extensions.update"
	PermSIPExtensionsDestroy = "call-center.sip-extensions.destroy"

	// Row-level security admin — same four names as
	// Modules/CallCenter/app/Livewire/UserFilters/Index.php's own
	// $this->authorize(...) calls. "edit" gates opening the form (a pure UI
	// concern there — heal-crm never checks it server-side beyond that);
	// "update"/"destroy" are the ones actually enforced on save/clear here.
	PermUserFiltersIndex   = "call-center.user-filters.index"
	PermUserFiltersEdit    = "call-center.user-filters.edit"
	PermUserFiltersUpdate  = "call-center.user-filters.update"
	PermUserFiltersDestroy = "call-center.user-filters.destroy"

	// Same two names as RealtimeMonitor/Index.php's own $this->authorize
	// calls. "index" gates viewing the board; "actions" gates the
	// supervisor controls on it (pause/unpause/logout an agent, redirect a
	// waiting call) — enforced server-side by PermSoftphoneSupervise on
	// those specific endpoints regardless, this is the UI-level match.
	PermRealtimeMonitorIndex   = "call-center.realtime-monitor.index"
	PermRealtimeMonitorActions = "call-center.realtime-monitor.actions"

	// Same two names as Modules/CallCenter/app/Livewire/Settings/Index.php's
	// own $this->authorize calls — no store/destroy, this is a fixed set of
	// rows (see migrations/callcenter/004_call_center_settings.sql), not a
	// free CRUD.
	PermSettingsIndex  = "call-center.settings.index"
	PermSettingsUpdate = "call-center.settings.update"

	// Same four names as SIP Extensions/User Filters use for their own
	// CRUD — see Modules/CallCenter/app/Livewire/QueueGroups/*.php (which
	// splits create/store into two permissions; this service collapses
	// that the same way it already did for SIP Extensions).
	PermQueueGroupsIndex   = "call-center.queue-groups.index"
	PermQueueGroupsStore   = "call-center.queue-groups.store"
	PermQueueGroupsUpdate  = "call-center.queue-groups.update"
	PermQueueGroupsDestroy = "call-center.queue-groups.destroy"

	// Same four-name shape as Queue Groups — DEFINITIONS only, see
	// internal/scheduledreports' own doc comment: nothing executes these
	// rows yet, there is no email sender or scheduler process.
	PermScheduledReportsIndex   = "call-center.scheduled-reports.index"
	PermScheduledReportsStore   = "call-center.scheduled-reports.store"
	PermScheduledReportsUpdate  = "call-center.scheduled-reports.update"
	PermScheduledReportsDestroy = "call-center.scheduled-reports.destroy"

	// User administration. No heal-crm counterpart to borrow names from —
	// its user directory lives in the Laravel monolith, not the CallCenter
	// module — so these follow the same shape as everything above. Backed by
	// v3's /secure/acl/users, since that is the directory this service now
	// signs people in against (see internal/auth/external.go).
	PermUsersIndex   = "call-center.users.index"
	PermUsersStore   = "call-center.users.store"
	PermUsersUpdate  = "call-center.users.update"
	PermUsersDestroy = "call-center.users.destroy"
)

// AllPermissions is the catalogue, in the order a UI would list it.
var AllPermissions = []string{
	PermDashboard,
	PermAnsweredIndex,
	PermAnsweredExport,
	PermUnansweredIndex,
	PermUnansweredExport,
	PermUnansweredCallback,
	PermCallSearchIndex,
	PermCallSearchExport,
	PermAgentPerformance,
	PermDistribution,
	PermSoftphoneSupervise,
	PermSIPExtensionsIndex,
	PermSIPExtensionsStore,
	PermSIPExtensionsUpdate,
	PermSIPExtensionsDestroy,
	PermUserFiltersIndex,
	PermUserFiltersEdit,
	PermUserFiltersUpdate,
	PermUserFiltersDestroy,
	PermRealtimeMonitorIndex,
	PermRealtimeMonitorActions,
	PermSettingsIndex,
	PermSettingsUpdate,
	PermQueueGroupsIndex,
	PermQueueGroupsStore,
	PermQueueGroupsUpdate,
	PermQueueGroupsDestroy,
	PermScheduledReportsIndex,
	PermScheduledReportsStore,
	PermScheduledReportsUpdate,
	PermScheduledReportsDestroy,
	PermUsersIndex,
	PermUsersStore,
	PermUsersUpdate,
	PermUsersDestroy,
}

var (
	ErrInvalidCredentials = errors.New("auth: username or password is incorrect")
	ErrUnknownUser        = errors.New("auth: no such user")
)

// User is one person who may sign in.
type User struct {
	ID           string
	Username     string
	PasswordHash string
	Roles        []string
	Permissions  []string
	// Super short-circuits every permission check, mirroring the bastion
	// console's own super-admin flag.
	Super bool

	// AllowedQueues/AllowedAgents mirror Laravel's call_center_user_filters
	// row-level security (docs/extraction-plan.md §4.3 rule 6). Empty means
	// unrestricted. This is a SECURITY BOUNDARY, not a UI convenience: it must
	// be applied to the query, not just used to populate a filter dropdown.
	AllowedQueues []string
	AllowedAgents []string
}

// Can reports whether the user holds perm.
func (u *User) Can(perm string) bool {
	if u.Super {
		return true
	}
	for _, p := range u.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// Store is the seam a real user directory plugs into (see the package doc).
type Store interface {
	ByUsername(username string) (*User, bool)
	ByID(id string) (*User, bool)
	// List returns every user this service knows about, ordered by
	// username. There is no user DIRECTORY anywhere else in this service
	// (see internal/softphone.ListItem's own doc comment on the same gap)
	// — this is the seam an admin screen that needs "which user" (assigning
	// a SIP extension, say) reads from, same as ByUsername/ByID above.
	List() []*User
	// SetFilters replaces a user's AllowedQueues/AllowedAgents row-level
	// restriction in place — the User Filters admin screen's save/clear
	// action. Reports true if the user was found. This mutates the SAME
	// store Verify()/ByID() read from, so a change here takes effect on
	// that user's very next request — there is no separate cache to bust.
	SetFilters(userID string, queues, agents []string) bool
}

// MemoryStore is an in-process Store. Dev/local use only.
type MemoryStore struct {
	mu       sync.RWMutex
	byID     map[string]*User
	byUsersn map[string]*User
}

func NewMemoryStore(users []*User) *MemoryStore {
	s := &MemoryStore{
		byID:     make(map[string]*User, len(users)),
		byUsersn: make(map[string]*User, len(users)),
	}
	for _, u := range users {
		s.byID[u.ID] = u
		s.byUsersn[strings.ToLower(u.Username)] = u
	}
	return s
}

func (s *MemoryStore) ByUsername(username string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byUsersn[strings.ToLower(username)]
	return u, ok
}

func (s *MemoryStore) ByID(id string) (*User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.byID[id]
	return u, ok
}

// Upsert adds or replaces a user. Used to mirror an externally
// authenticated identity in, so per-request lookups by id resolve.
func (s *MemoryStore) Upsert(u *User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.byID[u.ID]; ok {
		// Preserve row-level restrictions already configured against this
		// account — those are administered here, not in the external
		// directory, and a re-login must not silently clear them.
		u.AllowedQueues = existing.AllowedQueues
		u.AllowedAgents = existing.AllowedAgents
		delete(s.byUsersn, strings.ToLower(existing.Username))
	}
	s.byID[u.ID] = u
	s.byUsersn[strings.ToLower(u.Username)] = u
}

func (s *MemoryStore) SetFilters(userID string, queues, agents []string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.byID[userID]
	if !ok {
		return false
	}
	u.AllowedQueues = nonNil(queues)
	u.AllowedAgents = nonNil(agents)
	return true
}

func (s *MemoryStore) List() []*User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*User, 0, len(s.byID))
	for _, u := range s.byID {
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Username) < strings.ToLower(out[j].Username)
	})
	return out
}

// NewDevStore seeds two accounts for local use, so the permission gating is
// actually exercisable rather than theoretical:
//
//   - the admin holds everything (Super)
//   - the agent holds the dashboard, the unanswered queue and the callback
//     action, but NOT exports, call search, agent performance or distribution
//
// Signing in as the agent should visibly show fewer nav entries and no export
// buttons. If it does not, the gating is not wired up.
//
// Passwords are hashed at startup rather than stored pre-hashed, so no
// credential is committed to the repo.
func NewDevStore(adminPassword, agentPassword string) (*MemoryStore, error) {
	adminHash, err := pwhash.Hash(adminPassword)
	if err != nil {
		return nil, err
	}
	agentHash, err := pwhash.Hash(agentPassword)
	if err != nil {
		return nil, err
	}

	return NewMemoryStore([]*User{
		{
			ID:           "00000000-0000-0000-0000-000000000001",
			Username:     "admin",
			PasswordHash: adminHash,
			Roles:        []string{"administrator"},
			Permissions:  AllPermissions,
			Super:        true,
		},
		{
			ID:           "00000000-0000-0000-0000-000000000002",
			Username:     "agent",
			PasswordHash: agentHash,
			Roles:        []string{"agent"},
			Permissions: []string{
				PermDashboard,
				PermUnansweredIndex,
				PermUnansweredCallback,
			},
		},
	}), nil
}

// Authority is what the client resolves after signing in. It is the whole
// answer to "what may I do", and it is re-read rather than cached in the
// token.
type Authority struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	Super       bool     `json:"super"`

	// The row-level restrictions, echoed so the UI can pre-fill and constrain
	// its queue/agent pickers. The server applies them regardless — this is a
	// courtesy to the client, never the enforcement.
	AllowedQueues []string `json:"allowed_queues"`
	AllowedAgents []string `json:"allowed_agents"`
}

// Service authenticates users and issues tokens.
type Service struct {
	store     Store
	jwtSecret string
	tokenTTL  time.Duration
	// external is optional; nil means this service's own store is the only
	// authority. See internal/auth/external.go.
	external ExternalDirectory
}

func NewService(store Store, jwtSecret string, tokenTTL time.Duration) *Service {
	return &Service{store: store, jwtSecret: jwtSecret, tokenTTL: tokenTTL}
}

// UseExternalDirectory lets sign-ins fall through to an outside directory
// when the local store does not recognise the credentials.
func (s *Service) UseExternalDirectory(d ExternalDirectory) {
	s.external = d
}

// Login verifies credentials and returns a signed token plus its lifetime.
//
// A wrong username and a wrong password both return ErrInvalidCredentials, so
// the response cannot be used to enumerate accounts.
func (s *Service) Login(ctx context.Context, username, password string) (token string, expiresIn int, err error) {
	u, ok := s.store.ByUsername(username)
	if ok && pwhash.Verify(u.PasswordHash, password) == nil {
		return s.issue(u.ID)
	}

	// The local store is checked FIRST so the seeded accounts keep working
	// and their passwords are never sent to an outside directory.
	if s.external != nil {
		if id, xerr := s.adoptExternal(ctx, username, password); xerr == nil {
			return s.issue(id)
		}
	}

	if !ok {
		// Spend roughly the same work as a real verification would, so the
		// response time does not disclose whether the account exists.
		_ = pwhash.Verify(dummyHash, password)
	}
	return "", 0, ErrInvalidCredentials
}

// adoptExternal authenticates against the external directory and mirrors the
// result into the local store, returning the user id to issue a token for.
//
// The mirroring is required, not a cache: the token this service issues
// carries only a user id (see issue below), and every subsequent request
// resolves authority by looking that id up in the store. A user the store
// has never heard of would authenticate once and then be rejected by the
// very next request.
func (s *Service) adoptExternal(ctx context.Context, username, password string) (string, error) {
	identity, err := s.external.Authenticate(ctx, username, password)
	if err != nil {
		return "", err
	}

	perms, super := permissionsForRoles(identity.Roles)
	name := identity.Email
	if name == "" {
		name = username
	}

	upsert, ok := s.store.(interface{ Upsert(*User) })
	if !ok {
		return "", ErrInvalidCredentials
	}
	upsert.Upsert(&User{
		ID:          identity.ID,
		Username:    name,
		Roles:       identity.Roles,
		Permissions: perms,
		Super:       super,
		// No PasswordHash: the external directory verifies the password, and
		// storing a local one would create a second credential for the same
		// account that nothing keeps in step with it.
	})
	return identity.ID, nil
}

func (s *Service) issue(userID string) (string, int, error) {
	token, err := jwtsig.Sign(s.jwtSecret, userID, s.tokenTTL)
	if err != nil {
		return "", 0, err
	}
	return token, int(s.tokenTTL.Seconds()), nil
}

// ListUsers returns every user this service's store knows about — the seam
// an admin picker (SIP Extensions' "assign to user") reads from.
func (s *Service) ListUsers() []*User {
	return s.store.List()
}

// SetUserFilters replaces a user's row-level queue/agent restriction — see
// Store.SetFilters's own doc comment.
func (s *Service) SetUserFilters(userID string, queues, agents []string) bool {
	return s.store.SetFilters(userID, queues, agents)
}

// Verify checks a bearer token and resolves the user behind it. The user is
// looked up fresh every time: a token for an account that has since been
// removed is refused here, not honoured because it is still in date.
func (s *Service) Verify(token string) (*User, error) {
	claims, err := jwtsig.Verify(s.jwtSecret, token)
	if err != nil {
		return nil, err
	}
	u, ok := s.store.ByID(claims.UserID)
	if !ok {
		return nil, ErrUnknownUser
	}
	return u, nil
}

// AuthorityFor builds the authority payload for u.
func AuthorityFor(u *User) Authority {
	perms := u.Permissions
	if u.Super {
		perms = AllPermissions
	}
	return Authority{
		UserID:        u.ID,
		Username:      u.Username,
		Roles:         nonNil(u.Roles),
		Permissions:   nonNil(perms),
		Super:         u.Super,
		AllowedQueues: nonNil(u.AllowedQueues),
		AllowedAgents: nonNil(u.AllowedAgents),
	}
}

// JSON null and [] mean different things to a client iterating the value.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// A well-formed hash of a value nobody can supply, used to keep the
// unknown-user path's timing close to the real one.
const dummyHash = "pbkdf2-sha256$600000$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
