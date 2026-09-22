package softphone

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// ErrExtensionInUse is returned when the requested extension number or the
// target user already has a row — matching the two UNIQUE constraints on
// sip_extensions (extension, user_id). See migrations/callcenter/002_sip_extensions.sql
// for why user_id is unique: exactly one extension per user, a real
// constraint in heal-crm's own schema.
var ErrExtensionInUse = errors.New("softphone: extension number or user already has a SIP extension")

// ExtensionRow is one row of sip_extensions, DB-shaped (encrypted password,
// JSON columns as raw text) — see ToExtension for the decrypted, typed view
// callers actually want.
type ExtensionRow struct {
	ID                   string         `db:"id"`
	UserID               string         `db:"user_id"`
	Extension            string         `db:"extension"`
	SIPUsername          string         `db:"sip_username"`
	SIPPasswordEncrypted string         `db:"sip_password_encrypted"`
	DisplayName          sql.NullString `db:"display_name"`
	CIDNames             sql.NullString `db:"cid_names"` // JSON text
	Queues               sql.NullString `db:"queues"`    // JSON text
	IsSupervisor         bool           `db:"is_supervisor"`
	IsDefault            bool           `db:"is_default"`
	LastRegisteredAt     sql.NullTime   `db:"last_registered_at"`
	CreatedAt            time.Time      `db:"created_at"`
	UpdatedAt            time.Time      `db:"updated_at"`
}

// extensionNumberRe mirrors the Livewire form's own validation regex
// exactly: 'extension' => ['required', 'string', 'regex:/^\d{2,10}$/'].
var extensionNumberRe = regexp.MustCompile(`^\d{2,10}$`)

// normalizeQueues ports SipExtension::normalizeQueues(): split on
// |,.;\s, trim, drop empties, dedupe while preserving order.
func normalizeQueues(raw []string) []string {
	out := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	splitter := regexp.MustCompile(`[|,.;\s]+`)
	for _, item := range raw {
		for _, part := range splitter.Split(strings.TrimSpace(item), -1) {
			part = strings.TrimSpace(part)
			if part == "" || seen[part] {
				continue
			}
			seen[part] = true
			out = append(out, part)
		}
	}
	return out
}

// Repository is the real, MySQL-backed replacement for the earlier
// in-memory Store — see migrations/callcenter/002_sip_extensions.sql.
type Repository struct {
	db  *sqlx.DB
	enc *Encryptor
}

func NewRepository(db *sqlx.DB, enc *Encryptor) *Repository {
	return &Repository{db: db, enc: enc}
}

// ToExtension decrypts the password and unmarshals the JSON columns into
// the typed Extension shape the rest of the package works with. Password
// decryption failure is surfaced rather than silently returning an empty
// string, unlike Laravel's own getSipPasswordAttribute() (which swallows
// DecryptException and returns ”) — a silently-empty SIP password fails
// registration downstream in a way that looks like a config problem, not a
// decryption problem, and is a worse debugging experience for something
// that should never happen outside a key rotation gone wrong.
func (r *Repository) ToExtension(row ExtensionRow) (Extension, error) {
	password, err := r.enc.Decrypt(row.SIPPasswordEncrypted)
	if err != nil {
		return Extension{}, fmt.Errorf("softphone: decrypt password for extension %s: %w", row.Extension, err)
	}

	var queues []string
	if row.Queues.Valid && row.Queues.String != "" {
		_ = json.Unmarshal([]byte(row.Queues.String), &queues)
	}
	if queues == nil {
		queues = []string{}
	}

	return Extension{
		Extension:    row.Extension,
		DisplayName:  row.DisplayName.String,
		Password:     password,
		IsDefault:    row.IsDefault,
		IsSupervisor: row.IsSupervisor,
		Queues:       queues,
	}, nil
}

// ForUser returns the user's extension (there is at most one — see the
// UNIQUE constraint), matching the shape ExtensionFor/BootstrapFor need.
func (r *Repository) ForUser(ctx context.Context, userID string) (Extension, bool, error) {
	var row ExtensionRow
	err := r.db.GetContext(ctx, &row, `SELECT * FROM sip_extensions WHERE user_id = ?`, userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Extension{}, false, nil
		}
		return Extension{}, false, fmt.Errorf("softphone: load extension for user %s: %w", userID, err)
	}
	ext, err := r.ToExtension(row)
	if err != nil {
		return Extension{}, false, err
	}
	return ext, true, nil
}

