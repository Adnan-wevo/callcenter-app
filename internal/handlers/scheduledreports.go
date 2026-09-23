package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/scheduledreports"
)

// GET /api/v1/secure/admin/scheduled-reports
func (h *Handlers) ListScheduledReports(c *gin.Context) {
	search := c.Query("search")
	page := queryInt(c, "page", 1)
	perPage := queryInt(c, "per_page", 10)

	rows, total, err := h.scheduledReports.List(c.Request.Context(), search, page, perPage)
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not load scheduled reports", nil)
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

type scheduledReportBody struct {
	Name             string   `json:"name" binding:"required"`
	DestinationEmail string   `json:"destination_email" binding:"required"`
	Reports          []string `json:"reports" binding:"required,min=1"`
	Queues           []string `json:"queues"`
	LastDays         int      `json:"last_days"`
	CronDayMonth     string   `json:"cron_day_month"`
	CronDayWeek      string   `json:"cron_day_week"`
	CronHour         string   `json:"cron_hour" binding:"required"`
	CronMinute       string   `json:"cron_minute" binding:"required"`
	IsActive         bool     `json:"is_active"`
}

func (b scheduledReportBody) toInput() scheduledreports.Input {
	lastDays := b.LastDays
	if lastDays < 1 {
		lastDays = 1
	}
	return scheduledreports.Input{
		Name: b.Name, DestinationEmail: b.DestinationEmail, Reports: b.Reports, Queues: b.Queues,
		LastDays: lastDays, CronDayMonth: b.CronDayMonth, CronDayWeek: b.CronDayWeek,
		CronHour: b.CronHour, CronMinute: b.CronMinute, IsActive: b.IsActive,
	}
}

// POST /api/v1/secure/admin/scheduled-reports
func (h *Handlers) CreateScheduledReport(c *gin.Context) {
	var body scheduledReportBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "name, destination_email, reports, cron_hour and cron_minute are required", nil)
		return
	}
	id, err := h.scheduledReports.Create(c.Request.Context(), body.toInput())
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not create the scheduled report", nil)
		return
	}
	apires.Item(c, http.StatusCreated, gin.H{"id": id})
}

// PUT /api/v1/secure/admin/scheduled-reports/:id
func (h *Handlers) UpdateScheduledReport(c *gin.Context) {
	var body scheduledReportBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "name, destination_email, reports, cron_hour and cron_minute are required", nil)
		return
	}
	err := h.scheduledReports.Update(c.Request.Context(), c.Param("id"), body.toInput())
	if errors.Is(err, scheduledreports.ErrNotFound) {
		apires.Error(c, http.StatusNotFound, "no such scheduled report", nil)
		return
	}
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not update the scheduled report", nil)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{"status": "updated"})
}

// DELETE /api/v1/secure/admin/scheduled-reports/:id
func (h *Handlers) DeleteScheduledReport(c *gin.Context) {
	err := h.scheduledReports.Delete(c.Request.Context(), c.Param("id"))
	if errors.Is(err, scheduledreports.ErrNotFound) {
		apires.Error(c, http.StatusNotFound, "no such scheduled report", nil)
		return
	}
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not delete the scheduled report", nil)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{"status": "deleted"})
}
