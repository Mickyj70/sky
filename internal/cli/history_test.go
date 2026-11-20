package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mickyj70/sky/internal/history"
	"github.com/Mickyj70/sky/internal/provider"
)

func TestWatchRecordingOptInAndReplay(t *testing.T) {
	for _, record := range []bool{false, true} {
		t.Run(map[bool]string{false: "default", true: "record"}[record], func(t *testing.T) {
			dir := prepareWatch(t)
			clock := &watchClock{now: time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)}
			snapshot := clock.now
			calls := 0
			source := sourceFunc(func(ctx context.Context, q provider.Query) (provider.Result, error) {
				calls++
				return provider.Result{Source: "fixture", SnapshotAt: &snapshot, RetrievedAt: clock.now, Aircraft: []provider.Aircraft{}}, nil
			})
			for run := 0; run < 2; run++ {
				cmd := newWithClock(source, clock)
				args := []string{"watch", "run", "--data-dir", dir, "--max-requests", "2", "--json"}
				if record {
					args = append(args, "--record")
				}
				cmd.SetArgs(args)
				var out bytes.Buffer
				cmd.SetOut(&out)
				if err := Execute(context.Background(), cmd); err != nil {
					t.Fatal(err)
				}
				events := decodeEvents(t, out.Bytes())
				observations := 0
				for _, event := range events {
					if event.Type == "observation" {
						observations++
					}
				}
				if observations != 1 || events[len(events)-1].Reason != "budget_exhausted" {
					t.Fatal("recording changed suppression or termination")
				}
			}
			if calls != 4 {
				t.Fatalf("extra requests: %d", calls)
			}
			if !record {
				if _, err := os.Stat(filepath.Join(dir, history.Filename)); !os.IsNotExist(err) {
					t.Fatal("default watch created history")
				}
				return
			}
			archive, err := history.Open(context.Background(), dir)
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			stats, err := archive.Stats(context.Background())
			if err != nil || stats.Count != 1 {
				t.Fatalf("cross-run replay stats %+v %v", stats, err)
			}
		})
	}
}
func TestWatchRecordsNewSnapshotsDespiteDisplaySuppression(t *testing.T) {
	dir := prepareWatch(t)
	clock := &watchClock{now: time.Now().UTC()}
	calls := 0
	source := sourceFunc(func(context.Context, provider.Query) (provider.Result, error) {
		calls++
		stamp := clock.now
		return provider.Result{Source: "fixture", SnapshotAt: &stamp, RetrievedAt: stamp, Aircraft: []provider.Aircraft{}}, nil
	})
	cmd := newWithClock(source, clock)
	cmd.SetArgs([]string{"watch", "run", "--data-dir", dir, "--max-requests", "3", "--record", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := Execute(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	observations := 0
	for _, event := range decodeEvents(t, out.Bytes()) {
		if event.Type == "observation" {
			observations++
		}
	}
	archive, err := history.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	stats, err := archive.Stats(context.Background())
	if err != nil || stats.Count != 3 || observations != 1 || calls != 3 {
		t.Fatalf("stats=%+v err=%v displayed=%d calls=%d", stats, err, observations, calls)
	}
}
func TestWatchHistoryFailureStopsAndProviderErrorsAreNotStored(t *testing.T) {
	for _, badResult := range []bool{false, true} {
		t.Run(map[bool]string{false: "provider failure", true: "recording failure"}[badResult], func(t *testing.T) {
			dir := prepareWatch(t)
			calls := 0
			clock := &watchClock{now: time.Now().UTC()}
			source := sourceFunc(func(context.Context, provider.Query) (provider.Result, error) {
				calls++
				if badResult {
					return provider.Result{}, nil
				}
				return provider.Result{}, &provider.Error{Kind: provider.RateLimited, StatusCode: 429}
			})
			cmd := newWithClock(source, clock)
			cmd.SetArgs([]string{"watch", "run", "--data-dir", dir, "--max-requests", "2", "--record", "--json"})
			var out bytes.Buffer
			cmd.SetOut(&out)
			err := Execute(context.Background(), cmd)
			if badResult {
				if err == nil || !strings.Contains(err.Error(), "record history") || calls != 1 {
					t.Fatalf("error=%v calls=%d", err, calls)
				}
			} else if err != nil || calls != 2 {
				t.Fatalf("provider error=%v calls=%d", err, calls)
			}
			archive, err := history.Open(context.Background(), dir)
			if err != nil {
				t.Fatal(err)
			}
			defer archive.Close()
			stats, err := archive.Stats(context.Background())
			if err != nil || stats.Count != 0 {
				t.Fatal("failed response stored")
			}
			events := decodeEvents(t, out.Bytes())
			if badResult && events[len(events)-1].Reason != "error" {
				t.Fatal("recording failure not reported")
			}
		})
	}
}
func TestHistoryCommandsOfflineRecoveryAndRetentionGuards(t *testing.T) {
	dir := t.TempDir()
	backup := filepath.Join(t.TempDir(), "snapshot.db")
	restored := t.TempDir()
	execute := func(args ...string) (string, error) {
		cmd := NewWithSource(nil)
		cmd.SetArgs(append([]string{"--json"}, args...))
		var out bytes.Buffer
		cmd.SetOut(&out)
		err := Execute(context.Background(), cmd)
		return out.String(), err
	}
	out, err := execute("history", "status", "--data-dir", dir)
	if err != nil {
		t.Fatal(err)
	}
	var stats history.Stats
	if err = json.Unmarshal([]byte(out), &stats); err != nil || stats.Count != 0 {
		t.Fatalf("status=%s err=%v", out, err)
	}
	if _, err = execute("history", "backup", backup, "--data-dir", dir); err != nil {
		t.Fatal(err)
	}
	if _, err = execute("history", "restore", backup, "--data-dir", restored); err != nil {
		t.Fatal(err)
	}
	if _, err = execute("history", "restore", backup, "--data-dir", restored); err == nil {
		t.Fatal("overwrote history")
	}
	for _, args := range [][]string{{"watch", "run", "--max-requests", "1", "--record", "--history-retention", "0s"}, {"watch", "run", "--max-requests", "1", "--history-retention", "1h"}, {"history", "status", "--data-dir", " "}} {
		if _, err = execute(args...); err == nil {
			t.Fatalf("accepted invalid args %v", args)
		}
	}
}
