// Package scheduledreports stores scheduled-report DEFINITIONS —
// mirrors Modules/CallCenter/app/Models/ScheduledReport.php's fillable
// columns. Storage only: see
// migrations/callcenter/006_scheduled_reports.sql's own doc comment —
// there is no email sender and no cron/scheduler process in this Go
// service to actually execute a row. A row here is a saved intent.
package scheduledreports

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("scheduledreports: not found")

type ListItem struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	DestinationEmail string   `json:"destination_email"`
	Reports          []string `json:"reports"`
	Queues           []string `json:"queues"`
	LastDays         int      `json:"last_days"`
	CronDayMonth     string   `json:"cron_day_month"`
	CronDayWeek      string   `json:"cron_day_week"`
	CronHour         string   `json:"cron_hour"`
	CronMinute       string   `json:"cron_minute"`
	IsActive         bool     `json:"is_active"`
}

type row struct {
	ID               string `db:"id"`
	Name             string `db:"name"`
	DestinationEmail string `db:"destination_email"`
	Reports          string `db:"reports"` // JSON text
	Queues           []byte `db:"queues"`  // JSON text, nullable
	LastDays         int    `db:"last_days"`
	CronDayMonth     string `db:"cron_day_month"`
	CronDayWeek      string `db:"cron_day_week"`
	CronHour         string `db:"cron_hour"`
	CronMinute       string `db:"cron_minute"`
	IsActive         bool   `db:"is_active"`
}

func (r row) toItem() ListItem {
	item := ListItem{
		ID: r.ID, Name: r.Name, DestinationEmail: r.DestinationEmail,
		LastDays: r.LastDays, CronDayMonth: r.CronDayMonth, CronDayWeek: r.CronDayWeek,
		CronHour: r.CronHour, CronMinute: r.CronMinute, IsActive: r.IsActive,
		Reports: []string{}, Queues: []string{},
	}
	_ = json.Unmarshal([]byte(r.Reports), &item.Reports)
	if len(r.Queues) > 0 {
		_ = json.Unmarshal(r.Queues, &item.Queues)
	}
	if item.Reports == nil {
		item.Reports = []string{}
	}
	if item.Queues == nil {
		item.Queues = []string{}
	}
	return item
}

type Input struct {
	Name             string
	DestinationEmail string
	Reports          []string
	Queues           []string
	LastDays         int
	CronDayMonth     string
	CronDayWeek      string
	CronHour         string
	CronMinute       string
	IsActive         bool
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

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
		where = `WHERE name LIKE ? OR destination_email LIKE ?`
		like := "%" + search + "%"
		args = append(args, like, like)
	}

	var total int
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM call_center_scheduled_reports `+where, args...); err != nil {
		return nil, 0, fmt.Errorf("scheduledreports: count: %w", err)
	}

	query := `SELECT id, name, destination_email, reports, queues, last_days,
	                 cron_day_month, cron_day_week, cron_hour, cron_minute, is_active
	          FROM call_center_scheduled_reports ` + where + `
	          ORDER BY name ASC LIMIT ? OFFSET ?`
	args = append(args, perPage, (page-1)*perPage)

	var rows []row
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, 0, fmt.Errorf("scheduledreports: list: %w", err)
	}

	out := make([]ListItem, 0, len(rows))
	for _, rw := range rows {
		out = append(out, rw.toItem())
	}
	return out, total, nil
}

func (r *Repository) Create(ctx context.Context, in Input) (string, error) {
	id := newID()
	reportsJSON, err := json.Marshal(nonNil(in.Reports))
	if err != nil {
		return "", fmt.Errorf("scheduledreports: encode reports: %w", err)
	}
	queuesJSON, err := json.Marshal(nonNil(in.Queues))
	if err != nil {
		return "", fmt.Errorf("scheduledreports: encode queues: %w", err)
	}

	_, err = r.db.ExecContext(ctx,
		`INSERT INTO call_center_scheduled_reports
		 (id, name, destination_email, reports, queues, last_days,
		  cron_day_month, cron_day_week, cron_hour, cron_minute, is_active)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.Name, in.DestinationEmail, string(reportsJSON), string(queuesJSON), in.LastDays,
		defaultStar(in.CronDayMonth), defaultStar(in.CronDayWeek), in.CronHour, in.CronMinute, in.IsActive)
	if err != nil {
		return "", fmt.Errorf("scheduledreports: create: %w", err)
	}
	return id, nil
}

func (r *Repository) Update(ctx context.Context, id string, in Input) error {
	reportsJSON, err := json.Marshal(nonNil(in.Reports))
	if err != nil {
		return fmt.Errorf("scheduledreports: encode reports: %w", err)
	}
	queuesJSON, err := json.Marshal(nonNil(in.Queues))
	if err != nil {
		return fmt.Errorf("scheduledreports: encode queues: %w", err)
	}

	result, err := r.db.ExecContext(ctx,
		`UPDATE call_center_scheduled_reports SET
		   name = ?, destination_email = ?, reports = ?, queues = ?, last_days = ?,
		   cron_day_month = ?, cron_day_week = ?, cron_hour = ?, cron_minute = ?, is_active = ?
		 WHERE id = ?`,
		in.Name, in.DestinationEmail, string(reportsJSON), string(queuesJSON), in.LastDays,
		defaultStar(in.CronDayMonth), defaultStar(in.CronDayWeek), in.CronHour, in.CronMinute, in.IsActive, id)
	if err != nil {
		return fmt.Errorf("scheduledreports: update %s: %w", id, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM call_center_scheduled_reports WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("scheduledreports: delete %s: %w", id, err)
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func defaultStar(s string) string {
	if s == "" {
		return "*"
	}
	return s
}

func newID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
