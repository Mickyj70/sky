package cli

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Mickyj70/sky/internal/history"
	"github.com/Mickyj70/sky/internal/provider"
	"github.com/Mickyj70/sky/internal/tracking"
)

func historyFixture(t *testing.T) (string, tracking.Observation) {
	t.Helper()
	dir := t.TempDir()
	s, err := history.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	at := time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)
	snapshot := at.Add(-time.Second)
	zero := 0.0
	o := tracking.Observation{At: at, Target: tracking.Target{Kind: provider.ByRegistration, Identifier: "G-TEST"}, State: tracking.Observed, PositionAgeSeconds: &zero, Result: &provider.Result{Source: "adsb.lol", SnapshotAt: &snapshot, RetrievedAt: at, Aircraft: []provider.Aircraft{{Hex: "abc123", Callsign: "TEST1", Altitude: json.RawMessage(`"ground"`), Speed: &zero, Latitude: &zero}}}}
	if _, err = s.Record(context.Background(), o, 720*time.Hour); err != nil {
		t.Fatal(err)
	}
	return dir, o
}
func runHistory(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := NewWithSource(sourceFunc(func(context.Context, provider.Query) (provider.Result, error) {
		t.Fatal("history made a provider request")
		return provider.Result{}, nil
	}))
	cmd.SetArgs(args)
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	err := Execute(context.Background(), cmd)
	if stderr.Len() != 0 {
		t.Fatalf("unexpected stderr: %s", stderr.String())
	}
	return out.String(), err
}
func TestHistoryJSONAndCSVExportParity(t *testing.T) {
	dir, want := historyFixture(t)
	out, err := runHistory(t, "history", "--data-dir", dir, "--registration", "g-test", "--since", "2026-11-21T01:00:00+01:00", "--until", "2026-11-21T01:00:00Z", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var page history.Page
	if err = json.Unmarshal([]byte(out), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Records) != 1 || !reflect.DeepEqual(page.Records[0].Observation, want) || page.Attribution[0].License != "ODbL 1.0" || page.Collection != "local" {
		t.Fatalf("JSON mismatch: %s", out)
	}
	exported, err := runHistory(t, "history", "export", "--data-dir", dir, "--registration", "G-TEST", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	var other history.Page
	if err = json.Unmarshal([]byte(exported), &other); err != nil || !reflect.DeepEqual(other.Records, page.Records) {
		t.Fatalf("JSON export mismatch: %s %v", exported, err)
	}
	out, err = runHistory(t, "history", "export", "--data-dir", dir, "--format", "csv")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil || len(rows) != 2 {
		t.Fatalf("CSV: %s %v", out, err)
	}
	row := map[string]string{}
	for i, name := range rows[0] {
		row[name] = rows[1][i]
	}
	if row["source"] != "adsb.lol" || row["source_license"] != "ODbL 1.0" || row["collection"] != "local" || row["page_has_more"] != "false" || row["position_age_seconds"] != "0" || row["snapshot_at_utc"] != want.Result.SnapshotAt.Format(time.RFC3339Nano) {
		t.Fatalf("metadata lost: %+v", row)
	}
	var aircraft []provider.Aircraft
	if err = json.Unmarshal([]byte(row["aircraft_json"]), &aircraft); err != nil || !reflect.DeepEqual(aircraft, want.Result.Aircraft) {
		t.Fatalf("aircraft/units/missing values lost: %s %v", row["aircraft_json"], err)
	}
}
func TestHistoryEmptyErrorsAndPrune(t *testing.T) {
	dir, _ := historyFixture(t)
	out, err := runHistory(t, "history", "--hex", "ABC123", "--data-dir", dir, "--json")
	if err != nil || !strings.Contains(out, `"records": []`) {
		t.Fatalf("empty JSON: %s %v", out, err)
	}
	out, err = runHistory(t, "history", "export", "--format", "csv", "--hex", "ABC123", "--data-dir", dir)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil || len(rows) != 1 {
		t.Fatal("empty CSV needs headers only")
	}
	for _, args := range [][]string{
		{"--since", "0h"}, {"--until", "24h"}, {"--since", "1600-01-01T00:00:00Z"}, {"--since", "2026-11-22T00:00:00Z", "--until", "2026-11-21T00:00:00Z"}, {"--limit", "1001"}, {"--offset", "-1"}, {"--hex", "bad"}, {"--registration", "G-TEST", "--hex", "abc123"}, {"export", "--format", "yaml"}, {"export", "--format", "csv", "--json"}, {"prune", "--before", "-1h"},
	} {
		args = append([]string{"history", "--data-dir", dir}, args...)
		out, err = runHistory(t, args...)
		if err == nil || ExitCode(err) != 2 || out != "" {
			t.Fatalf("invalid args %v output=%q error=%v", args, out, err)
		}
	}
	for _, apply := range []bool{false, true} {
		args := []string{"history", "prune", "--data-dir", dir, "--before", "2026-11-22T00:00:00Z", "--json"}
		if apply {
			args = append(args, "--apply")
		}
		out, err = runHistory(t, args...)
		if err != nil {
			t.Fatal(err)
		}
		var result history.PruneResult
		if err = json.Unmarshal([]byte(out), &result); err != nil || result.Matched != 1 || result.Applied != apply {
			t.Fatalf("prune: %s %v", out, err)
		}
		if !apply && result.Deleted != 0 || apply && result.Deleted != 1 {
			t.Fatal("incorrect deletion count")
		}
	}
}
func TestHistoryCorruptionAndOutputFailure(t *testing.T) {
	dir, _ := historyFixture(t)
	cmd := NewWithSource(nil)
	cmd.SetArgs([]string{"history", "--data-dir", dir, "--json"})
	cmd.SetOut(brokenWriter{})
	if err := Execute(context.Background(), cmd); err == nil || ExitCode(err) != 1 {
		t.Fatalf("writer error: %v", err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, history.Filename))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE observations SET payload='{' "); err != nil {
		t.Fatal(err)
	}
	db.Close()
	for _, args := range [][]string{{"history", "--json"}, {"history", "export", "--format", "csv"}} {
		args = append(args, "--data-dir", dir)
		out, err := runHistory(t, args...)
		if err == nil || ExitCode(err) != 1 || out != "" || !strings.Contains(err.Error(), "corrupt") {
			t.Fatalf("corruption output=%q err=%v", out, err)
		}
	}
}
func TestSpreadsheetSafeSourceAndPagingMetadata(t *testing.T) {
	for _, value := range []string{"=1+1", "+CMD()", "-CMD()", "@SUM(1)", " \t=1", "\ufeff=1", "\r@A"} {
		if !strings.HasPrefix(spreadsheetText(value), "'") {
			t.Fatalf("unsafe CSV %q", value)
		}
	}
	if spreadsheetText("adsb.lol") != "adsb.lol" {
		t.Fatal("normal text altered")
	}
	dir, o := historyFixture(t)
	o.Result.Source = " \t=HYPERLINK(\"x\",\"y\")"
	s, err := history.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Record(context.Background(), o, 720*time.Hour); err != nil {
		t.Fatal(err)
	}
	s.Close()
	out, err := runHistory(t, "history", "export", "--data-dir", dir, "--format", "csv", "--limit", "1")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if rows[1][13] != "true" || rows[1][14] != "1" {
		t.Fatalf("pagination omitted: %v", rows)
	}
	out, err = runHistory(t, "history", "export", "--data-dir", dir, "--format", "csv")
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(out))
	reader.Read()
	found := false
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(row[9], "HYPERLINK") {
			found = true
			if row[9] != "'"+o.Result.Source {
				t.Fatalf("unsafe source: %q", row[9])
			}
		}
	}
	if !found {
		t.Fatal("missing dangerous source fixture")
	}
}
func TestRelativeHistoryBound(t *testing.T) {
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	since, err := historyBound("24h", now, true)
	if err != nil || !since.Equal(now.Add(-24*time.Hour)) {
		t.Fatalf("relative bound: %v %v", since, err)
	}
}
