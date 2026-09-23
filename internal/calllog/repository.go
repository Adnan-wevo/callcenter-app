package calllog

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
)

var ErrNotFound = errors.New("calllog: not found")

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{db: db}
}

// CreateInput mirrors the $data array createCallLog() takes from the
// browser. Direction is the RAW value the browser sends
// ('incoming'|'queue'|'outgoing'|...); normalizeDirection below applies
// Softphone.php's own mapping.
type CreateInput struct {
	CallerID    string
	CallerName  string
	Queue       string
	Direction   string
	Channel     string
	UniqueID    string
	Destination string
	UserID      string // empty if unauthenticated (should not happen behind JWTAuth, but mirrors auth()->user() being nullable in the PHP)
	Extension   string
}

// normalizeDirection mirrors createCallLog()'s own normalisation exactly:
// incoming/queue -> inbound, outgoing -> outbound, anything else passed
// through as-is (the PHP does the same — an unrecognised direction string
// is written verbatim rather than rejected).
func normalizeDirection(raw string) string {
	switch raw {
	case "incoming", "queue":
		return DirectionInbound
	case "outgoing":
		return DirectionOutbound
	case "":
		return DirectionInbound // PHP's array default ($data['direction'] ?? 'inbound')
	default:
		return raw
	}
}

// CreateOrMerge is createCallLog() ported directly: try to find an
// in-flight row for the SAME call (by unique_id, then by channel within
// 90s, then by caller_id[+queue] within 30s among ongoing/answered rows);
// if found, merge the browser-only fields in; otherwise insert a new row.
// Returns the row's id either way.
func (r *Repository) CreateOrMerge(ctx context.Context, in CreateInput) (string, error) {
	direction := normalizeDirection(in.Direction)

	existing, err := r.findExistingForCreate(ctx, in)
	if err != nil {
		return "", err
	}

	if existing != nil {
		if err := r.mergeBrowserFields(ctx, existing, in); err != nil {
			return "", err
		}
		return existing.ID, nil
	}

	id := newUUIDv4()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO call_logs
			(id, user_id, extension, call_id, unique_id, channel, direction,
			 caller_id, caller_name, destination, queue, status, started_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id,
		nullIfEmpty(in.UserID),
		nullIfEmpty(in.Extension),
		nullIfEmpty(in.UniqueID),
		nullIfEmpty(in.UniqueID),
		nullIfEmpty(in.Channel),
		direction,
		nullIfEmpty(in.CallerID),
		in.CallerName,
		nullIfEmpty(in.Destination),
		nullIfEmpty(in.Queue),
		StatusOngoing,
		time.Now().UTC(),
		"{}",
	)
	if err != nil {
		return "", fmt.Errorf("calllog: create: %w", err)
	}
	return id, nil
}

// findExistingForCreate implements createCallLog()'s three match rules, in
// the same priority order the PHP tries them.
func (r *Repository) findExistingForCreate(ctx context.Context, in CreateInput) (*CallLog, error) {
	// Match 1: by unique_id (most reliable).
	if in.UniqueID != "" {
		var row CallLog
		err := r.db.GetContext(ctx, &row, `SELECT * FROM call_logs WHERE unique_id = ? LIMIT 1`, in.UniqueID)
		if err == nil {
			return &row, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("calllog: match by unique_id: %w", err)
		}
	}

	// Match 1b: by channel, started within the last 90s.
	if in.Channel != "" {
		var row CallLog
		err := r.db.GetContext(ctx, &row, `
			SELECT * FROM call_logs
			WHERE channel = ? AND started_at >= ?
			ORDER BY started_at DESC LIMIT 1`,
			in.Channel, time.Now().UTC().Add(-90*time.Second))
		if err == nil {
			return &row, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("calllog: match by channel: %w", err)
		}
	}

	// Match 2: by caller_id (+queue if given), ongoing/answered, within 30s.
	if in.CallerID != "" {
		args := []any{StatusOngoing, StatusAnswered, in.CallerID, time.Now().UTC().Add(-30 * time.Second)}
		query := `
			SELECT * FROM call_logs
			WHERE status IN (?, ?) AND caller_id = ? AND started_at >= ?`
		if in.Queue != "" {
			query += ` AND queue = ?`
			args = append(args, in.Queue)
		}
		query += ` ORDER BY started_at DESC LIMIT 1`

		var row CallLog
		err := r.db.GetContext(ctx, &row, query, args...)
		if err == nil {
			return &row, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("calllog: match by caller_id: %w", err)
		}
	}

	return nil, nil
}

