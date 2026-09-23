package pbxv3

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// User is one account in v3's directory (/secure/acl/users).
type User struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Email string   `json:"email"`
	Roles []string `json:"roles,omitempty"`
}

// Page carries v3's pagination counters, which it returns beside the rows
// rather than inside them.
type Page struct {
	CurrentPage int `json:"current_page"`
	PerPage     int `json:"per_page"`
	Total       int `json:"total"`
	LastPage    int `json:"last_page"`
}

// ListUsers returns one page of the directory. search is matched by v3
// against name and email.
func (c *Client) ListUsers(ctx context.Context, page, perPage int, search string) ([]User, Page, error) {
	q := url.Values{}
	if page > 0 {
		q.Set("page", strconv.Itoa(page))
	}
	q.Set("per_page", strconv.Itoa(snapPerPage(perPage)))
	if search != "" {
		q.Set("search", search)
	}

	// The rows sit in `data` and the counters in a sibling `meta`, so this
	// one reads the whole envelope rather than going through do()'s
	// data-only unwrapping.
	var out struct {
		Data []User `json:"data"`
		Meta Page   `json:"meta"`
	}
	if err := c.raw(ctx, http.MethodGet, "/secure/acl/users/datatables", q, nil, &out); err != nil {
		return nil, Page{}, err
	}
	return out.Data, out.Meta, nil
}

// allowedPerPage is what v3's datatables endpoints accept. Anything else is
// not an error there — it is silently replaced with the default of 10, so a
// caller asking for 25 gets 10 rows back and a page count computed against a
// size it never asked for. Snapping here keeps that mismatch out of the API
// this service presents.
var allowedPerPage = []int{10, 50, 100, 500, 1000}

// snapPerPage rounds a requested size DOWN to the nearest v3 accepts, so a
// page never carries more rows than the caller asked to render.
func snapPerPage(requested int) int {
	if requested <= 0 {
		return allowedPerPage[0]
	}
	best := allowedPerPage[0]
	for _, allowed := range allowedPerPage {
		if allowed <= requested {
			best = allowed
		}
	}
	return best
}

// CreateUser adds an account. v3 hashes the password; it is never stored by
// this service.
func (c *Client) CreateUser(ctx context.Context, name, email, password string) (User, error) {
	body := map[string]string{"name": name, "email": email, "password": password}
	var out User
	if err := c.do(ctx, http.MethodPost, "/secure/acl/users/store", nil, body, &out); err != nil {
		return User{}, err
	}
	return out, nil
}

// UpdateUser changes an account's name and email. v3 takes both on every
// update, so a caller changing one must send the other unchanged.
func (c *Client) UpdateUser(ctx context.Context, id, name, email string) error {
	body := map[string]string{"name": name, "email": email}
	return c.do(ctx, http.MethodPut, "/secure/acl/users/update/"+url.PathEscape(id), nil, body, nil)
}

func (c *Client) DeleteUser(ctx context.Context, id string) error {
	return c.do(ctx, http.MethodDelete, "/secure/acl/users/destroy/"+url.PathEscape(id), nil, nil, nil)
}

// AssignRole grants a role. Roles are what this service derives authority
// from (see auth.permissionsForRoles), so this is the call that decides what
// a user can actually do here.
func (c *Client) AssignRole(ctx context.Context, id, role string) error {
	body := map[string]string{"role": role}
	return c.do(ctx, http.MethodPost, "/secure/acl/users/"+url.PathEscape(id)+"/roles/assign", nil, body, nil)
}

func (c *Client) RevokeRole(ctx context.Context, id, role string) error {
	path := "/secure/acl/users/" + url.PathEscape(id) + "/roles/revoke/" + url.PathEscape(role)
	return c.do(ctx, http.MethodDelete, path, nil, nil, nil)
}

// UserRoles returns the roles currently held by one user.
func (c *Client) UserRoles(ctx context.Context, id string) ([]string, error) {
	// `data` is the role array itself, not an object wrapping one.
	var out []struct {
		Name string `json:"name"`
	}
	if err := c.do(ctx, http.MethodGet, "/secure/acl/users/"+url.PathEscape(id)+"/roles", nil, nil, &out); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out))
	for _, r := range out {
		names = append(names, r.Name)
	}
	return names, nil
}

// ListRoles returns every role that can be assigned, so a form offers the
// real set rather than a hardcoded list that drifts.
func (c *Client) ListRoles(ctx context.Context) ([]string, error) {
	var out struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := c.raw(ctx, http.MethodGet, "/secure/acl/roles/datatables", url.Values{"per_page": {"100"}}, nil, &out); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(out.Data))
	for _, r := range out.Data {
		names = append(names, r.Name)
	}
	return names, nil
}

// raw is do() without the envelope unwrapping, for the paginated endpoints
// whose counters live beside `data` rather than inside it.
func (c *Client) raw(ctx context.Context, method, path string, query url.Values, body, out any) error {
	for attempt := 0; attempt < 2; attempt++ {
		token, err := c.currentToken(ctx)
		if err != nil {
			return err
		}
		var encoded []byte
		if body != nil {
			if encoded, err = marshal(body); err != nil {
				return err
			}
		}

		status, rawBody, err := c.send(ctx, method, path, query, encoded, token)
		if err != nil {
			return err
		}
		if status == http.StatusUnauthorized && attempt == 0 {
			c.forget(token)
			continue
		}
		if status < 200 || status > 299 {
			return fmt.Errorf("pbxv3: %s %s: %d %s", method, path, status, summarize(rawBody))
		}
		return unmarshal(rawBody, out)
	}
	return fmt.Errorf("pbxv3: %s %s: still unauthorized after re-login", method, path)
}
