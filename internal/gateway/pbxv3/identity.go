package pbxv3

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// Identity is who v3 says a set of credentials belongs to.
//
// Roles rather than permissions is deliberate. v3 issues 307 permission
// strings in its own `module__resource__action` vocabulary, which does not
// line up with this service's `call-center.resource.action` one; mapping
// them name by name would be a large table that silently rots every time
// either side adds an endpoint. Roles are a handful of stable names, so
// that is what authority here is derived from.
type Identity struct {
	// ID is v3's own user id (the JWT's `sub`), used as this service's user
	// id too so the two directories agree on who somebody is.
	ID    string
	Email string
	Roles []string
}

// v3Claims is the subset of v3's access-token payload this service reads.
type v3Claims struct {
	Sub   string   `json:"sub"`
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

// Login verifies credentials against v3 and reports who they belong to.
//
// Separate from the internal login() used for this client's own service
// account: that one caches a token for subsequent calls, while this one
// authenticates somebody else and keeps nothing.
func (c *Client) Login(ctx context.Context, email, password string) (Identity, error) {
	token, err := c.loginAs(ctx, email, password)
	if err != nil {
		return Identity{}, err
	}
	claims, err := decodeClaims(token)
	if err != nil {
		return Identity{}, err
	}
	if claims.Sub == "" {
		return Identity{}, fmt.Errorf("pbxv3: token carries no subject")
	}
	return Identity{ID: claims.Sub, Email: claims.Email, Roles: claims.Roles}, nil
}

// decodeClaims reads a JWT payload WITHOUT verifying its signature.
//
// Not a gap: the token is not being trusted as a bearer credential here. It
// came back over this client's own request to v3's login endpoint, and the
// question being asked of it is only "who did v3 say that was". Verifying
// would need v3's JWT secret, which this service has no business holding.
// Nothing downstream accepts this token — this service issues its own.
func decodeClaims(token string) (v3Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return v3Claims{}, fmt.Errorf("pbxv3: token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return v3Claims{}, fmt.Errorf("pbxv3: decode token payload: %w", err)
	}
	var claims v3Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return v3Claims{}, fmt.Errorf("pbxv3: parse token payload: %w", err)
	}
	return claims, nil
}