// mergeBrowserFields ports createCallLog()'s array_filter merge: each field
// takes the new value if non-empty, otherwise keeps whatever the existing
// row already had — never overwrites a real value with a blank one.
func (r *Repository) mergeBrowserFields(ctx context.Context, existing *CallLog, in CreateInput) error {
	extension := coalesce(in.Extension, existing.Extension.String)
	userID := coalesce(in.UserID, existing.UserID.String)
	destination := coalesce(in.Destination, existing.Destination.String)
	callerName := coalesce(in.CallerName, existing.CallerName.String)
	queue := coalesce(in.Queue, existing.Queue.String)
	uniqueID := coalesce(in.UniqueID, existing.UniqueID.String)
	channel := coalesce(in.Channel, existing.Channel.String)

	_, err := r.db.ExecContext(ctx, `
		UPDATE call_logs SET
			extension = ?, user_id = ?, destination = ?, caller_name = ?,
			queue = ?, call_id = ?, unique_id = ?, channel = ?
		WHERE id = ?`,
		nullIfEmpty(extension), nullIfEmpty(userID), nullIfEmpty(destination), callerName,
		nullIfEmpty(queue), nullIfEmpty(uniqueID), nullIfEmpty(uniqueID), nullIfEmpty(channel),
		existing.ID,
	)
	if err != nil {
		return fmt.Errorf("calllog: merge existing %s: %w", existing.ID, err)
	}
	return nil
}

// MarkAnswered is markCallAnswered() ported directly.
func (r *Repository) MarkAnswered(ctx context.Context, id string, extension, userID string) error {
	var existing CallLog
	if err := r.db.GetContext(ctx, &existing, `SELECT * FROM call_logs WHERE id = ?`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// PHP: silently returns if the call log id is unknown — a
			// answer event for a call log the browser already lost track of
			// is not an error condition worth surfacing.
			return nil
		}
		return fmt.Errorf("calllog: load %s for answer: %w", id, err)
	}

	ext := coalesce(extension, existing.Extension.String)
	uid := coalesce(userID, existing.UserID.String)

	_, err := r.db.ExecContext(ctx, `
		UPDATE call_logs SET status = ?, answered_at = ?, extension = ?, user_id = ?
		WHERE id = ?`,
		StatusAnswered, time.Now().UTC(), nullIfEmpty(ext), nullIfEmpty(uid), id)
	if err != nil {
		return fmt.Errorf("calllog: mark answered %s: %w", id, err)
	}
	return nil
}

// FinalizeInput mirrors finalizeCallLog()'s $data array.
type FinalizeInput struct {
	Status      string // optional; "" means "let the server decide" (matches PHP's $data['status'] ?? null)
	Duration    int    // talk seconds, as reported by the browser
	WaitSeconds int
	Delete      bool // _delete: true means "this queue call was never answered, drop the row"
}

