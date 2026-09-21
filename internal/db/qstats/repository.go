package qstats

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// Repository is a thin read-only data access layer over the qstats database.
//
// Only ListQueueNames and ListQueueAgents are currently wired to an HTTP
// handler (see internal/handlers) — see the "Open questions" section in
// README.md for why. The remaining methods are implemented per spec item 3
// ("qstats DB connection layer ... untuk 5 table") and are ready to wire up
// once we know which reports (if any) should read from qstats directly vs.
// via the pbx-worker HTTP gateway.
type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListQueueNames(ctx context.Context) ([]QueueName, error) {
	var rows []QueueName
	// ASSUMPTION: table/column names — see models.go header.
	err := r.db.SelectContext(ctx, &rows, `SELECT id, extension, name, created_at FROM queue_names ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("qstats: list queue_names: %w", err)
	}
	return rows, nil
}

func (r *Repository) ListQueueAgents(ctx context.Context, queueID *int64) ([]QueueAgent, error) {
	var rows []QueueAgent
	var err error
	if queueID != nil {
		err = r.db.SelectContext(ctx, &rows, `SELECT id, agent_id, name, queue_id, created_at FROM queue_agents WHERE queue_id = ? ORDER BY name`, *queueID)
	} else {
		err = r.db.SelectContext(ctx, &rows, `SELECT id, agent_id, name, queue_id, created_at FROM queue_agents ORDER BY name`)
	}
	if err != nil {
		return nil, fmt.Errorf("qstats: list queue_agents: %w", err)
	}
	return rows, nil
}

// ListQueueStats is not currently wired to any handler — see doc comment above.
func (r *Repository) ListQueueStats(ctx context.Context, queueID int64, limit, offset int) ([]QueueStat, error) {
	var rows []QueueStat
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, queue_id, agent_id, call_id, event_type, wait_seconds, talk_seconds, started_at, ended_at
		 FROM queue_stats WHERE queue_id = ? ORDER BY started_at DESC LIMIT ? OFFSET ?`,
		queueID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("qstats: list queue_stats: %w", err)
	}
	return rows, nil
}

// ListQueueStatsSummary reads the queue_stats_mv materialized view — not
// currently wired to any handler; the spec's endpoint list doesn't include
// an explicit "daily summary" report, but the table exists so it's exposed
// here for when that's clarified.
func (r *Repository) ListQueueStatsSummary(ctx context.Context, queueID int64) ([]QueueStatMV, error) {
	var rows []QueueStatMV
	err := r.db.SelectContext(ctx, &rows,
		`SELECT queue_id, stat_date, total_calls, answered_calls, abandoned_calls, avg_wait_seconds, avg_talk_seconds
		 FROM queue_stats_mv WHERE queue_id = ? ORDER BY stat_date DESC`,
		queueID)
	if err != nil {
		return nil, fmt.Errorf("qstats: list queue_stats_mv: %w", err)
	}
	return rows, nil
}

// ListQueueEvents is not currently wired to any handler.
func (r *Repository) ListQueueEvents(ctx context.Context, queueID int64, limit, offset int) ([]QueueEvent, error) {
	var rows []QueueEvent
	err := r.db.SelectContext(ctx, &rows,
		`SELECT id, queue_id, agent_id, event_type, event_time, metadata
		 FROM queue_events WHERE queue_id = ? ORDER BY event_time DESC LIMIT ? OFFSET ?`,
		queueID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("qstats: list queue_events: %w", err)
	}
	return rows, nil
}

// GetRecordingByCallID is not currently wired to any handler. It may be
// needed to enrich the pbx-worker call-detail response with a recording
// URL/path — see internal/handlers/calls.go TODO.
func (r *Repository) GetRecordingByCallID(ctx context.Context, callID string) (*Recording, error) {
	var rec Recording
	err := r.db.GetContext(ctx, &rec,
		`SELECT id, call_id, queue_id, agent_id, file_path, duration_seconds, recorded_at
		 FROM recordings WHERE call_id = ? LIMIT 1`,
		callID)
	if err != nil {
		return nil, fmt.Errorf("qstats: get recording for call %s: %w", callID, err)
	}
	return &rec, nil
}
