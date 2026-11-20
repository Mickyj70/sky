// Package history stores opt-in, locally collected provider observations.
// Its database is independent of the schema-1 saved watchlist.
package history

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Mickyj70/sky/internal/storage"
	"github.com/Mickyj70/sky/internal/tracking"
	_ "modernc.org/sqlite"
)

const Filename = "history.db"
const applicationID = 0x534b5948 // SKYH; independent of SKY1 watchlists
const schemaVersion = 1

const schema = `CREATE TABLE observations (
 key TEXT PRIMARY KEY,
 kind TEXT NOT NULL CHECK(kind IN ('registration','hex')),
 identifier TEXT NOT NULL,
 source TEXT NOT NULL,
 at_ns INTEGER NOT NULL,
 payload TEXT NOT NULL
) WITHOUT ROWID;
CREATE INDEX observations_target_time ON observations(kind,identifier,at_ns);
CREATE INDEX observations_time ON observations(at_ns);
PRAGMA application_id = 1397446984;
PRAGMA user_version = 1;`

type Store struct {
	db   *sql.DB
	path string
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) Path() string { return s.path }

func connect(path string, readonly bool) (*sql.DB, error) {
	path = filepath.ToSlash(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	uri := url.URL{Scheme: "file", Path: path}
	q := url.Values{}
	if readonly {
		q.Set("mode", "ro")
		q.Add("_pragma", "query_only(1)")
	} else {
		q.Set("mode", "rw")
		q.Set("_txlock", "immediate")
	}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "trusted_schema(0)")
	uri.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

// Open only initializes a new empty database. No implicit schema upgrades occur.
func Open(ctx context.Context, dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path, err := filepath.Abs(filepath.Join(dir, Filename))
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err == nil {
		err = f.Close()
	} else if os.IsExist(err) {
		err = nil
	}
	if err != nil {
		return nil, err
	}
	return openFile(ctx, path)
}
func openFile(ctx context.Context, path string) (*Store, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("history database must be a regular file")
	}
	db, err := connect(path, false)
	if err != nil {
		return nil, err
	}
	var tx *sql.Tx
	fail := func(err error) (*Store, error) {
		if tx != nil {
			tx.Rollback()
		}
		db.Close()
		return nil, fmt.Errorf("open history: %w", err)
	}
	// Existing schemas are validated in a read transaction, without a writer lock.
	tx, err = db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fail(err)
	}
	app, version, err := identity(ctx, tx)
	if err == nil && (app != 0 || version != 0) {
		err = validate(ctx, tx)
		tx.Rollback()
		if err != nil {
			return fail(err)
		}
		return &Store{db, path}, nil
	}
	tx.Rollback()
	if err != nil {
		return fail(err)
	}
	tx, err = db.BeginTx(ctx, nil)
	if err != nil {
		return fail(err)
	}
	defer tx.Rollback()
	app, version, err = identity(ctx, tx)
	if err != nil {
		return fail(err)
	}
	if app == 0 && version == 0 {
		var count int
		if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE name NOT GLOB 'sqlite_*'").Scan(&count); err != nil {
			return fail(err)
		}
		if count != 0 {
			return fail(fmt.Errorf("refusing to initialize unrelated history database"))
		}
		if _, err = tx.ExecContext(ctx, schema); err != nil {
			return fail(err)
		}
	}
	if err = validate(ctx, tx); err != nil {
		return fail(err)
	}
	if err = tx.Commit(); err != nil {
		return fail(err)
	}
	return &Store{db, path}, nil
}
func identity(ctx context.Context, tx *sql.Tx) (int, int, error) {
	var app, version int
	if err := tx.QueryRowContext(ctx, "PRAGMA application_id").Scan(&app); err != nil {
		return 0, 0, err
	}
	err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version)
	return app, version, err
}
func validate(ctx context.Context, tx *sql.Tx) error {
	app, version, err := identity(ctx, tx)
	if err != nil {
		return err
	}
	if app != applicationID {
		return fmt.Errorf("database is not Sky history")
	}
	if version != schemaVersion {
		return fmt.Errorf("unsupported history schema %d (supports %d); use a compatible binary or restore its backup", version, schemaVersion)
	}
	rows, err := tx.QueryContext(ctx, "SELECT key,kind,identifier,source,at_ns,payload FROM observations LIMIT 0")
	if err != nil {
		return err
	}
	return rows.Close()
}

