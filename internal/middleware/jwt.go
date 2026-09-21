package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/auth"
)

// contextUserKey is where the authenticated user is stashed for handlers.
// Unexported with an accessor below, so no handler can fabricate one by
// writing the same string key.
const contextUserKey = "auth.user"

// JWTAuth verifies the Authorization: Bearer <token> header and resolves the
// user behind it on EVERY request — the token is identity, the store is
// authority. A token for an account that has since been removed is refused
// here rather than honoured because it is still in date.
func JWTAuth(svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, ok := bearerToken(header)
		if !ok {
			apires.Error(c, http.StatusUnauthorized, "authentication required", nil)
			c.Abort()
			return
		}

		user, err := svc.Verify(token)
		if err != nil {
			// Deliberately one message for every cause (expired, forged,
			// malformed, unknown user): the client's reaction is the same in
			// all cases, and naming which one it was tells an attacker which
			// part of their token to fix.
			apires.Error(c, http.StatusUnauthorized, "session is invalid or has expired", nil)
			c.Abort()
			return
		}

		c.Set(contextUserKey, user)
		c.Next()
	}
}

// RequirePermission admits the request only when the caller holds at least
// one of perms. OR-combined, mirroring the route guards on the client and
// Laravel's own dual-scope gates.
//
// This is the enforcement. The client hides controls it believes are not
// permitted, but that is a courtesy — nothing is trusted from the browser.
func RequirePermission(perms ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := UserFrom(c)
		if !ok {
			apires.Error(c, http.StatusUnauthorized, "authentication required", nil)
			c.Abort()
			return
		}

		for _, p := range perms {
			if user.Can(p) {
				c.Next()
				return
			}
		}

		apires.Error(c, http.StatusForbidden, "you do not have permission to do that", nil)
		c.Abort()
	}
}

// UserFrom returns the authenticated user placed by JWTAuth.
func UserFrom(c *gin.Context) (*auth.User, bool) {
	v, exists := c.Get(contextUserKey)
	if !exists {
		return nil, false
	}
	u, ok := v.(*auth.User)
	return u, ok
}

// bearerToken pulls the credential out of an Authorization header. The scheme
// match is case-insensitive per RFC 7235.
func bearerToken(header string) (string, bool) {
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}
