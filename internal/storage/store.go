// Package storage persists local watch targets without provider requests.
package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const (
	Filename      = "watchlist.db"
	schemaVersion = 1
	applicationID = 0x534b5931 // SKY1
)

type Kind string

const (
	Registration Kind = "registration"
	Hex          Kind = "hex"
)

type Target struct {
	Kind       Kind      `json:"kind"`
	Identifier string    `json:"identifier"`
	CreatedAt  time.Time `json:"created_at"`
}

var (
	registrationPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{1,11}$`)
	hexPattern          = regexp.MustCompile(`^[a-f0-9]{6}$`)
)

func Normalize(kind Kind, identifier string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	switch kind {
	case Registration:
		identifier = strings.ToUpper(identifier)
		if registrationPattern.MatchString(identifier) {
			return identifier, nil
		}
	case Hex:
		identifier = strings.ToLower(identifier)
		if hexPattern.MatchString(identifier) {
			return identifier, nil
		}
	}
	return "", fmt.Errorf("invalid %s target; use a registration or a six-character ICAO hex address", kind)
}

type Store struct {
	db   *sql.DB
	path string
}

func (s *Store) Path() string { return s.path }
func (s *Store) Close() error { return s.db.Close() }

// Open initializes a new schema transactionally, or validates an existing one.
// Unknown applications and newer schemas are refused without migration.
func Open(ctx context.Context, dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	path, err := filepath.Abs(filepath.Join(dir, Filename))
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err == nil {
		if err = f.Close(); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("create watchlist database: %w", err)
	}
	return openFile(ctx, path)
}

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
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "trusted_schema(0)")
	uri.RawQuery = q.Encode()
	db, err := sql.Open("sqlite", uri.String())
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return db, nil
}

func openFile(ctx context.Context, path string) (*Store, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("watchlist database must be a regular file: %s", path)
	}
	db, err := connect(path, false)
	if err != nil {
		return nil, err
	}
	if err = initialize(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("open watchlist: %w", err)
	}
	return &Store{db: db, path: path}, nil
}

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func identity(ctx context.Context, q rowQuerier) (int, int, error) {
	var app, version int
	if err := q.QueryRowContext(ctx, "PRAGMA application_id").Scan(&app); err != nil {
		return 0, 0, err
	}
	if err := q.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, 0, err
	}
	return app, version, nil
}
func checkIdentity(app, version int) error {
	if app != applicationID {
		return fmt.Errorf("database is not a Sky watchlist")
	}
	if version != schemaVersion {
		return fmt.Errorf("unsupported watchlist schema %d (this binary supports %d); use a compatible version or restore its backup", version, schemaVersion)
	}
	return nil
}
func initialize(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	app, version, err := identity(ctx, tx)
	if err != nil {
		return err
	}
	if app == 0 && version == 0 {
		var objects int
		if err := tx.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE name NOT GLOB 'sqlite_*'").Scan(&objects); err != nil {
			return err
		}
		if objects != 0 {
			return fmt.Errorf("refusing to initialize an unrelated database")
		}
		schema := fmt.Sprintf(`CREATE TABLE watch_targets (
   kind TEXT NOT NULL CHECK (kind IN ('registration','hex')),
   identifier TEXT NOT NULL,
   created_at TEXT NOT NULL,
   PRIMARY KEY (kind, identifier)
  ) WITHOUT ROWID;
  PRAGMA application_id = %d;
  PRAGMA user_version = %d;`, applicationID, schemaVersion)
		if _, err := tx.ExecContext(ctx, schema); err != nil {
			return err
		}
	} else if err := checkIdentity(app, version); err != nil {
		return err
	}
	// Detect missing tables/columns before exposing a store to writers.
	rows, err := tx.QueryContext(ctx, "SELECT kind,identifier,created_at FROM watch_targets LIMIT 0")
	if err != nil {
		return err
	}
	rows.Close()
	return tx.Commit()
}

func (s *Store) Add(ctx context.Context, kind Kind, id string) (Target, bool, error) {
	id, err := Normalize(kind, id)
	if err != nil {
		return Target{}, false, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Target{}, false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "INSERT INTO watch_targets(kind,identifier,created_at) VALUES(?,?,?) ON CONFLICT(kind,identifier) DO NOTHING", kind, id, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return Target{}, false, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return Target{}, false, err
	}
	var target Target
	var stamp string
	if err := tx.QueryRowContext(ctx, "SELECT kind,identifier,created_at FROM watch_targets WHERE kind=? AND identifier=?", kind, id).Scan(&target.Kind, &target.Identifier, &stamp); err != nil {
		return Target{}, false, err
	}
	target.CreatedAt, err = time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return Target{}, false, fmt.Errorf("invalid saved creation timestamp: %w", err)
	}
	target.CreatedAt = target.CreatedAt.UTC()
	if err := tx.Commit(); err != nil {
		return Target{}, false, err
	}
	return target, count == 1, nil
}

func (s *Store) Remove(ctx context.Context, kind Kind, id string) (bool, error) {
	id, err := Normalize(kind, id)
	if err != nil {
		return false, err
	}
	result, err := s.db.ExecContext(ctx, "DELETE FROM watch_targets WHERE kind=? AND identifier=?", kind, id)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

type rowsQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func targets(ctx context.Context, q rowsQuerier) ([]Target, error) {
	rows, err := q.QueryContext(ctx, "SELECT kind,identifier,created_at FROM watch_targets ORDER BY kind,identifier")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := make([]Target, 0)
	seen := map[string]bool{}
	for rows.Next() {
		var target Target
		var stamp string
		if err := rows.Scan(&target.Kind, &target.Identifier, &stamp); err != nil {
			return nil, err
		}
		canonical, err := Normalize(target.Kind, target.Identifier)
		if err != nil || canonical != target.Identifier {
			return nil, fmt.Errorf("database contains an invalid or noncanonical watch target")
		}
		key := string(target.Kind) + ":" + target.Identifier
		if seen[key] {
			return nil, fmt.Errorf("database contains duplicate watch targets")
		}
		seen[key] = true
		target.CreatedAt, err = time.Parse(time.RFC3339Nano, stamp)
		if err != nil {
			return nil, fmt.Errorf("invalid saved creation timestamp: %w", err)
		}
		target.CreatedAt = target.CreatedAt.UTC()
		list = append(list, target)
	}
	return list, rows.Err()
}
func (s *Store) List(ctx context.Context) ([]Target, error) { return targets(ctx, s.db) }
