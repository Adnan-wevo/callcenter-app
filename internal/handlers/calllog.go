package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"callcenter-service/internal/apires"
	"callcenter-service/internal/calllog"
	"callcenter-service/internal/middleware"
)

// createCallLogBody mirrors Softphone::createCallLog()'s $data array
// field-for-field.
type createCallLogBody struct {
	CallerID    string `json:"caller_id"`
	CallerName  string `json:"caller_name"`
	Queue       string `json:"queue"`
	Direction   string `json:"direction"`
	Channel     string `json:"channel"`
	UniqueID    string `json:"unique_id"`
	Destination string `json:"destination"`
}

// POST /api/v1/secure/softphone/call-logs
//
// The browser calls this the moment a call starts ringing (inbound) or is
// dialled (outbound) — the direct equivalent of Softphone::createCallLog(),
// called from the Alpine JS side of heal-crm's softphone panel. See
// internal/calllog's own doc comment for why this reconciles against an
// existing row rather than always inserting.
func (h *Handlers) CreateCallLog(c *gin.Context) {
	user, ok := middleware.UserFrom(c)
	if !ok {
		apires.Error(c, http.StatusUnauthorized, "authentication required", nil)
		return
	}

	var body createCallLogBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "invalid request body", nil)
		return
	}

	extension := ""
	if ext, ok := h.softphone.ExtensionFor(c.Request.Context(), user.ID); ok {
		extension = ext
	}

	id, err := h.callLogs.CreateOrMerge(c.Request.Context(), calllog.CreateInput{
		CallerID:    body.CallerID,
		CallerName:  body.CallerName,
		Queue:       body.Queue,
		Direction:   body.Direction,
		Channel:     body.Channel,
		UniqueID:    body.UniqueID,
		Destination: body.Destination,
		UserID:      user.ID,
		Extension:   extension,
	})
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not record the call", nil)
		return
	}

	apires.Item(c, http.StatusCreated, gin.H{"id": id})
}

// POST /api/v1/secure/softphone/call-logs/:id/answered
//
// Softphone::markCallAnswered() ported directly.
func (h *Handlers) MarkCallAnswered(c *gin.Context) {
	user, ok := middleware.UserFrom(c)
	if !ok {
		apires.Error(c, http.StatusUnauthorized, "authentication required", nil)
		return
	}

	extension := ""
	if ext, ok := h.softphone.ExtensionFor(c.Request.Context(), user.ID); ok {
		extension = ext
	}

	if err := h.callLogs.MarkAnswered(c.Request.Context(), c.Param("id"), extension, user.ID); err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not update the call", nil)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{"status": "answered"})
}

type finalizeCallLogBody struct {
	Status      string `json:"status"`
	Duration    int    `json:"duration"`
	WaitSeconds int    `json:"wait_seconds"`
	Delete      bool   `json:"_delete"`
}

// POST /api/v1/secure/softphone/call-logs/:id/finalize
//
// Softphone::finalizeCallLog() ported directly, including the guard that
// never lets an answered call be downgraded to missed/abandoned.
func (h *Handlers) FinalizeCallLog(c *gin.Context) {
	var body finalizeCallLogBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "invalid request body", nil)
		return
	}

	err := h.callLogs.Finalize(c.Request.Context(), c.Param("id"), calllog.FinalizeInput{
		Status:      body.Status,
		Duration:    body.Duration,
		WaitSeconds: body.WaitSeconds,
		Delete:      body.Delete,
	})
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not finalize the call", nil)
		return
	}
	apires.Item(c, http.StatusOK, gin.H{"status": "finalized"})
}

// callLogEntry is the JSON shape of one History-tab row — a flattened,
// null-free view of calllog.CallLog, since the DB row's sql.NullString/
// sql.NullTime fields would otherwise serialise as {"String":"x","Valid":true}.
type callLogEntry struct {
	ID           string  `json:"id"`
	Direction    string  `json:"direction"`
	Status       string  `json:"status"`
	CallerID     string  `json:"caller_id"`
	CallerName   string  `json:"caller_name"`
	Destination  string  `json:"destination"`
	Queue        string  `json:"queue"`
	WaitSeconds  int     `json:"wait_seconds"`
	TalkSeconds  int     `json:"talk_seconds"`
	StartedAt    *string `json:"started_at"`
	AnsweredAt   *string `json:"answered_at"`
	EndedAt      *string `json:"ended_at"`
	HasRecording bool    `json:"has_recording"`
}