// encode normalizes the persisted envelope and hashes snapshot identity/content.
// Evaluation/retrieval times are excluded from identity when a snapshot exists:
// the first local encounter is retained when the same snapshot is replayed.
func encode(o tracking.Observation) (tracking.Observation, string, []byte, error) {
	if o.Failure != nil || o.Result == nil {
		return o, "", nil, fmt.Errorf("history requires a successful lookup result")
	}
	id, err := storage.Normalize(storage.Kind(o.Target.Kind), o.Target.Identifier)
	if err != nil {
		return o, "", nil, err
	}
	o.Target.Identifier = id
	o.At = o.At.UTC()
	if o.At.IsZero() || !time.Unix(0, o.At.UnixNano()).Equal(o.At) {
		return o, "", nil, fmt.Errorf("history timestamp outside supported nanosecond range")
	}
	if o.State != tracking.Observed && o.State != tracking.Stale && o.State != tracking.Missing && o.State != tracking.Unknown {
		return o, "", nil, fmt.Errorf("invalid history observation state")
	}
	result := *o.Result
	result.RetrievedAt = result.RetrievedAt.UTC()
	if strings.TrimSpace(result.Source) == "" || result.RetrievedAt.IsZero() {
		return o, "", nil, fmt.Errorf("history requires source and retrieval time")
	}
	if result.SnapshotAt != nil {
		stamp := result.SnapshotAt.UTC()
		result.SnapshotAt = &stamp
	}
	result.Aircraft = append(result.Aircraft[:0:0], result.Aircraft...)
	sort.SliceStable(result.Aircraft, func(i, j int) bool { return result.Aircraft[i].Hex < result.Aircraft[j].Hex })
	o.Result = &result
	payload, err := json.Marshal(o)
	if err != nil {
		return o, "", nil, err
	}
	identityResult := result
	if result.SnapshotAt != nil {
		identityResult.RetrievedAt = time.Time{}
	}
	keyData, err := json.Marshal(struct {
		Target tracking.Target
		Result any
	}{o.Target, identityResult})
	if err != nil {
		return o, "", nil, err
	}
	sum := sha256.Sum256(keyData)
	return o, hex.EncodeToString(sum[:]), payload, nil
}

// Record atomically prunes old encounters and saves this response. Repeated
// snapshots are deduplicated across runs. Errors must stop recording, not vanish.
func (s *Store) Record(ctx context.Context, o tracking.Observation, retention time.Duration) (bool, error) {
	if retention <= 0 {
		return false, fmt.Errorf("history retention must be positive")
	}
	o, key, payload, err := encode(o)
	if err != nil {
		return false, err
	}
	cutoff := o.At.Add(-retention)
	if !time.Unix(0, cutoff.UnixNano()).Equal(cutoff) {
		return false, fmt.Errorf("retention cutoff outside supported range")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "DELETE FROM observations WHERE at_ns < ?", cutoff.UnixNano()); err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, "INSERT INTO observations(key,kind,identifier,source,at_ns,payload) VALUES(?,?,?,?,?,?) ON CONFLICT(key) DO NOTHING", key, o.Target.Kind, o.Target.Identifier, o.Result.Source, o.At.UnixNano(), string(payload))
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return n == 1, nil
}

type Stats struct {
	Database string `json:"database"`
	Count    int64  `json:"count"`
	Bytes    int64  `json:"bytes"`
}

func (s *Store) Stats(ctx context.Context) (Stats, error) {
	stats := Stats{Database: s.path}
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM observations").Scan(&stats.Count); err != nil {
		return stats, err
	}
	// Includes allocated main-file space; retention reuses pages, not VACUUM.
	info, err := os.Stat(s.path)
	if err != nil {
		return stats, err
	}
	stats.Bytes = info.Size()
	return stats, nil
}
