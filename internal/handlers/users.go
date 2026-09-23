package handlers

import (
	"net/http"

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
// from. Serves v3's directory when one is configured and the local store
// otherwise, because with v3 wired in the local store only ever holds the
// two seeded accounts plus whoever has signed in since this process
// started — a picker built from that would be missing most of the company.
func (h *Handlers) ListUsers(c *gin.Context) {
	if h.pbxV3 != nil {
		rows, _, err := h.pbxV3.ListUsers(c.Request.Context(), 1, 200, c.Query("search"))
		if err == nil {
			out := make([]publicUser, 0, len(rows))
			for _, u := range rows {
				out = append(out, publicUser{ID: u.ID, Username: labelFor(u.Name, u.Email), Roles: u.Roles})
			}
			apires.OK(c, out)
			return
		}
		// Fall through to the local store rather than failing the picker:
		// a short list beats a broken form.
	}

	users := h.auth.ListUsers()
	out := make([]publicUser, 0, len(users))
	for _, u := range users {
		out = append(out, publicUser{ID: u.ID, Username: u.Username, Roles: u.Roles})
	}
	apires.OK(c, out)
}

func labelFor(name, email string) string {
	if name != "" {
		return name
	}
	return email
}

// adminUser is one row of the user-administration screen.
type adminUser struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

// requireDirectory reports whether user administration is available at all.
// It is backed entirely by v3, so without it there is nothing to administer
// — the local store is seeded from environment variables and cannot grow an
// account.
func (h *Handlers) requireDirectory(c *gin.Context) bool {
	if h.pbxV3 == nil {
		apires.Error(c, http.StatusServiceUnavailable,
			"user administration needs pbx-worker v3 to be configured", nil)
		return false
	}
	return true
}

// GET /api/v1/secure/admin/users/manage
func (h *Handlers) ListManagedUsers(c *gin.Context) {
	if !h.requireDirectory(c) {
		return
	}
	page := queryInt(c, "page", 1)
	perPage := queryInt(c, "per_page", 25)
	if perPage > 200 {
		perPage = 200
	}

	rows, meta, err := h.pbxV3.ListUsers(c.Request.Context(), page, perPage, c.Query("search"))
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "could not load users", nil)
		return
	}

	// v3's list omits roles, so they are fetched per row. Bounded by
	// per_page (200 at most) and only on a screen an administrator opens
	// deliberately, which is a fair trade for a list that actually shows
	// what each account can do — the whole point of the screen.
	out := make([]adminUser, 0, len(rows))
	for _, u := range rows {
		roles, rerr := h.pbxV3.UserRoles(c.Request.Context(), u.ID)
		if rerr != nil {
			roles = nil
		}
		out = append(out, adminUser{ID: u.ID, Name: u.Name, Email: u.Email, Roles: roles})
	}

	lastPage := meta.LastPage
	if lastPage < 1 {
		lastPage = 1
	}
	apires.Collection(c, http.StatusOK, out, apires.Meta{
		Page: page, PerPage: perPage, Total: meta.Total, LastPage: lastPage,
	})
}

// GET /api/v1/secure/admin/roles
func (h *Handlers) ListRoles(c *gin.Context) {
	if !h.requireDirectory(c) {
		return
	}
	roles, err := h.pbxV3.ListRoles(c.Request.Context())
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "could not load roles", nil)
		return
	}
	apires.OK(c, roles)
}

type createUserBody struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
	Role     string `json:"role"`
}

// POST /api/v1/secure/admin/users
func (h *Handlers) CreateUser(c *gin.Context) {
	if !h.requireDirectory(c) {
		return
	}
	var body createUserBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "name, email and password are required", nil)
		return
	}

	user, err := h.pbxV3.CreateUser(c.Request.Context(), body.Name, body.Email, body.Password)
	if err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "could not create that user", nil)
		return
	}

	// The role is what decides the account's authority here (see
	// auth.permissionsForRoles), so a failure to assign it is reported
	// rather than swallowed: the user exists but cannot do what was asked
	// for, and an administrator needs to know that.
	if body.Role != "" {
		if err := h.pbxV3.AssignRole(c.Request.Context(), user.ID, body.Role); err != nil {
			apires.Error(c, http.StatusBadGateway,
				"the user was created but the role could not be assigned", gin.H{"id": user.ID})
			return
		}
	}
	apires.Item(c, http.StatusCreated, gin.H{"id": user.ID})
}

type updateUserBody struct {
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required"`
	Role  string `json:"role"`
}

// PUT /api/v1/secure/admin/users/:id
func (h *Handlers) UpdateUser(c *gin.Context) {
	if !h.requireDirectory(c) {
		return
	}
	var body updateUserBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "name and email are required", nil)
		return
	}
	id := c.Param("id")

	if err := h.pbxV3.UpdateUser(c.Request.Context(), id, body.Name, body.Email); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "could not update that user", nil)
		return
	}

	// Roles are a set in v3, so switching one means revoking what is there
	// rather than adding beside it — otherwise demoting somebody would
	// leave their old, higher role in place and change nothing.
	if body.Role != "" {
		current, err := h.pbxV3.UserRoles(c.Request.Context(), id)
		if err != nil {
			apires.Error(c, http.StatusBadGateway, "could not read that user's roles", nil)
			return
		}
		for _, role := range current {
			if role != body.Role {
				_ = h.pbxV3.RevokeRole(c.Request.Context(), id, role)
			}
		}
		if err := h.pbxV3.AssignRole(c.Request.Context(), id, body.Role); err != nil {
			apires.Error(c, http.StatusBadGateway, "could not assign that role", nil)
			return
		}
	}
	apires.Item(c, http.StatusOK, gin.H{"status": "updated"})
}

// DELETE /api/v1/secure/admin/users/:id
func (h *Handlers) DeleteUser(c *gin.Context) {
	if !h.requireDirectory(c) {
		return
	}
	if err := h.pbxV3.DeleteUser(c.Request.Context(), c.Param("id")); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "could not delete that user", nil)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{"status": "deleted"})
}
