package qstats

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Repository is a thin read-only data access layer over the qstats database.
//
// See models.go for schema provenance notes. None of these methods are
// confirmed to be the right integration point for reporting — the real
// Laravel app sources report data via internal/gateway/pbxworker instead,
// and only reads qstats directly for health-check row counts.
// ListQueueNames/ListQueueAgents are wired to handlers today as a
// placeholder inherited from before this schema was confirmed; revisit
// whether they should instead go through the pbx-worker gateway (see
// README "Open questions").
type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListQueueNames(ctx context.Context) ([]QueueName, error) {
	var rows []QueueName
	err := r.db.SelectContext(ctx, &rows, `SELECT queue_id, queue FROM qname ORDER BY queue`)
	if err != nil {
		return nil, fmt.Errorf("qstats: list qname: %w", err)
	}
	return rows, nil
}

// ListQueueAgents lists every row in qagent. The real schema has no
// queue-linkage column on this table — agents are not tied to a single
// queue at the qagent-row level — so there is no per-queue filter here
// (an earlier version of this method assumed a queue_id column that
// doesn't exist).
func (r *Repository) ListQueueAgents(ctx context.Context) ([]QueueAgent, error) {
	var rows []QueueAgent
	err := r.db.SelectContext(ctx, &rows, `SELECT agent_id, agent FROM qagent ORDER BY agent`)
	if err != nil {
		return nil, fmt.Errorf("qstats: list qagent: %w", err)
	}
	return rows, nil
}

// ListEventTypes lists every row in qevent — a small lookup table, not a
// per-occurrence log. Not currently wired to any handler.
func (r *Repository) ListEventTypes(ctx context.Context) ([]QueueEvent, error) {
	var rows []QueueEvent
	err := r.db.SelectContext(ctx, &rows, `SELECT event_id, event FROM qevent ORDER BY event_id`)
	if err != nil {
		return nil, fmt.Errorf("qstats: list qevent: %w", err)
	}
	return rows, nil
}

// ListQueueStats is not currently wired to any handler. qname is the FK
// column name on queue_stats (see QueueStat doc comment), not a display
// name.
func (r *Repository) ListQueueStats(ctx context.Context, qname int64, limit, offset int) ([]QueueStat, error) {
	var rows []QueueStat
	err := r.db.SelectContext(ctx, &rows,
		`SELECT queue_stats_id, datetime, qname, qagent, qevent, uniqueid
		 FROM queue_stats WHERE qname = ? ORDER BY datetime DESC LIMIT ? OFFSET ?`,
		qname, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("qstats: list queue_stats: %w", err)
	}
	return rows, nil
}

// ListQueueStatsMV is not currently wired to any handler. Only the three
// confirmed datetime columns are selected — see QueueStatMV doc comment
// for why the rest of this table's schema is not modeled yet.
func (r *Repository) ListQueueStatsMV(ctx context.Context, limit int) ([]QueueStatMV, error) {
	var rows []QueueStatMV
	err := r.db.SelectContext(ctx, &rows,
		`SELECT datetime, datetimeconnect, datetimeend FROM queue_stats_mv ORDER BY datetime DESC LIMIT ?`,
		limit)
	if err != nil {
		return nil, fmt.Errorf("qstats: list queue_stats_mv: %w", err)
	}
	return rows, nil
}

// GetRecordingByUniqueID is not currently wired to any handler. It may be
// needed to enrich the pbx-worker call-detail response with a recording
// filename if pbx-worker doesn't already include one — see
// internal/handlers/calls.go.
func (r *Repository) GetRecordingByUniqueID(ctx context.Context, uniqueID string) (*Recording, error) {
	var rec Recording
	err := r.db.GetContext(ctx, &rec,
		`SELECT uniqueid, filename FROM recordings WHERE uniqueid = ? LIMIT 1`,
		uniqueID)
	if err != nil {
		return nil, fmt.Errorf("qstats: get recording for uniqueid %s: %w", uniqueID, err)
	}
	return &rec, nil
}
