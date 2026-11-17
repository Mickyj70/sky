package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mickyj70/sky/internal/provider"
	"github.com/Mickyj70/sky/internal/storage"
)

func executeWatch(t *testing.T, dir string, args ...string) (map[string]any, error) {
	t.Helper()
	source := sourceFunc(func(context.Context, provider.Query) (provider.Result, error) {
		t.Fatal("watchlist command queried a flight provider")
		return provider.Result{}, nil
	})
	cmd := NewWithSource(source)
	cmd.SetArgs(append([]string{"--json", "watch", "--data-dir", dir}, args...))
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	err := Execute(context.Background(), cmd)
	if err != nil {
		if stdout.Len() != 0 {
			t.Fatalf("failed command wrote result: %s", stdout.String())
		}
		return nil, err
	}
	if stderr.Len() != 0 {
		t.Fatalf("success wrote stderr: %s", stderr.String())
	}
	var result map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %s", stdout.String())
	}
	return result, nil
}

func TestWatchLifecycleOffline(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	result, err := executeWatch(t, dir, "list")
	if err != nil {
		t.Fatal(err)
	}
	if targets, ok := result["targets"].([]any); !ok || len(targets) != 0 {
		t.Fatalf("expected empty array: %+v", result)
	}
	for i, id := range []string{"g-zbke", "G-ZBKE"} {
		result, err = executeWatch(t, dir, "add", "--registration", id)
		if err != nil {
			t.Fatal(err)
		}
		if result["changed"] != (i == 0) {
			t.Fatalf("unexpected duplicate behavior: %+v", result)
		}
	}
	if _, err := executeWatch(t, dir, "add", "--hex", "406D7B"); err != nil {
		t.Fatal(err)
	}
	result, err = executeWatch(t, dir, "list")
	if err != nil {
		t.Fatal(err)
	}
	if len(result["targets"].([]any)) != 2 {
		t.Fatalf("targets not persisted: %+v", result)
	}
	backup := filepath.Join(t.TempDir(), "watch.db")
	if _, err := executeWatch(t, dir, "backup", backup); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		result, err = executeWatch(t, dir, "remove", "--registration", "G-ZBKE")
		if err != nil {
			t.Fatal(err)
		}
		if result["changed"] != (i == 0) {
			t.Fatalf("unexpected removal: %+v", result)
		}
	}
	restored := filepath.Join(t.TempDir(), "recovered")
	result, err = executeWatch(t, restored, "restore", backup)
	if err != nil {
		t.Fatal(err)
	}
	if result["count"] != float64(2) {
		t.Fatalf("wrong restore count: %+v", result)
	}
	result, err = executeWatch(t, restored, "list")
	if err != nil || len(result["targets"].([]any)) != 2 {
		t.Fatalf("recovered list: %+v %v", result, err)
	}
	if _, err := executeWatch(t, dir, "restore", backup); err == nil || ExitCode(err) != 1 {
		t.Fatalf("expected no-overwrite runtime error, got %v", err)
	}
}

func TestWatchValidationBeforeCreatingState(t *testing.T) {
	for _, args := range [][]string{{"add"}, {"remove"}, {"add", "--registration", ""}, {"add", "--registration", "G-ZBKE", "--hex", "406d7b"}, {"remove", "--hex", "wrong"}, {"list", "extra"}, {"backup", ""}, {"restore", ""}} {
		dir := filepath.Join(t.TempDir(), "unused")
		_, err := executeWatch(t, dir, args...)
		if err == nil || ExitCode(err) != 2 {
			t.Fatalf("%v: expected usage exit 2, got %v", args, err)
		}
		if _, err := os.Stat(filepath.Join(dir, storage.Filename)); !os.IsNotExist(err) {
			t.Fatalf("invalid args created database: %v", args)
		}
	}
	t.Setenv("SKY_DATA_DIR", filepath.Join(t.TempDir(), "safe-fallback"))
	if _, err := executeWatch(t, "", "list"); err == nil || ExitCode(err) != 2 {
		t.Fatalf("explicit blank directory should fail, got %v", err)
	}
}

func TestWatchDirectoryOverrideAndStorageErrors(t *testing.T) {
	fallback := filepath.Join(t.TempDir(), "environment")
	t.Setenv("SKY_DATA_DIR", fallback)
	override := filepath.Join(t.TempDir(), "explicit")
	if _, err := executeWatch(t, override, "list"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(fallback); !os.IsNotExist(err) {
		t.Fatal("explicit override used environment directory")
	}
	file := filepath.Join(t.TempDir(), "not-directory")
	if err := os.WriteFile(file, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := executeWatch(t, file, "list"); err == nil || ExitCode(err) != 1 {
		t.Fatalf("expected storage exit 1, got %v", err)
	}
	cmd := NewWithSource(nil)
	cmd.SetArgs([]string{"--json", "watch", "list"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := Execute(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(fallback, storage.Filename)); err != nil {
		t.Fatal("environment directory not used")
	}
}