// CreateInput mirrors the Livewire form's save() attributes array.
type CreateInput struct {
	UserID       string
	Extension    string
	SIPUsername  string
	SIPPassword  string // plaintext in, encrypted at rest
	DisplayName  string
	Queues       string // raw comma/pipe/etc-separated string, as the form field holds it
	IsSupervisor bool
	IsDefault    bool
}

// validate mirrors validationRules() in SipExtensions\Index.php. requireUserID
// is false when called from Update (the owning user is fixed by :id, not
// resubmitted — see UpdateInput's own doc comment). requirePassword is true
// only on create; on update an empty password means "keep the existing
// one", so it is validated for LENGTH only when non-empty, on both paths.
func (in CreateInput) validate(requireUserID, requirePassword bool) error {
	if requireUserID && strings.TrimSpace(in.UserID) == "" {
		return errors.New("user_id is required")
	}
	if !extensionNumberRe.MatchString(in.Extension) {
		return errors.New("extension must be 2-10 digits")
	}
	if strings.TrimSpace(in.SIPUsername) == "" || len(in.SIPUsername) > 120 {
		return errors.New("sip_username is required (max 120 characters)")
	}
	if requirePassword && in.SIPPassword == "" {
		return errors.New("sip_password is required")
	}
	if in.SIPPassword != "" && len(in.SIPPassword) < 8 {
		return errors.New("sip_password must be at least 8 characters")
	}
	if len(in.DisplayName) > 120 {
		return errors.New("display_name must be at most 120 characters")
	}
	if len(in.Queues) > 255 {
		return errors.New("queues must be at most 255 characters")
	}
	return nil
}

