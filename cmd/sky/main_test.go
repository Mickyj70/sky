package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Mickyj70/sky/internal/cli"
	"github.com/Mickyj70/sky/internal/provider"
	"github.com/Mickyj70/sky/internal/storage"
)

// The helper runs the production application runner in another OS process.
// Source injection exists only in this test executable, not in the sky binary.
func TestProcessHelper(t *testing.T) {
	if os.Getenv("SKY_PROCESS_HELPER") != "1" {
		return
	}
	args := os.Args
	for i, a := range args {
		if a == "--" {
			args = args[i+1:]
			break
		}
	}
	ctx := context.Background()
	stop := func() {}
	if os.Getenv("SKY_TEST_SIGNALS") == "1" {
		ctx, stop = signal.NotifyContext(ctx, os.Interrupt)
	}
	if os.Getenv("SKY_TEST_CANCELED") == "1" {
		c, cancel := context.WithCancel(ctx)
		cancel()
		ctx = c
	}
	source := &provider.Client{BaseURL: os.Getenv("SKY_TEST_URL"), HTTP: &http.Client{}}
	code := run(ctx, cli.NewWithSource(source), args, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func TestProcessContract(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile("../../internal/provider/testdata/aircraft.json")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name         string
		args         []string
		status       int
		body         string
		code         int
		cancel, wait bool
	}{
		{name: "json", args: []string{"plane", "N123AB", "--json"}, status: 200, body: string(fixture)},
		{name: "empty", args: []string{"plane", "--hex", "abc123", "--json"}, status: 200, body: `{"ac":[]}`},
		{name: "table", args: []string{"callsign", "BAW123"}, status: 200, body: string(fixture)},
		{name: "invalid identifier", args: []string{"plane", "bad/id", "--json"}, code: 2},
		{name: "unknown command", args: []string{"not-a-command"}, code: 2},
		{name: "invalid flag", args: []string{"--not-a-flag"}, code: 2},
		{name: "required coordinate", args: []string{"nearby", "--lat", "0"}, code: 2},
		{name: "authorization", args: []string{"plane", "N123AB", "--json"}, status: 401, code: 5},
		{name: "throttled", args: []string{"plane", "N123AB", "--json"}, status: 429, code: 4},
		{name: "unavailable", args: []string{"plane", "N123AB", "--json"}, status: 503, code: 3},
		{name: "malformed", args: []string{"plane", "N123AB", "--json"}, status: 200, body: `{"ac":null}`, code: 6},
		{name: "timeout", args: []string{"plane", "N123AB", "--json", "--timeout", "10ms"}, code: 124, wait: true},
		{name: "canceled", args: []string{"plane", "N123AB", "--json"}, code: 130, cancel: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.wait {
					<-r.Context().Done()
					return
				}
				if tc.status == 0 {
					t.Error("invalid invocation made a request")
					return
				}
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			child := exec.Command(binary, append([]string{"-test.run=^TestProcessHelper$", "--"}, tc.args...)...)
			child.Env = append(os.Environ(), "SKY_PROCESS_HELPER=1", "SKY_TEST_URL="+server.URL)
			if tc.cancel {
				child.Env = append(child.Env, "SKY_TEST_CANCELED=1")
			}
			var stdout, stderr bytes.Buffer
			child.Stdout = &stdout
			child.Stderr = &stderr
			err := child.Run()
			code := 0
			if err != nil {
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					t.Fatal(err)
				}
				code = exit.ExitCode()
			}
			if code != tc.code {
				t.Fatalf("exit %d, want %d; stderr %s", code, tc.code, stderr.String())
			}
			if tc.code != 0 {
				if stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "sky:") {
					t.Fatalf("unclean error streams: stdout=%q stderr=%q", stdout.String(), stderr.String())
				}
				return
			}
			if stderr.Len() != 0 {
				t.Fatalf("success wrote stderr: %s", stderr.String())
			}
			if tc.name == "table" {
				if !strings.Contains(stdout.String(), "Source: adsb.lol") {
					t.Fatal("missing table")
				}
				return
			}
			var result provider.Result
			if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
				t.Fatalf("stdout is not pure JSON: %s", stdout.String())
			}
			if result.Aircraft == nil {
				t.Fatal("empty result must be []")
			}
			if tc.name == "json" {
				if result.Aircraft[0].Hex != "000001" {
					t.Fatal("aircraft must be sorted by hex")
				}
				if !strings.Contains(stdout.String(), `"gs": 0`) {
					t.Fatal("zero speed lost")
				}
			}
		})
	}
}

