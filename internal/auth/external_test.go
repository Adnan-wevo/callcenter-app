package auth

import "testing"

func TestPermissionsForRoles(t *testing.T) {
	cases := []struct {
		name      string
		roles     []string
		wantSuper bool
		wantPerms int
	}{
		{"v3 super admin", []string{"root", "user"}, true, 0},
		{"admin gets the full catalogue", []string{"admin"}, false, len(AllPermissions)},
		{"supervisor gets the full catalogue", []string{"supervisor"}, false, len(AllPermissions)},
		{"plain user falls to the agent floor", []string{"user"}, false, len(agentPermissions())},
		{"unknown role falls to the agent floor", []string{"whatever"}, false, len(agentPermissions())},
		{"no roles at all falls to the agent floor", nil, false, len(agentPermissions())},
		{"super wins over a lesser role beside it", []string{"user", "root"}, true, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			perms, super := permissionsForRoles(c.roles)
			if super != c.wantSuper {
				t.Errorf("super = %v, want %v", super, c.wantSuper)
			}
			if len(perms) != c.wantPerms {
				t.Errorf("len(perms) = %d, want %d", len(perms), c.wantPerms)
			}
		})
	}
}

// The agent floor must never include an export or an admin CRUD permission:
// an unrecognised role lands here, so anything granted by default is granted
// to a user this service knows nothing about.
func TestAgentFloorGrantsNothingPrivileged(t *testing.T) {
	forbidden := []string{
		PermAnsweredExport, PermUnansweredExport, PermCallSearchExport,
		PermSIPExtensionsStore, PermSIPExtensionsDestroy,
		PermUserFiltersUpdate, PermUserFiltersDestroy,
		PermSettingsUpdate, PermQueueGroupsStore, PermScheduledReportsStore,
		PermSoftphoneSupervise, PermRealtimeMonitorActions,
	}
	for _, p := range agentPermissions() {
		for _, bad := range forbidden {
			if p == bad {
				t.Errorf("agent floor grants privileged permission %q", p)
			}
		}
	}
}

// The roles below are the ones v3 actually defines, so these are the cases
// that decide what a real user of this system can do.
func TestPermissionsForRealV3Roles(t *testing.T) {
	if _, super := permissionsForRoles([]string{"root"}); !super {
		t.Error("root must be super")
	}
	if _, super := permissionsForRoles([]string{"operator"}); super {
		t.Error("operator must not be super")
	}
	perms, _ := permissionsForRoles([]string{"operator"})
	var supervise bool
	for _, p := range perms {
		if p == PermSoftphoneSupervise {
			supervise = true
		}
	}
	if !supervise {
		t.Error("operator should hold softphone.supervise")
	}
	// Holding a lesser role alongside root must not cost the super flag.
	if _, super := permissionsForRoles([]string{"operator", "root"}); !super {
		t.Error("root alongside operator must still be super")
	}
}
