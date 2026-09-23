package handlers

import (
	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
)

// publicUser is a User with everything credential-shaped stripped — never
// PasswordHash, never Permissions (an admin picking who to assign a SIP
// extension to needs a name, not their grant list).
type publicUser struct {
	ID       string   `json:"id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}

// GET /api/v1/secure/admin/users
//
// The seam the SIP Extensions admin screen's "assign to user" picker reads
// from — there is no user directory anywhere else in this service (see
// auth.Store.List's own doc comment and softphone.ListItem's, which names
// the same gap from the other side: a SIP extension row has a user_id and
// nothing this service can resolve it against without this endpoint).
func (h *Handlers) ListUsers(c *gin.Context) {
	users := h.auth.ListUsers()
	out := make([]publicUser, 0, len(users))
	for _, u := range users {
		out = append(out, publicUser{ID: u.ID, Username: u.Username, Roles: u.Roles})
	}
	apires.OK(c, out)
}
