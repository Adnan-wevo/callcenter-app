package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/middleware"
	"callcenter-service/internal/softphone"
)

// GET /api/v1/secure/softphone/bootstrap
//
// The credentials and PBX coordinates the browser's SIP user agent needs to
// register. This is the Go equivalent of what heal-crm's Livewire component
// hands to JsSIP today.
//
// # It serves the CALLER's extension, never one named in the request
//
// The user is taken from the bearer token. There is deliberately no
// `?user_id=` — an endpoint that returns a SIP password for whoever is named
// in the query is an endpoint that hands out the ability to place calls
// billed to the customer.
//
// An agent with no extension assigned gets 404 rather than an empty payload:
// the difference between "you have no phone" and "your phone is misconfigured"
// is worth keeping.
func (h *Handlers) SoftphoneBootstrap(c *gin.Context) {
	user, ok := middleware.UserFrom(c)
	if !ok {
		apires.Error(c, http.StatusUnauthorized, "authentication required", nil)
		return
	}

	bootstrap, err := h.softphone.BootstrapFor(user.ID)
	if err != nil {
		if errors.Is(err, softphone.ErrNoExtension) {
			apires.Error(c, http.StatusNotFound, "no SIP extension is assigned to your account", nil)
			return
		}
		apires.Error(c, http.StatusInternalServerError, "could not load softphone settings", nil)
		return
	}

	apires.OK(c, bootstrap)
}
