// Package queuegroups is named sets of queues an admin can label and
// reorder — mirrors Modules/CallCenter/app/Models/QueueGroup.php. See
// migrations/callcenter/005_queue_groups.sql's own doc comment: this is
// organisational metadata only, nothing in this service's reports groups
// BY queue group yet.
package queuegroups

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("queuegroups: not found")

// ListItem is one row plus its member queue names.
type ListItem struct {
	ID          string   `db:"id" json:"id"`
	Name        string   `db:"name" json:"name"`
	Description string   `db:"description" json:"description"`
	IsActive    bool     `db:"is_active" json:"is_active"`
	SortOrder   int      `db:"sort_order" json:"sort_order"`
	Queues      []string `db:"-" json:"queues"`
}

type row struct {
	ID          string         `db:"id"`
	Name        string         `db:"name"`
	Description sql.NullString `db:"description"`
	IsActive    bool           `db:"is_active"`
	SortOrder   int            `db:"sort_order"`
}

type Input struct {
	Name        string
	Description string
	IsActive    bool
	SortOrder   int
	Queues      []string
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// List mirrors QueueGroups\Index: search by name, ordered by sort_order
// then name, paginated.
func (r *Repository) List(ctx context.Context, search string, page, perPage int) ([]ListItem, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 10
	}

	where := ""
	args := []any{}
	if search != "" {
		where = `WHERE name LIKE ?`
		args = append(args, "%"+search+"%")
	}

	var total int
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM call_center_queue_groups `+where, args...); err != nil {
		return nil, 0, fmt.Errorf("queuegroups: count: %w", err)
	}

	query := `SELECT id, name, description, is_active, sort_order FROM call_center_queue_groups ` +
		where + ` ORDER BY sort_order ASC, name ASC LIMIT ? OFFSET ?`
	args = append(args, perPage, (page-1)*perPage)

	var rows []row
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, 0, fmt.Errorf("queuegroups: list: %w", err)
	}

	out := make([]ListItem, 0, len(rows))
	for _, rw := range rows {
		queues, err := r.queuesFor(ctx, rw.ID)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, ListItem{
			ID: rw.ID, Name: rw.Name, Description: rw.Description.String,
			IsActive: rw.IsActive, SortOrder: rw.SortOrder, Queues: queues,
		})
	}
	return out, total, nil
}

func (r *Repository) queuesFor(ctx context.Context, groupID string) ([]string, error) {
	queues := []string{}
	if err := r.db.SelectContext(ctx, &queues,
		`SELECT queue_name FROM call_center_queue_group_queues WHERE queue_group_id = ? ORDER BY queue_name`,
		groupID); err != nil {
		return nil, fmt.Errorf("queuegroups: queues for %s: %w", groupID, err)
	}
	return queues, nil
}

// Create inserts the group and its queue memberships in one transaction.
func (r *Repository) Create(ctx context.Context, in Input) (string, error) {
	id := newID()

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("queuegroups: begin create: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO call_center_queue_groups (id, name, description, is_active, sort_order) VALUES (?, ?, ?, ?, ?)`,
		id, in.Name, nullIfEmpty(in.Description), in.IsActive, in.SortOrder); err != nil {
		return "", fmt.Errorf("queuegroups: create: %w", err)
	}
	if err := insertQueues(ctx, tx, id, in.Queues); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("queuegroups: commit create: %w", err)
	}
	return id, nil
}

// Update replaces the group's fields AND its whole queue membership set —
// same delete-then-recreate semantics as QueueGroup::syncQueues().
func (r *Repository) Update(ctx context.Context, id string, in Input) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("queuegroups: begin update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx,
		`UPDATE call_center_queue_groups SET name = ?, description = ?, is_active = ?, sort_order = ? WHERE id = ?`,
		in.Name, nullIfEmpty(in.Description), in.IsActive, in.SortOrder, id)
	if err != nil {
		return fmt.Errorf("queuegroups: update %s: %w", id, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM call_center_queue_group_queues WHERE queue_group_id = ?`, id); err != nil {
		return fmt.Errorf("queuegroups: clear queues for %s: %w", id, err)
	}
	if err := insertQueues(ctx, tx, id, in.Queues); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("queuegroups: commit update: %w", err)
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM call_center_queue_groups WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("queuegroups: delete %s: %w", id, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func insertQueues(ctx context.Context, tx *sqlx.Tx, groupID string, queues []string) error {
	for _, q := range queues {
		if q == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO call_center_queue_group_queues (id, queue_group_id, queue_name) VALUES (?, ?, ?)`,
			newID(), groupID, q); err != nil {
			return fmt.Errorf("queuegroups: insert queue %s for %s: %w", q, groupID, err)
		}
	}
	return nil
}

func nullIfEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
