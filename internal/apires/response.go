// Package apires provides the standard JSON response envelope.
//
// Every response this service produces is one of two objects, discriminated
// by `status` rather than by the HTTP status code, so a client can branch on
// the body alone:
//
//	{"status":"success","data":<T>,"meta"?:{...},"message"?:"..."}
//	{"status":"error","message":"...","code":422,"errors"?:{"field":["..."]}}
//
// This is decision D1 in docs/extraction-plan.md. It matches the envelope the
// Angular client consumes (see web/src/app/core/api/api.types.ts) and the one
// the sibling bastion service already uses. Note that CallCenter in Laravel
// has no *Resource.php classes at all, so there was no existing Laravel
// convention to inherit here — this shape was chosen, not discovered.
package apires

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Meta carries pagination counters alongside a collection.
//
// The pbx-worker API has no pagination of its own (it returns whole result
// sets), so any Meta this service emits was computed here — see
// docs/extraction-plan.md §4.4.
type Meta struct {
	Page     int    `json:"page"`
	PerPage  int    `json:"per_page"`
	Total    int    `json:"total"`
	LastPage int    `json:"last_page"`
	Search   string `json:"search,omitempty"`
	Sort     string `json:"sort,omitempty"`
	Order    string `json:"order,omitempty"`
}

// Item writes a single resource.
func Item(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"status": "success", "data": data})
}

// OK is Item at 200.
func OK(c *gin.Context, data any) {
	Item(c, http.StatusOK, data)
}

// Collection writes a paginated list.
func Collection(c *gin.Context, status int, data any, meta Meta) {
	c.JSON(status, gin.H{"status": "success", "data": data, "meta": meta})
}

// Error writes a failure. details is optional and, for a 422, should be the
// field-name -> complaints map the client binds to its form inputs.
func Error(c *gin.Context, status int, message string, details any) {
	body := gin.H{"status": "error", "message": message, "code": status}
	if details != nil {
		body["errors"] = details
	}
	c.JSON(status, body)
}
