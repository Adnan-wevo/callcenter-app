package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/softphone"
)

// GET /api/v1/secure/admin/sip-extensions
//
// Ports SipExtensions\Index::sipExtensions() — search across
// extension/display_name, is_default first then extension, paginated. No
// search-by-user-email here: this service has no user directory beyond the
// seeded auth store (see internal/auth), so filtering is scoped to what
// sip_extensions itself holds.
func (h *Handlers) ListSIPExtensions(c *gin.Context) {
	search := c.Query("search")
	page := queryInt(c, "page", 1)
	perPage := queryInt(c, "per_page", 15)

	rows, total, err := h.sipExtensions.List(c.Request.Context(), search, page, perPage)
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not load SIP extensions", nil)
		return
	}

	lastPage := (total + perPage - 1) / perPage
	if lastPage < 1 {
		lastPage = 1
	}
	apires.Collection(c, http.StatusOK, rows, apires.Meta{
		Page: page, PerPage: perPage, Total: total, LastPage: lastPage,
	})
}

// sipExtensionBody mirrors SipExtensions\Index's own form fields exactly —
// same names, same shape, so the validation error a client sees matches
// the field it actually filled in. UserID has no `binding:"required"`
// here because it is only meaningful on create — the owning user cannot be
// changed by an update (sip_extensions.user_id is UNIQUE and the row's
// identity is its :id, not its owner), so CreateSIPExtension checks it
// itself and UpdateSIPExtension never reads it.
type sipExtensionBody struct {
	UserID       string `json:"user_id"`
	Extension    string `json:"extension" binding:"required"`
	SIPUsername  string `json:"sip_username" binding:"required"`
	SIPPassword  string `json:"sip_password"`
	DisplayName  string `json:"display_name"`
	Queues       string `json:"queues"`
	IsSupervisor bool   `json:"is_supervisor"`
	IsDefault    bool   `json:"is_default"`
}

// POST /api/v1/secure/admin/sip-extensions
//
// This is what unblocks assigning a real SIP extension to a user — the
// admin form's create path (SipExtensions\Index::save() when editingId is
// unset).
func (h *Handlers) CreateSIPExtension(c *gin.Context) {
	var body sipExtensionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "invalid request body", nil)
		return
	}
	if body.UserID == "" {
		apires.Error(c, http.StatusUnprocessableEntity, "user_id is required", nil)
		return
	}

	id, err := h.sipExtensions.Create(c.Request.Context(), softphone.CreateInput{
		UserID: body.UserID, Extension: body.Extension, SIPUsername: body.SIPUsername,
		SIPPassword: body.SIPPassword, DisplayName: body.DisplayName, Queues: body.Queues,
		IsSupervisor: body.IsSupervisor, IsDefault: body.IsDefault,
	})
	if err != nil {
		respondSIPExtensionError(c, err)
		return
	}
	apires.Item(c, http.StatusCreated, gin.H{"id": id})
}

// PUT /api/v1/secure/admin/sip-extensions/:id
//
// SIPPassword left blank means "keep the existing password" — matches the
// Livewire form's own rule exactly (validationRules() makes sip_password
// nullable only in edit mode, and save() blanks it to null when unchanged).
func (h *Handlers) UpdateSIPExtension(c *gin.Context) {
	var body sipExtensionBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "invalid request body", nil)
		return
	}

	err := h.sipExtensions.Update(c.Request.Context(), c.Param("id"), softphone.UpdateInput{
		Extension: body.Extension, SIPUsername: body.SIPUsername, SIPPassword: body.SIPPassword,
		DisplayName: body.DisplayName, Queues: body.Queues,
		IsSupervisor: body.IsSupervisor, IsDefault: body.IsDefault,
	})
	if err != nil {
		respondSIPExtensionError(c, err)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{"status": "updated"})
}

// DELETE /api/v1/secure/admin/sip-extensions/:id
func (h *Handlers) DeleteSIPExtension(c *gin.Context) {
	if err := h.sipExtensions.Delete(c.Request.Context(), c.Param("id")); err != nil {
		respondSIPExtensionError(c, err)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{"status": "deleted"})
}

func respondSIPExtensionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, softphone.ErrExtensionInUse):
		apires.Error(c, http.StatusUnprocessableEntity, "that extension number or user already has a SIP extension", nil)
	case errors.Is(err, softphone.ErrNoExtension):
		apires.Error(c, http.StatusNotFound, "no such SIP extension", nil)
	default:
		apires.Error(c, http.StatusUnprocessableEntity, err.Error(), nil)
	}
}

func queryInt(c *gin.Context, key string, fallback int) int {
	v := c.Query(key)
	if v == "" {
		return fallback
	}
	n := 0
	for _, r := range v {
		if r < '0' || r > '9' {
			return fallback
		}
		n = n*10 + int(r-'0')
	}
	if n == 0 {
		return fallback
	}
	return n
}
