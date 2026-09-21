package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/gateway/laravel"
	"callcenter-service/internal/middleware"
)

// callbackRequestBody carries the ONE thing the caller gets to choose.
//
// The acting agent is deliberately absent: it comes from the bearer token
// (see below), mirroring the Livewire original, which passes `Auth::id()`
// and never a value from the page.
type callbackRequestBody struct {
	PhoneNumber string `json:"phone_number" binding:"required"`
}

// POST /api/v1/secure/call-center/unanswered-calls/callback
//
// Records an ad-hoc callback attempt in Laravel. This replaces the in-process
// call the CallCenter module used to make into the Callback module —
// `app(RecordAdHocCallbackAttempt::class)->handle($phone, Auth::id())` in
// Modules/CallCenter/app/Livewire/UnansweredCalls/Index.php.
//
// # The agent comes from the token, not the body
//
// An earlier version of this handler took `agent_id` as a request field,
// which would have let any caller with the callback permission record an
// attempt against somebody else's name. The Livewire original never had that
// hole because it used the session's own identity, and neither does this.
//
// # What this does NOT do
//
// The Livewire version also dispatches a `softphone-callback` browser event
// that makes the embedded softphone dial. An HTTP response cannot do that, so
// this endpoint records the attempt only. How the dial is actually triggered
// is open decision D7 in docs/extraction-plan.md.
func (h *Handlers) RecordAdHocCallback(c *gin.Context) {
	user, ok := middleware.UserFrom(c)
	if !ok {
		apires.Error(c, http.StatusUnauthorized, "authentication required", nil)
		return
	}

	var body callbackRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "a phone number is required", gin.H{
			"phone_number": []string{"required"},
		})
		return
	}

	// The Livewire original trims and rejects blank before doing anything,
	// and so does this: a whitespace-only string passes `required`.
	phone := strings.TrimSpace(body.PhoneNumber)
	if phone == "" {
		apires.Error(c, http.StatusUnprocessableEntity, "a phone number is required", gin.H{
			"phone_number": []string{"must not be blank"},
		})
		return
	}

	err := h.laravelCallback.RecordAdHocCallback(c.Request.Context(), laravel.CallbackRequest{
		PhoneNumber: phone,
		AgentID:     user.ID,
	})
	if err != nil {
		apires.Error(c, http.StatusBadGateway, "could not record the callback attempt", nil)
		return
	}

	apires.Item(c, http.StatusAccepted, gin.H{
		"phone_number": phone,
		"agent_id":     user.ID,
		"outcome":      "pending",
	})
}
