package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Mickyj70/sky/internal/storage"
	"github.com/Mickyj70/sky/internal/tracking"
)

const MaxLimit = 1000
const MaxOffset = 1000000
const MaxPageBytes = 16 << 20

// Filter bounds use first local encounter time, inclusive Since/exclusive Until.
// Offset pagination is stable on an unchanged database, ordered by time then key.
type Filter struct {
	Kind          storage.Kind
	Identifier    string
	Since, Until  *time.Time
	Limit, Offset int
}

func validTime(t time.Time) bool { return !t.IsZero() && time.Unix(0, t.UnixNano()).Equal(t) }
func (f Filter) Validate() (Filter, error) {
	if f.Limit < 1 || f.Limit > MaxLimit || f.Offset < 0 || f.Offset > MaxOffset {
		return f, fmt.Errorf("history limit must be 1..%d and offset 0..%d", MaxLimit, MaxOffset)
	}
	if f.Kind != "" || f.Identifier != "" {
		id, err := storage.Normalize(f.Kind, f.Identifier)
		if err != nil {
			return f, err
		}
		f.Identifier = id
	}
	for _, t := range []*time.Time{f.Since, f.Until} {
		if t != nil && !validTime(*t) {
			return f, fmt.Errorf("history bound outside supported nanosecond range")
		}
	}
	if f.Since != nil && f.Until != nil && !f.Since.Before(*f.Until) {
		return f, fmt.Errorf("since must be before until")
	}
	return f, nil
}
func (f Filter) where() (string, []any) {
	where := " WHERE 1=1"
	args := []any{}
	if f.Kind != "" {
		where += " AND kind=? AND identifier=?"
		args = append(args, f.Kind, f.Identifier)
	}
	if f.Since != nil {
		where += " AND at_ns>=?"
		args = append(args, f.Since.UnixNano())
	}
	if f.Until != nil {
		where += " AND at_ns<?"
		args = append(args, f.Until.UnixNano())
	}
	return where, args
}

type Record struct {
	Key         string               `json:"key"`
	Observation tracking.Observation `json:"observation"`
}
type Attribution struct {
	Source  string `json:"source"`
	License string `json:"license"`
	URL     string `json:"url,omitempty"`
}

func SourceAttribution(source string) Attribution {
	if source == "adsb.lol" {
		return Attribution{source, "ODbL 1.0", "https://www.adsb.lol/docs/open-data/api"}
	}
	return Attribution{Source: source, License: "See provider terms"}
}

type Page struct {
	Collection  string        `json:"collection"`
	Since       *time.Time    `json:"since,omitempty"`
	Until       *time.Time    `json:"until,omitempty"`
	Records     []Record      `json:"records"`
	Attribution []Attribution `json:"attribution"`
	Limit       int           `json:"limit"`
	Offset      int           `json:"offset"`
	HasMore     bool          `json:"has_more"`
	NextOffset  *int          `json:"next_offset,omitempty"`
}

// scanRecord also serves backup import, so query and restore reject the same
// corrupt row metadata/payload identities instead of silently exporting them.
func scanRecord(row interface{ Scan(...any) error }) (Record, []byte, error) {
	var key, kind, id, source, payload string
	var stamp int64
	if err := row.Scan(&key, &kind, &id, &source, &stamp, &payload); err != nil {
		return Record{}, nil, err
	}
	var o tracking.Observation
	if err := json.Unmarshal([]byte(payload), &o); err != nil {
		return Record{}, nil, fmt.Errorf("corrupt history payload: %w", err)
	}
	normalized, want, encoded, err := encode(o)
	if err != nil {
		return Record{}, nil, fmt.Errorf("corrupt history observation: %w", err)
	}
	if want != key || kind != string(normalized.Target.Kind) || id != normalized.Target.Identifier || source != normalized.Result.Source || stamp != normalized.At.UnixNano() {
		return Record{}, nil, fmt.Errorf("history row identity mismatch")
	}
	return Record{key, normalized}, encoded, nil
}

func (s *Store) Query(ctx context.Context, f Filter) (Page, error) {
	f, err := f.Validate()
	if err != nil {
		return Page{}, err
	}
	page := Page{Collection: "local", Records: []Record{}, Attribution: []Attribution{}, Limit: f.Limit, Offset: f.Offset, Since: f.Since, Until: f.Until}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Page{}, err
	}
	defer tx.Rollback()
	if err = validate(ctx, tx); err != nil {
		return Page{}, err
	}
	where, args := f.where()
	args = append(args, f.Limit+1, f.Offset)
	rows, err := tx.QueryContext(ctx, "SELECT key,kind,identifier,source,at_ns,payload FROM observations"+where+" ORDER BY at_ns,key LIMIT ? OFFSET ?", args...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	size := 0
	seen := map[string]bool{}
	for rows.Next() {
		record, payload, err := scanRecord(rows)
		if err != nil {
			return Page{}, err
		}
		if len(page.Records) == f.Limit {
			page.HasMore = true
			if f.Offset+f.Limit <= MaxOffset {
				next := f.Offset + f.Limit
				page.NextOffset = &next
			}
			break
		}
		size += len(payload)
		if size > MaxPageBytes {
			return Page{}, fmt.Errorf("history page exceeds %d bytes; lower --limit", MaxPageBytes)
		}
		page.Records = append(page.Records, record)
		source := record.Observation.Result.Source
		if !seen[source] {
			page.Attribution = append(page.Attribution, SourceAttribution(source))
			seen[source] = true
		}
	}
	if err = rows.Err(); err != nil {
		return Page{}, err
	}
	if err = rows.Close(); err != nil {
		return Page{}, err
	}
	if err = tx.Commit(); err != nil {
		return Page{}, err
	}
	return page, nil
}

type PruneResult struct {
	Database string    `json:"database"`
	Before   time.Time `json:"before"`
	Applied  bool      `json:"applied"`
	Matched  int64     `json:"matched"`
	Deleted  int64     `json:"deleted"`
}

// Prune previews by default; apply deletes in the same immediate transaction as
// its count. Watchlist data and provider snapshot times do not affect selection.
func (s *Store) Prune(ctx context.Context, before time.Time, apply bool) (PruneResult, error) {
	if !validTime(before) {
		return PruneResult{}, fmt.Errorf("invalid history prune cutoff")
	}
	result := PruneResult{Database: s.path, Before: before.UTC(), Applied: apply}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: !apply})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	if err = validate(ctx, tx); err != nil {
		return result, err
	}
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM observations WHERE at_ns<?", before.UnixNano()).Scan(&result.Matched); err != nil {
		return result, err
	}
	if apply {
		deleted, err := tx.ExecContext(ctx, "DELETE FROM observations WHERE at_ns<?", before.UnixNano())
		if err != nil {
			return result, err
		}
		result.Deleted, err = deleted.RowsAffected()
		if err != nil {
			return result, err
		}
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	return result, nil
}
