package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/queuegroups"
)

// GET /api/v1/secure/admin/queue-groups
func (h *Handlers) ListQueueGroups(c *gin.Context) {
	search := c.Query("search")
	page := queryInt(c, "page", 1)
	perPage := queryInt(c, "per_page", 10)

	rows, total, err := h.queueGroups.List(c.Request.Context(), search, page, perPage)
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not load queue groups", nil)
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

type queueGroupBody struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	IsActive    bool     `json:"is_active"`
	SortOrder   int      `json:"sort_order"`
	Queues      []string `json:"queues"`
}

// POST /api/v1/secure/admin/queue-groups
func (h *Handlers) CreateQueueGroup(c *gin.Context) {
	var body queueGroupBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "name is required", nil)
		return
	}
	id, err := h.queueGroups.Create(c.Request.Context(), queuegroups.Input{
		Name: body.Name, Description: body.Description,
		IsActive: body.IsActive, SortOrder: body.SortOrder, Queues: body.Queues,
	})
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not create the queue group", nil)
		return
	}
	apires.Item(c, http.StatusCreated, gin.H{"id": id})
}

// PUT /api/v1/secure/admin/queue-groups/:id
func (h *Handlers) UpdateQueueGroup(c *gin.Context) {
	var body queueGroupBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "name is required", nil)
		return
	}
	err := h.queueGroups.Update(c.Request.Context(), c.Param("id"), queuegroups.Input{
		Name: body.Name, Description: body.Description,
		IsActive: body.IsActive, SortOrder: body.SortOrder, Queues: body.Queues,
	})
	if errors.Is(err, queuegroups.ErrNotFound) {
		apires.Error(c, http.StatusNotFound, "no such queue group", nil)
		return
	}
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not update the queue group", nil)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{"status": "updated"})
}

// DELETE /api/v1/secure/admin/queue-groups/:id
func (h *Handlers) DeleteQueueGroup(c *gin.Context) {
	err := h.queueGroups.Delete(c.Request.Context(), c.Param("id"))
	if errors.Is(err, queuegroups.ErrNotFound) {
		apires.Error(c, http.StatusNotFound, "no such queue group", nil)
		return
	}
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not delete the queue group", nil)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{"status": "deleted"})
}
