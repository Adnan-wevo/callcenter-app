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
