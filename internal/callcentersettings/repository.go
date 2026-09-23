// Package callcentersettings is this product's own configurable settings —
// resolving open decision D3 in internal/reports/settings.go's own doc
// comment ("this service cannot read that table yet"). Same
// category/setting_key/value shape as heal-crm's call_center_settings,
// but seeded with only the keys internal/reports.Settings actually
// consumes — see migrations/callcenter/004_call_center_settings.sql's own
// comment on why the rest of heal-crm's categories aren't here.
package callcentersettings

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type Row struct {
	Category    string         `db:"category" json:"category"`
	Key         string         `db:"setting_key" json:"key"`
	Value       sql.NullString `db:"value" json:"-"`
	Description sql.NullString `db:"description" json:"description"`
}

// ValueString never null over the wire — an unset setting is an empty
// string, not a JSON null a form has to guard against.
func (r Row) ValueString() string {
	return r.Value.String
}

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// Categories lists the distinct categories, alphabetically.
func (r *Repository) Categories(ctx context.Context) ([]string, error) {
	var cats []string
	if err := r.db.SelectContext(ctx, &cats,
		`SELECT DISTINCT category FROM call_center_settings ORDER BY category`); err != nil {
		return nil, fmt.Errorf("callcentersettings: list categories: %w", err)
	}
	return cats, nil
}

// ListCategory returns every row in one category, ordered by key.
func (r *Repository) ListCategory(ctx context.Context, category string) ([]Row, error) {
	var rows []Row
	if err := r.db.SelectContext(ctx, &rows,
		`SELECT category, setting_key, value, description FROM call_center_settings
		 WHERE category = ? ORDER BY setting_key`, category); err != nil {
		return nil, fmt.Errorf("callcentersettings: list category %s: %w", category, err)
	}
	return rows, nil
}

// SetCategory upserts every key in values for that category. Only keys
// that already exist as a row are touched — matching heal-crm's own
// updateOrCreate-per-existing-row behaviour, this never invents a new
// setting key from a client-supplied name.
func (r *Repository) SetCategory(ctx context.Context, category string, values map[string]string) error {
	existing, err := r.ListCategory(ctx, category)
	if err != nil {
		return err
	}
	known := make(map[string]bool, len(existing))
	for _, row := range existing {
		known[row.Key] = true
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("callcentersettings: begin update: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for key, value := range values {
		if !known[key] {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE call_center_settings SET value = ? WHERE category = ? AND setting_key = ?`,
			value, category, key); err != nil {
			return fmt.Errorf("callcentersettings: update %s.%s: %w", category, key, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("callcentersettings: commit update: %w", err)
	}
	return nil
}

// GetInt reads one value as an int, falling back when the row is missing,
// empty or non-numeric — the seam internal/reports.Settings's real
// loader (see handlers.Handlers.loadReportSettings) reads through, so a
// bad or absent value degrades to the documented default rather than
// breaking every report.
func (r *Repository) GetInt(ctx context.Context, category, key string, fallback int) int {
	var value sql.NullString
	err := r.db.GetContext(ctx, &value,
		`SELECT value FROM call_center_settings WHERE category = ? AND setting_key = ?`, category, key)
	if err != nil || !value.Valid || value.String == "" {
		return fallback
	}
	n := 0
	for _, c := range value.String {
		if c < '0' || c > '9' {
			return fallback
		}
		n = n*10 + int(c-'0')
	}
	return n
}
