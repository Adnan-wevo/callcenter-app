package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
)

// userFilterRow is one User Filters admin-screen row: enough to show WHO
// and what they're currently restricted to. Empty AllowedQueues/AllowedAgents
// means unrestricted — see auth.User's own doc comment; this mirrors that
// exactly rather than inventing a separate "is restricted" flag.
type userFilterRow struct {
	ID            string   `json:"id"`
	Username      string   `json:"username"`
	AllowedQueues []string `json:"allowed_queues"`
	AllowedAgents []string `json:"allowed_agents"`
}

// GET /api/v1/secure/admin/user-filters
//
// Mirrors UserFilters\Index::users(): every user, searchable by
// name (username here — this store has no email), paginated. Pagination is
// done in memory (paginate() below) because the whole user list already IS
// in memory — auth.MemoryStore is dev/local-only, see its own doc comment.
func (h *Handlers) ListUserFilters(c *gin.Context) {
	search := strings.ToLower(strings.TrimSpace(c.Query("search")))

	all := h.auth.ListUsers()
	rows := make([]userFilterRow, 0, len(all))
	for _, u := range all {
		if search != "" && !strings.Contains(strings.ToLower(u.Username), search) {
			continue
		}
		rows = append(rows, userFilterRow{
			ID:            u.ID,
			Username:      u.Username,
			AllowedQueues: nonNilStrings(u.AllowedQueues),
			AllowedAgents: nonNilStrings(u.AllowedAgents),
		})
	}

	page, meta := paginate(c, rows)
	apires.Collection(c, http.StatusOK, page, meta)
}

type userFiltersBody struct {
	Queues []string `json:"queues"`
	Agents []string `json:"agents"`
}

// PUT /api/v1/secure/admin/user-filters/:id
//
// Mirrors UserFilters\Index::save(): replaces the WHOLE set for this user
// — not a merge, matching the Livewire original's delete-then-recreate.
func (h *Handlers) UpdateUserFilters(c *gin.Context) {
	var body userFiltersBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "invalid request body", nil)
		return
	}
	if !h.auth.SetUserFilters(c.Param("id"), body.Queues, body.Agents) {
		apires.Error(c, http.StatusNotFound, "no such user", nil)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{"status": "updated"})
}

// DELETE /api/v1/secure/admin/user-filters/:id
//
// Mirrors UserFilters\Index::clearFilters(): back to unrestricted.
func (h *Handlers) ClearUserFilters(c *gin.Context) {
	if !h.auth.SetUserFilters(c.Param("id"), nil, nil) {
		apires.Error(c, http.StatusNotFound, "no such user", nil)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{"status": "cleared"})
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