// Finalize is finalizeCallLog() ported directly — including its safety net:
// a call that WAS answered can never be downgraded to missed/abandoned by
// a late or out-of-order browser event.
func (r *Repository) Finalize(ctx context.Context, id string, in FinalizeInput) error {
	var existing CallLog
	if err := r.db.GetContext(ctx, &existing, `SELECT * FROM call_logs WHERE id = ?`, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil // same "unknown id is not an error" reasoning as MarkAnswered.
		}
		return fmt.Errorf("calllog: load %s for finalize: %w", id, err)
	}

	wasAnswered := existing.AnsweredAt.Valid

	if in.Delete && !wasAnswered {
		_, err := r.db.ExecContext(ctx, `DELETE FROM call_logs WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("calllog: delete unanswered %s: %w", id, err)
		}
		return nil
	}

	var status string
	if wasAnswered && (in.Status == "" || in.Status == StatusMissed || in.Status == StatusAbandoned) {
		// Never let a call that was genuinely answered end up recorded as
		// missed/abandoned just because the browser's finalize event raced
		// ahead of (or lost track of) the fact it was answered.
		status = StatusCompleted
	} else if in.Status != "" {
		status = in.Status
	} else if wasAnswered {
		status = StatusCompleted
	} else {
		status = StatusMissed
	}

	talkSeconds := in.Duration
	if talkSeconds <= 0 && wasAnswered {
		talkSeconds = int(time.Since(existing.AnsweredAt.Time).Seconds())
		if talkSeconds < 0 {
			talkSeconds = 0
		}
	}

	waitSeconds := in.WaitSeconds
	if waitSeconds == 0 {
		waitSeconds = existing.WaitSeconds
	}

	_, err := r.db.ExecContext(ctx, `
		UPDATE call_logs SET status = ?, talk_seconds = ?, wait_seconds = ?, ended_at = ?
		WHERE id = ?`,
		status, talkSeconds, waitSeconds, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("calllog: finalize %s: %w", id, err)
	}
	return nil
}

// DetectQueueResult is detectQueue()'s return shape.
type DetectQueueResult struct {
	Queue     string
	CallLogID string
}

// DetectQueue implements the DB-backed subset of detectQueue()'s fallback
// chain: match by unique_id, then by channel, then by caller_id (all
// within recent windows, same as the PHP). Two of the PHP's five layers
// are NOT implemented here and are the caller's responsibility to add if
// needed:
//
//   - Layer 0 (webhook hint cache) and layer 4 (DispositionTracker) depend
//     on subsystems this service does not have yet — a webhook ingestion
//     pipeline and a queue-disposition tracker, respectively. Neither
//     exists in this product yet.
//   - Layer 2 (caller_id == current agent's own extension, for the
//     agent-leg case) needs the calling agent's identity, which this
//     package deliberately does not depend on — see handlers/calllog.go,
//     which can add that check itself before calling DetectQueue if needed.
//
// Layer 3 (the PBX worker's live queue snapshot) IS implemented, via the
// optional livePBX parameter — pass nil to skip it (e.g. in a test).
func (r *Repository) DetectQueue(ctx context.Context, callerID string, callLogID, uniqueID, channel *string, liveQueueLookup func(callerID string) (queue string, ok bool)) (DetectQueueResult, error) {
	result := DetectQueueResult{}
	if callLogID != nil {
		result.CallLogID = *callLogID
	}

	// 1. By unique_id.
	if uniqueID != nil && *uniqueID != "" {
		if row, err := r.findWithQueue(ctx, `unique_id = ? AND queue IS NOT NULL AND queue != '' AND started_at >= ?`,
			*uniqueID, time.Now().UTC().Add(-2*time.Minute)); err != nil {
			return result, err
		} else if row != nil {
			result.Queue = row.Queue.String
			if result.CallLogID == "" {
				result.CallLogID = row.ID
			}
		}
	}

	// 1b. By channel.
	if result.Queue == "" && channel != nil && *channel != "" {
		if row, err := r.findWithQueue(ctx, `channel = ? AND queue IS NOT NULL AND queue != '' AND started_at >= ?`,
			*channel, time.Now().UTC().Add(-2*time.Minute)); err != nil {
			return result, err
		} else if row != nil {
			result.Queue = row.Queue.String
			if result.CallLogID == "" {
				result.CallLogID = row.ID
			}
		}
	}

	// 3. Live PBX worker queue snapshot (optional).
	if result.Queue == "" && callerID != "" && liveQueueLookup != nil {
		if q, ok := liveQueueLookup(callerID); ok {
			result.Queue = q
		}
	}

	// 5. By caller_id, recent, queue set (not limited to ongoing/answered —
	// the browser may have already finalized the call before this runs).
	if result.Queue == "" && callerID != "" {
		if row, err := r.findWithQueue(ctx, `caller_id = ? AND queue IS NOT NULL AND queue != '' AND started_at >= ?`,
			callerID, time.Now().UTC().Add(-30*time.Second)); err != nil {
			return result, err
		} else if row != nil {
			result.Queue = row.Queue.String
			if result.CallLogID == "" {
				result.CallLogID = row.ID
			}
		}
	}

	// If a queue was found and we have a call log id whose queue is
	// currently unset, backfill it — mirrors the PHP's own conditional
	// UPDATE ... WHERE queue IS NULL OR queue = ''.
	if result.Queue != "" && result.CallLogID != "" {
		_, err := r.db.ExecContext(ctx, `
			UPDATE call_logs SET queue = ?
			WHERE id = ? AND (queue IS NULL OR queue = '')`,
			result.Queue, result.CallLogID)
		if err != nil {
			return result, fmt.Errorf("calllog: backfill queue for %s: %w", result.CallLogID, err)
		}
	}

	return result, nil
}

func (r *Repository) findWithQueue(ctx context.Context, whereClause string, args ...any) (*CallLog, error) {
	var row CallLog
	query := `SELECT * FROM call_logs WHERE ` + whereClause + ` ORDER BY started_at DESC LIMIT 1`
	err := r.db.GetContext(ctx, &row, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("calllog: find with queue: %w", err)
	}
	return &row, nil
}

// ListForExtension is the softphone panel's "History" tab: an agent's own
// past calls, newest first. Pagination is done here in SQL (LIMIT/OFFSET)
// rather than the paginate-in-memory helper handlers/handlers.go uses for
// pbx-worker report rows — those come back as a whole window with no
// pagination of their own; this table is this service's, so the database
// does the work instead of pulling every row an agent has ever placed into
// memory to slice it.
func (r *Repository) ListForExtension(ctx context.Context, extension string, page, perPage int) ([]CallLog, int, error) {
	var total int
	if err := r.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM call_logs WHERE extension = ?`, extension); err != nil {
		return nil, 0, fmt.Errorf("calllog: count for extension %s: %w", extension, err)
	}

	rows := []CallLog{}
	if total > 0 {
		offset := (page - 1) * perPage
		err := r.db.SelectContext(ctx, &rows,
			`SELECT * FROM call_logs WHERE extension = ? ORDER BY started_at DESC, created_at DESC LIMIT ? OFFSET ?`,
			extension, perPage, offset)
		if err != nil {
			return nil, 0, fmt.Errorf("calllog: list for extension %s: %w", extension, err)
		}
	}
	return rows, total, nil
}

// Get fetches one row by id.
func (r *Repository) Get(ctx context.Context, id string) (*CallLog, error) {
	var row CallLog
	err := r.db.GetContext(ctx, &row, `SELECT * FROM call_logs WHERE id = ?`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("calllog: get %s: %w", id, err)
	}
	return &row, nil
}

func coalesce(newVal, existingVal string) string {
	if newVal != "" {
		return newVal
	}
	return existingVal
}

func nullIfEmpty(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
