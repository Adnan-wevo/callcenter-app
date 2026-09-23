package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/auth"
	"callcenter-service/internal/middleware"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// POST /api/v1/open/auth/login
//
// Open route: it is called by someone who holds nothing yet, which is the
// point.
func (h *Handlers) Login(c *gin.Context) {
	var body loginRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "username and password are required", gin.H{
			"username": []string{"required"},
			"password": []string{"required"},
		})
		return
	}

	token, expiresIn, err := h.auth.Login(c.Request.Context(), body.Username, body.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			apires.Error(c, http.StatusUnauthorized, "Username or password is incorrect.", nil)
			return
		}
		apires.Error(c, http.StatusInternalServerError, "could not complete sign in", nil)
		return
	}

	apires.OK(c, tokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   expiresIn,
	})
}

// GET /api/v1/secure/me/authority
//
// What the caller may do. Read fresh on every call rather than baked into the
// token, so a revoked grant stops applying immediately. It is mounted behind
// authentication alone and never behind a permission, so it cannot 403 — the
// client resyncs through it after a forbidden response, and a 403 here would
// loop.
func (h *Handlers) MyAuthority(c *gin.Context) {
	user, ok := middleware.UserFrom(c)
	if !ok {
		apires.Error(c, http.StatusUnauthorized, "authentication required", nil)
		return
	}
	apires.OK(c, auth.AuthorityFor(user))
}
