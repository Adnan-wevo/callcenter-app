package pbxv3

import (
	"context"

	"callcenter-service/internal/auth"
)

// AuthDirectory adapts this client to auth.ExternalDirectory, so v3's user
// administration (/secure/acl/users) becomes the directory this service
// signs people in against.
type AuthDirectory struct {
	client *Client
}

func NewAuthDirectory(client *Client) *AuthDirectory {
	return &AuthDirectory{client: client}
}

var _ auth.ExternalDirectory = (*AuthDirectory)(nil)

// Authenticate treats the submitted username as v3's email, which is what
// it identifies accounts by. A username with no "@" is still passed through
// rather than rejected early: deciding what counts as an email is v3's job,
// not this adapter's.
func (d *AuthDirectory) Authenticate(ctx context.Context, username, password string) (auth.ExternalIdentity, error) {
	identity, err := d.client.Login(ctx, username, password)
	if err != nil {
		return auth.ExternalIdentity{}, err
	}
	return auth.ExternalIdentity{
		ID:    identity.ID,
		Email: identity.Email,
		Roles: identity.Roles,
	}, nil
}