// Create provisions a new extension. Mirrors SipExtensions\Index::save()
// for the create path plus makeDefault()'s single-default-per-user
// invariant when IsDefault is requested.
func (r *Repository) Create(ctx context.Context, in CreateInput) (string, error) {
	if err := in.validate(true, true); err != nil {
		return "", err
	}

	encPassword, err := r.enc.Encrypt(in.SIPPassword)
	if err != nil {
		return "", fmt.Errorf("softphone: encrypt password: %w", err)
	}
	queuesJSON, _ := json.Marshal(normalizeQueues(strings.Split(in.Queues, ",")))

	id := newUUIDv4()

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("softphone: begin create: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	if in.IsDefault {
		if err := unsetOtherDefaults(ctx, tx, in.UserID, ""); err != nil {
			return "", err
		}
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO sip_extensions
			(id, user_id, extension, sip_username, sip_password_encrypted,
			 display_name, queues, is_supervisor, is_default)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, in.UserID, in.Extension, in.SIPUsername, encPassword,
		nullIfEmpty(in.DisplayName), string(queuesJSON), in.IsSupervisor, in.IsDefault,
	)
	if err != nil {
		if isDuplicateKeyErr(err) {
			return "", ErrExtensionInUse
		}
		return "", fmt.Errorf("softphone: create extension: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", fmt.Errorf("softphone: commit create: %w", err)
	}
	return id, nil
}

// UpdateInput mirrors the Livewire form's save() attributes for the edit
// path. SIPPassword empty means "leave unchanged" (the form does the same:
// `if ($this->editingId && $this->sipPassword === ”) { $this->sipPassword = null; }`).
type UpdateInput struct {
	Extension    string
	SIPUsername  string
	SIPPassword  string
	DisplayName  string
	Queues       string
	IsSupervisor bool
	IsDefault    bool
}

func (r *Repository) Update(ctx context.Context, id string, in UpdateInput) error {
	createIn := CreateInput{
		Extension: in.Extension, SIPUsername: in.SIPUsername, SIPPassword: in.SIPPassword,
		DisplayName: in.DisplayName, Queues: in.Queues,
	}
	if err := createIn.validate(false, false); err != nil {
		return err
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("softphone: begin update: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var userID string
	if err := tx.GetContext(ctx, &userID, `SELECT user_id FROM sip_extensions WHERE id = ?`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoExtension
		}
		return fmt.Errorf("softphone: load extension %s for update: %w", id, err)
	}

	if in.IsDefault {
		if err := unsetOtherDefaults(ctx, tx, userID, id); err != nil {
			return err
		}
	}

	queuesJSON, _ := json.Marshal(normalizeQueues(strings.Split(in.Queues, ",")))

	if in.SIPPassword == "" {
		_, err = tx.ExecContext(ctx, `
			UPDATE sip_extensions SET
				extension = ?, sip_username = ?, display_name = ?, queues = ?,
				is_supervisor = ?, is_default = ?
			WHERE id = ?`,
			in.Extension, in.SIPUsername, nullIfEmpty(in.DisplayName), string(queuesJSON),
			in.IsSupervisor, in.IsDefault, id,
		)
	} else {
		encPassword, encErr := r.enc.Encrypt(in.SIPPassword)
		if encErr != nil {
			return fmt.Errorf("softphone: encrypt password: %w", encErr)
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE sip_extensions SET
				extension = ?, sip_username = ?, sip_password_encrypted = ?,
				display_name = ?, queues = ?, is_supervisor = ?, is_default = ?
			WHERE id = ?`,
			in.Extension, in.SIPUsername, encPassword, nullIfEmpty(in.DisplayName),
			string(queuesJSON), in.IsSupervisor, in.IsDefault, id,
		)
	}
	if err != nil {
		if isDuplicateKeyErr(err) {
			return ErrExtensionInUse
		}
		return fmt.Errorf("softphone: update extension %s: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("softphone: commit update: %w", err)
	}
	return nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM sip_extensions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("softphone: delete extension %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoExtension
	}
	return nil
}

// ListItem is one row for the admin listing — no password, matches
// SipExtensions\Index::sipExtensions()'s selected columns plus the owning
// user id (this service has no `users` table to join against; the caller
// resolves display info for user_id itself — see handlers/sipextensions.go).
type ListItem struct {
	ID           string         `db:"id" json:"id"`
	UserID       string         `db:"user_id" json:"user_id"`
	Extension    string         `db:"extension" json:"extension"`
	DisplayName  string         `db:"display_name" json:"display_name"`
	IsSupervisor bool           `db:"is_supervisor" json:"is_supervisor"`
	IsDefault    bool           `db:"is_default" json:"is_default"`
	Queues       []string       `db:"-" json:"queues"`
	QueuesRaw    sql.NullString `db:"queues" json:"-"`
}

// List mirrors SipExtensions\Index::sipExtensions(): search across
// extension/display_name (no user email search here — no user directory
// in this service, see ListItem's own doc comment), ordered default-first
// then by extension, paginated.
func (r *Repository) List(ctx context.Context, search string, page, perPage int) ([]ListItem, int, error) {
	if page < 1 {
		page = 1
	}
	if perPage < 1 || perPage > 100 {
		perPage = 15 // matches the Livewire paginate(15)
	}

	where := ""
	args := []any{}
	if search != "" {
		where = `WHERE extension LIKE ? OR display_name LIKE ?`
		like := "%" + search + "%"
		args = append(args, like, like)
	}

	var total int
	countQuery := `SELECT COUNT(*) FROM sip_extensions ` + where
	if err := r.db.GetContext(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, fmt.Errorf("softphone: count extensions: %w", err)
	}

	query := `SELECT id, user_id, extension, display_name, is_supervisor, is_default, queues
	          FROM sip_extensions ` + where + `
	          ORDER BY is_default DESC, extension ASC
	          LIMIT ? OFFSET ?`
	args = append(args, perPage, (page-1)*perPage)

	var rows []ListItem
	if err := r.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, 0, fmt.Errorf("softphone: list extensions: %w", err)
	}
	for i := range rows {
		if rows[i].QueuesRaw.Valid && rows[i].QueuesRaw.String != "" {
			_ = json.Unmarshal([]byte(rows[i].QueuesRaw.String), &rows[i].Queues)
		}
		if rows[i].Queues == nil {
			rows[i].Queues = []string{}
		}
	}

	return rows, total, nil
}

// unsetOtherDefaults ports SipExtension::makeDefault()'s invariant: only
// one default extension per user. excludeID is the row currently being
// written (empty on create), so it is never excluded from itself needing
// clearing on an update that flips is_default on a row that was not
// already the default.
func unsetOtherDefaults(ctx context.Context, tx *sqlx.Tx, userID, excludeID string) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE sip_extensions SET is_default = 0 WHERE user_id = ? AND id != ?`,
		userID, excludeID)
	if err != nil {
		return fmt.Errorf("softphone: clear other defaults for user %s: %w", userID, err)
	}
	return nil
}

func nullIfEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func isDuplicateKeyErr(err error) bool {
	// go-sql-driver/mysql wraps duplicate-key violations in a *mysql.MySQLError
	// with number 1062; string-matching keeps this file free of a driver
	// import for one error code.
	return err != nil && strings.Contains(err.Error(), "Error 1062")
}

// newUUIDv4 is the same RFC 4122 v4 generator as internal/calllog's —
// duplicated rather than shared across packages for one six-line function,
// to avoid a cross-package dependency neither otherwise needs.
func newUUIDv4() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("softphone: failed to read random bytes for uuid: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
