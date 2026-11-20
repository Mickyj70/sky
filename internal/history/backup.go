package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Mickyj70/sky/internal/tracking"
)

func syncFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	err = f.Sync()
	closeErr := f.Close()
	if err != nil {
		return err
	}
	return closeErr
}

// Backup creates a consistent SQLite snapshot without overwriting any file.
func (s *Store) Backup(ctx context.Context, destination string) (string, error) {
	path, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return "", fmt.Errorf("backup destination must not exist: %w", err)
	}
	info, statErr := f.Stat()
	err = f.Close()
	if statErr != nil || err != nil {
		os.Remove(path)
		return "", fmt.Errorf("reserve history backup: stat=%v close=%v", statErr, err)
	}
	success := false
	defer func() {
		if !success {
			if current, err := os.Lstat(path); err == nil && os.SameFile(info, current) {
				os.Remove(path)
			}
		}
	}()
	if _, err = s.db.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		return "", err
	}
	if err = walkBackup(ctx, path, nil); err != nil {
		return "", fmt.Errorf("verify history backup: %w", err)
	}
	if err = syncFile(path); err != nil {
		return "", err
	}
	success = true
	return path, nil
}

// walkBackup validates every row; only known data is imported, never source DDL.
func walkBackup(ctx context.Context, path string, visit func(tracking.Observation, string, []byte) error) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("history backup must be a regular file")
	}
	db, err := connect(path, true)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = validate(ctx, tx); err != nil {
		return err
	}
	var check string
	if err = tx.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil {
		return err
	}
	if check != "ok" {
		return fmt.Errorf("history backup integrity failed: %s", check)
	}
	rows, err := tx.QueryContext(ctx, "SELECT key,kind,identifier,source,at_ns,payload FROM observations ORDER BY key")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var key, kind, id, source, payload string
		var stamp int64
		if err = rows.Scan(&key, &kind, &id, &source, &stamp, &payload); err != nil {
			return err
		}
		var o tracking.Observation
		if err = json.Unmarshal([]byte(payload), &o); err != nil {
			return err
		}
		normalized, want, encoded, err := encode(o)
		if err != nil {
			return err
		}
		if want != key || kind != string(normalized.Target.Kind) || id != normalized.Target.Identifier || source != normalized.Result.Source || stamp != normalized.At.UnixNano() {
			return fmt.Errorf("history backup row identity mismatch")
		}
		if visit != nil {
			if err = visit(normalized, key, encoded); err != nil {
				return err
			}
		}
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	return tx.Commit()
}

// Restore validates and imports a snapshot into a fresh schema, then publishes
// it using a non-overwriting hard link. Watchlist files are never touched.
func Restore(ctx context.Context, backup, dir string) (string, int64, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", 0, err
	}
	if err = os.MkdirAll(dir, 0700); err != nil {
		return "", 0, err
	}
	final := filepath.Join(dir, Filename)
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		if _, err = os.Lstat(final + suffix); err == nil {
			return "", 0, fmt.Errorf("history restore destination exists; choose a new directory")
		} else if !os.IsNotExist(err) {
			return "", 0, err
		}
	}
	file, err := os.CreateTemp(dir, ".sky-history-restore-*.db")
	if err != nil {
		return "", 0, err
	}
	stage := file.Name()
	defer os.Remove(stage)
	if err = file.Close(); err != nil {
		return "", 0, err
	}
	s, err := openFile(ctx, stage)
	if err != nil {
		return "", 0, err
	}
	defer s.Close()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback()
	var count int64
	err = walkBackup(ctx, backup, func(o tracking.Observation, key string, payload []byte) error {
		_, err := tx.ExecContext(ctx, "INSERT INTO observations(key,kind,identifier,source,at_ns,payload) VALUES(?,?,?,?,?,?)", key, o.Target.Kind, o.Target.Identifier, o.Result.Source, o.At.UnixNano(), string(payload))
		if err == nil {
			count++
		}
		return err
	})
	if err != nil {
		return "", 0, fmt.Errorf("read history backup: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return "", 0, err
	}
	if err = s.Close(); err != nil {
		return "", 0, err
	}
	if err = syncFile(stage); err != nil {
		return "", 0, err
	}
	if err = ctx.Err(); err != nil {
		return "", 0, err
	}
	if err = os.Link(stage, final); err != nil {
		return "", 0, fmt.Errorf("publish history without overwrite (requires hard links): %w", err)
	}
	return final, count, nil
}