func toCallLogEntry(row calllog.CallLog) callLogEntry {
	return callLogEntry{
		ID:           row.ID,
		Direction:    row.Direction,
		Status:       row.Status,
		CallerID:     row.CallerID.String,
		CallerName:   row.CallerName.String,
		Destination:  row.Destination.String,
		Queue:        row.Queue.String,
		WaitSeconds:  row.WaitSeconds,
		TalkSeconds:  row.TalkSeconds,
		StartedAt:    nullTimeString(row.StartedAt),
		AnsweredAt:   nullTimeString(row.AnsweredAt),
		EndedAt:      nullTimeString(row.EndedAt),
		HasRecording: row.HasRecording(),
	}
}

func nullTimeString(t sql.NullTime) *string {
	if !t.Valid {
		return nil
	}
	s := t.Time.Format(time.RFC3339)
	return &s
}

// GET /api/v1/secure/softphone/call-logs/mine
//
// The softphone panel's own "History" tab: the CALLER's own past calls,
// scoped by their extension exactly like queue login/logout/pause are —
// authentication is the only gate, there is no separate permission for
// "may see your own call history", and there is deliberately no way to
// pass another extension's number in.
func (h *Handlers) ListMyCallLogs(c *gin.Context) {
	user, ok := middleware.UserFrom(c)
	if !ok {
		apires.Error(c, http.StatusUnauthorized, "authentication required", nil)
		return
	}

	extension, ok := h.softphone.ExtensionFor(c.Request.Context(), user.ID)
	if !ok || extension == "" {
		apires.Error(c, http.StatusNotFound, "no SIP extension is assigned to your account", nil)
		return
	}

	page := queryInt(c, "page", 1)
	perPage := queryInt(c, "per_page", 25)
	if perPage > 200 {
		perPage = 200
	}

	rows, total, err := h.callLogs.ListForExtension(c.Request.Context(), extension, page, perPage)
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not load your call history", nil)
		return
	}

	entries := make([]callLogEntry, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, toCallLogEntry(row))
	}

	lastPage := (total + perPage - 1) / perPage
	if lastPage < 1 {
		lastPage = 1
	}
	apires.Collection(c, http.StatusOK, entries, apires.Meta{
		Page: page, PerPage: perPage, Total: total, LastPage: lastPage,
	})
}

type detectQueueBody struct {
	CallerID  string `json:"caller_id" binding:"required"`
	CallLogID string `json:"call_log_id"`
	UniqueID  string `json:"unique_id"`
	Channel   string `json:"channel"`
}

// POST /api/v1/secure/softphone/detect-queue
//
// Softphone::detectQueue()'s DB-backed layers, plus a live PBX-worker
// snapshot lookup — see calllog.Repository.DetectQueue's own doc comment
// for exactly which of the PHP original's 5 layers are and are not here.
func (h *Handlers) DetectQueue(c *gin.Context) {
	var body detectQueueBody
	if err := c.ShouldBindJSON(&body); err != nil {
		apires.Error(c, http.StatusUnprocessableEntity, "caller_id is required", nil)
		return
	}

	var callLogID, uniqueID, channel *string
	if body.CallLogID != "" {
		callLogID = &body.CallLogID
	}
	if body.UniqueID != "" {
		uniqueID = &body.UniqueID
	}
	if body.Channel != "" {
		channel = &body.Channel
	}

	liveLookup := func(callerID string) (string, bool) {
		if h.pbxControl == nil {
			return "", false
		}
		snap, _, err := h.pbxControl.Queues(c.Request.Context())
		if err != nil {
			return "", false
		}
		for queueID, q := range snap.Queues {
			for _, caller := range q.Callers {
				if caller.CallerID.String() == callerID {
					return queueID, true
				}
			}
		}
		return "", false
	}

	result, err := h.callLogs.DetectQueue(c.Request.Context(), body.CallerID, callLogID, uniqueID, channel, liveLookup)
	if err != nil {
		apires.Error(c, http.StatusInternalServerError, "could not detect queue", nil)
		return
	}

	apires.Item(c, http.StatusOK, gin.H{
		"queue":       result.Queue,
		"call_log_id": result.CallLogID,
	})
}
