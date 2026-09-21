// Package apires provides the standard JSON response envelope, matching the
// existing Laravel API Resource convention: {"data": ...} for a single item
// or a list, with a "meta" block added for paginated lists.
package apires

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Meta mirrors Laravel's default pagination meta shape.
type Meta struct {
	CurrentPage int `json:"current_page"`
	PerPage     int `json:"per_page"`
	Total       int `json:"total"`
	LastPage    int `json:"last_page"`
}

// Item writes {"data": data}.
func Item(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"data": data})
}

// OK is a shortcut for Item(c, http.StatusOK, data).
func OK(c *gin.Context, data any) {
	Item(c, http.StatusOK, data)
}

// Collection writes {"data": data, "meta": meta} for a paginated list.
func Collection(c *gin.Context, status int, data any, meta Meta) {
	c.JSON(status, gin.H{"data": data, "meta": meta})
}

// Error writes a Laravel-style error envelope: {"message": "...", "errors": {...}}.
// details may be nil.
func Error(c *gin.Context, status int, message string, details any) {
	body := gin.H{"message": message}
	if details != nil {
		body["errors"] = details
	}
	c.JSON(status, body)
}