func TestBinaryEntrypoint(t *testing.T) {
	name := "sky"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(t.TempDir(), name)
	compiler := filepath.Join(runtime.GOROOT(), "bin", "go")
	if runtime.GOOS == "windows" {
		compiler += ".exe"
	}
	build := exec.Command(compiler, "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build sky: %v\n%s", err, out)
	}
	for _, args := range [][]string{{"--help"}, {"--version"}, {"plane", "bad/id"}} {
		child := exec.Command(binary, args...)
		var stdout, stderr bytes.Buffer
		child.Stdout = &stdout
		child.Stderr = &stderr
		err := child.Run()
		if args[0] == "plane" {
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 2 || stdout.Len() != 0 {
				t.Fatalf("invalid args: %v stdout=%q stderr=%q", err, stdout.String(), stderr.String())
			}
		} else if err != nil || stderr.Len() != 0 {
			t.Fatalf("%v: %v %s", args, err, stderr.String())
		}
	}
}

func TestWatchPersistenceAcrossProcesses(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "watch-data")
	for i, args := range [][]string{
		{"watch", "add", "--registration", "G-ZBKE", "--data-dir", dir, "--json"},
		{"watch", "list", "--data-dir", dir, "--json"},
		{"watch", "remove", "--registration", "G-ZBKE", "--data-dir", dir, "--json"},
		{"watch", "list", "--data-dir", dir, "--json"},
	} {
		child := exec.Command(binary, append([]string{"-test.run=^TestProcessHelper$", "--"}, args...)...)
		child.Env = append(os.Environ(), "SKY_PROCESS_HELPER=1", "SKY_TEST_URL=http://127.0.0.1:1")
		var out, stderr bytes.Buffer
		child.Stdout = &out
		child.Stderr = &stderr
		if err := child.Run(); err != nil || stderr.Len() != 0 {
			t.Fatalf("%v: %v %s", args, err, stderr.String())
		}
		var result map[string]any
		if err := json.Unmarshal(out.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if i == 1 && len(result["targets"].([]any)) != 1 {
			t.Fatal("saved target did not survive process exit")
		}
		if i == 3 && len(result["targets"].([]any)) != 0 {
			t.Fatal("removal did not survive process exit")
		}
	}
}

func TestWatchRunProcessOutputAndCancellation(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "saved")
	s, err := storage.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Add(context.Background(), storage.Registration, "G-TEST"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	for _, canceled := range []bool{false, true} {
		t.Run(fmt.Sprint(canceled), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"ac":[],"msg":"No error"}`) }))
			defer server.Close()
			args := []string{"-test.run=^TestProcessHelper$", "--", "watch", "run", "--data-dir", dir, "--max-requests", "1", "--json"}
			child := exec.Command(binary, args...)
			child.Env = append(os.Environ(), "SKY_PROCESS_HELPER=1", "SKY_TEST_URL="+server.URL)
			if canceled {
				child.Env = append(child.Env, "SKY_TEST_CANCELED=1")
			}
			var stdout, stderr bytes.Buffer
			child.Stdout = &stdout
			child.Stderr = &stderr
			err := child.Run()
			if canceled {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != 130 {
					t.Fatalf("expected canceled exit, got %v %s", err, stderr.String())
				}
				if stdout.Len() != 0 || !strings.HasPrefix(stderr.String(), "sky:") {
					t.Fatal("pre-start cancellation should have no event stream")
				}
				return
			} else if err != nil || stderr.Len() != 0 {
				t.Fatalf("watch process: %v %s", err, stderr.String())
			}
			decoder := json.NewDecoder(&stdout)
			kinds := []string{}
			for {
				var event map[string]any
				err := decoder.Decode(&event)
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				kinds = append(kinds, event["type"].(string))
			}
			if len(kinds) < 2 || kinds[0] != "started" || kinds[len(kinds)-1] != "stopped" {
				t.Fatalf("incomplete watch stream: %v", kinds)
			}
		})
	}
}

func TestWatchRunInterruptDuringWait(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("os.Interrupt delivery is not supported by this subprocess test on Windows")
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "saved")
	store, err := storage.Open(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Add(context.Background(), storage.Registration, "G-TEST"); err != nil {
		t.Fatal(err)
	}
	store.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"ac":[]}`) }))
	defer server.Close()
	deadline, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	child := exec.CommandContext(deadline, binary, "-test.run=^TestProcessHelper$", "--", "watch", "run", "--data-dir", dir, "--max-requests", "10", "--interval", "1h", "--json")
	child.Env = append(os.Environ(), "SKY_PROCESS_HELPER=1", "SKY_TEST_SIGNALS=1", "SKY_TEST_URL="+server.URL)
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	child.Stderr = &stderr
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	interrupted, stopped := false, false
	for scanner.Scan() {
		var event struct {
			Type, Reason string
			RequestsUsed int `json:"requests_used"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		if event.Type == "progress" && !interrupted {
			if err := child.Process.Signal(os.Interrupt); err != nil {
				t.Fatal(err)
			}
			interrupted = true
		}
		if event.Type == "stopped" {
			stopped = event.Reason == "canceled" && event.RequestsUsed == 1
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	err = child.Wait()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 130 || !interrupted || !stopped {
		t.Fatalf("interrupt: err=%v interrupted=%v stopped=%v stderr=%s", err, interrupted, stopped, stderr.String())
	}
}
