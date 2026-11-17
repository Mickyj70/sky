package storage

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestOpenAndListWhileAnotherWriterHasUncommittedChanges(t *testing.T) {
	for _, journal := range []string{"DELETE", "WAL"} {
		t.Run(journal, func(t *testing.T) {
			dir := t.TempDir()
			ctx := context.Background()
			writer := openTestStore(t, dir)
			if _, err := writer.db.ExecContext(ctx, "PRAGMA journal_mode="+journal); err != nil {
				t.Fatal(err)
			}
			if _, _, err := writer.Add(ctx, Registration, "G-TEST"); err != nil {
				t.Fatal(err)
			}
			transaction, err := writer.db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer transaction.Rollback()
			if _, err := transaction.ExecContext(ctx, "INSERT INTO watch_targets VALUES(?,?,?)", Hex, "406d7b", time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
				t.Fatal(err)
			}
			// An immediate-open regression would wait for the writer and return BUSY.
			readContext, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			reader, err := Open(readContext, dir)
			if err != nil {
				t.Fatalf("existing store should open while a writer is active: %v", err)
			}
			saved, err := reader.List(readContext)
			if err != nil || len(saved) != 1 || saved[0].Identifier != "G-TEST" {
				reader.Close()
				t.Fatalf("reader did not see committed state: %+v %v", saved, err)
			}
			if err := reader.Close(); err != nil {
				t.Fatal(err)
			}
			if err := transaction.Commit(); err != nil {
				t.Fatal(err)
			}
			reader = openTestStore(t, dir)
			saved, err = reader.List(ctx)
			if err != nil || len(saved) != 2 {
				t.Fatalf("committed update unavailable: %+v %v", saved, err)
			}
		})
	}
}

func TestConcurrentFirstOpenRechecksSchemaUnderWriteLock(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new")
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := Open(context.Background(), dir)
			if err == nil {
				_, _, err = s.Add(context.Background(), Registration, "G-TEST")
				s.Close()
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	s := openTestStore(t, dir)
	saved, err := s.List(context.Background())
	if err != nil || len(saved) != 1 {
		t.Fatalf("initialization race lost data: %+v %v", saved, err)
	}
}

func TestReadValidationRejectsMissingColumns(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, "ALTER TABLE watch_targets RENAME COLUMN created_at TO broken"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if reader, err := Open(ctx, dir); err == nil {
		reader.Close()
		t.Fatal("accepted damaged schema")
	}
	if _, err := os.Stat(filepath.Join(dir, Filename)); err != nil {
		t.Fatal("validation removed existing database")
	}
}
