package auth

import "context"

// ExternalIdentity is a user vouched for by an outside directory.
type ExternalIdentity struct {
	ID    string
	Email string
	Roles []string
}

// ExternalDirectory is an authority this service will accept sign-ins from
// besides its own store — pbx-worker v3, in practice.
//
// This exists because the local store is `NewDevStore`: two accounts seeded
// from environment variables into memory, with no way to add a third and
// nothing surviving a restart. There is no super-admin and no user
// administration, while v3 already ships a full one (/secure/acl/users,
// roles, permissions). Rather than build a second user directory here and
// keep two in sync, v3's becomes the one that matters and this is the seam
// it arrives through.
type ExternalDirectory interface {
	// Authenticate reports who these credentials belong to, or an error if
	// the directory rejects them.
	Authenticate(ctx context.Context, username, password string) (ExternalIdentity, error)
}

// permissionsForRoles turns a directory's role names into this service's own
// authority.
//
// Roles, not permissions, on purpose. v3 issues 307 permission strings in a
// `module__resource__action` vocabulary that does not correspond to this
// service's `call-center.resource.action` one, so mapping them individually
// would mean a large table that quietly goes stale whenever either side
// gains an endpoint. Role names are few and stable.
//
// Anything unrecognised gets the agent set rather than nothing: a user v3
// accepted is a real user, and leaving them with zero authority would look
// like a broken sign-in rather than a deliberate restriction. It is the
// conservative direction — the agent set is the floor, not admin.
func permissionsForRoles(roles []string) (perms []string, super bool) {
	// The names below are the ones v3 actually defines (checked against
	// /secure/acl/roles: user, root, agent, operator), plus the obvious
	// synonyms a differently-seeded deployment might use.
	for _, role := range roles {
		switch role {
		case "root", "super-admin", "superadmin":
			return nil, true
		case "admin", "supervisor":
			return AllPermissions, false
		}
	}
	// Second pass, so a role that merely adds reach never outranks one of
	// the above when a user holds both.
	for _, role := range roles {
		if role == "operator" {
			// The operator console is a supervisor seat: it watches the live
			// board and acts on other people's calls, which is exactly what
			// PermSoftphoneSupervise gates.
			return append(agentPermissions(),
				PermSoftphoneSupervise,
				PermRealtimeMonitorActions,
				PermCallSearchIndex,
			), false
		}
	}
	return agentPermissions(), false
}

// agentPermissions is what a plain agent holds: their own phone, and the
// screens a person taking calls needs. Deliberately excludes every export,
// the admin CRUD surfaces and row-level security administration.
func agentPermissions() []string {
	return []string{
		PermDashboard,
		PermAnsweredIndex,
		PermUnansweredIndex,
		PermUnansweredCallback,
		PermRealtimeMonitorIndex,
	}
}
