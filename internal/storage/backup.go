package storage

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Backup uses SQLite's own consistent snapshot mechanism, including committed
// WAL contents. Existing destination files are never overwritten.
func (s *Store) Backup(ctx context.Context, destination string) (string, error) {
	path, err := filepath.Abs(destination)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return "", err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return "", fmt.Errorf("reserve backup (destination must not exist): %w", err)
	}
	info, statErr := f.Stat()
	closeErr := f.Close()
	if statErr != nil || closeErr != nil {
		os.Remove(path)
		return "", fmt.Errorf("reserve backup: stat=%v close=%v", statErr, closeErr)
	}
	success := false
	defer func() {
		if !success {
			if current, err := os.Lstat(path); err == nil && os.SameFile(info, current) {
				os.Remove(path)
			}
		}
	}()
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO ?", path); err != nil {
		return "", fmt.Errorf("create SQLite backup: %w", err)
	}
	if _, err := readBackup(ctx, path); err != nil {
		return "", fmt.Errorf("verify SQLite backup: %w", err)
	}
	if err := syncFile(path); err != nil {
		return "", err
	}
	success = true
	return path, nil
}

func syncFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	syncErr := f.Sync()
	closeErr := f.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func readBackup(ctx context.Context, path string) ([]Target, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("backup must be a regular file")
	}
	db, err := connect(abs, true)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	app, version, err := identity(ctx, tx)
	if err != nil {
		return nil, err
	}
	if err := checkIdentity(app, version); err != nil {
		return nil, err
	}
	var check string
	if err := tx.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil {
		return nil, err
	}
	if check != "ok" {
		return nil, fmt.Errorf("backup integrity check failed: %s", check)
	}
	result, err := targets(ctx, tx)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return result, nil
}

// Restore validates and imports a snapshot into a temporary database. Publishing
// with a hard link is atomic and refuses an existing destination, including races.
// Restore never overwrites a database; use a new data directory for recovery.
func Restore(ctx context.Context, backup, dir string) (string, int, error) {
	saved, err := readBackup(ctx, backup)
	if err != nil {
		return "", 0, fmt.Errorf("read watchlist backup: %w", err)
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return "", 0, err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", 0, err
	}
	final := filepath.Join(dir, Filename)
	for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
		if _, err := os.Lstat(final + suffix); err == nil {
			return "", 0, fmt.Errorf("restore destination contains a database or sidecar; choose a new data directory")
		} else if !os.IsNotExist(err) {
			return "", 0, err
		}
	}
	temporary, err := os.CreateTemp(dir, ".sky-restore-*.db")
	if err != nil {
		return "", 0, err
	}
	stage := temporary.Name()
	if err := temporary.Close(); err != nil {
		os.Remove(stage)
		return "", 0, err
	}
	defer os.Remove(stage)
	store, err := openFile(ctx, stage)
	if err != nil {
		return "", 0, err
	}
	if err := importTargets(ctx, store, saved); err != nil {
		store.Close()
		return "", 0, err
	}
	if err := store.Close(); err != nil {
		return "", 0, err
	}
	if err := syncFile(stage); err != nil {
		return "", 0, err
	}
	if err := ctx.Err(); err != nil {
		return "", 0, err
	}
	if err := os.Link(stage, final); err != nil {
		return "", 0, fmt.Errorf("publish restored database without overwrite (requires hard-link support): %w", err)
	}
	return final, len(saved), nil
}

func importTargets(ctx context.Context, s *Store, saved []Target) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, target := range saved {
		if _, err := tx.ExecContext(ctx, "INSERT INTO watch_targets(kind,identifier,created_at) VALUES(?,?,?)", target.Kind, target.Identifier, target.CreatedAt.UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
