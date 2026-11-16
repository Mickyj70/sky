package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Mickyj70/sky/internal/provider"
)

func TestInvalidInput(t *testing.T) {
	cases := [][]string{
		{"plane", "abc/def"}, {"plane", "--hex", "123"}, {"plane"},
		{"callsign", "BAW/123"}, {"nearby", "--lat", "91", "--lon", "0"},
		{"nearby", "--lat", "0"}, {"nearby", "--lat", "NaN", "--lon", "0"},
		{"nearby", "--lat", "0", "--lon", "0", "--radius-km", "464"},
		{"nearby", "--lat", "0", "--lon", "0", "--radius-km", "0"},
		{"airline", "UA", "--lat", "0", "--lon", "0"}, {"plane", "N123AB", "--timeout", "0s"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			c := New()
			c.SetArgs(args)
			c.SetOut(&bytes.Buffer{})
			c.SetErr(&bytes.Buffer{})
			if err := c.Execute(); err == nil {
				t.Fatal("invalid input accepted")
			}
		})
	}
}

func TestRenderMissingVersusZero(t *testing.T) {
	zero := 0.0
	r := provider.Result{Source: "adsb.lol", RetrievedAt: time.Unix(0, 0).UTC(), Aircraft: []provider.Aircraft{{Hex: "abc123", Altitude: json.RawMessage(`"ground"`), Speed: &zero, Seen: &zero}}}
	var b bytes.Buffer
	if err := render(&b, r); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"ground", "0.0", "—", "POS AGE S", "Snapshot: unknown"} {
		if !strings.Contains(b.String(), s) {
			t.Errorf("missing %q in %s", s, b.String())
		}
	}
}

func TestAirlineQuery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/point/0/0/53.995680" {
			t.Errorf("incorrect kilometre conversion or coordinates: %s", r.URL.Path)
		}
		fmt.Fprint(w, `{"ac":[{"hex":"000001","flight":"BAW123  ","alt_baro":10000},{"hex":"000002","flight":"BAW456","alt_baro":"ground"},{"hex":"000003","flight":"UAL1","alt_baro":20000},{"hex":"000004","flight":"BAW789"},{"hex":"000005","flight":"BAW000","alt_baro":0}],"now":1700000000000}`)
	}))
	defer server.Close()
	cmd := NewWithSource(&provider.Client{BaseURL: server.URL, HTTP: server.Client()})
	cmd.SetArgs([]string{"airline", "BAW", "--lat", "0", "--lon", "0", "--airborne", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result provider.Result
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Aircraft) != 2 || result.Aircraft[0].Callsign != "BAW123" || result.Aircraft[1].Callsign != "BAW000" {
		t.Fatalf("unexpected operator filtering: %+v", result.Aircraft)
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

type sourceFunc func(context.Context, provider.Query) (provider.Result, error)

func (f sourceFunc) Lookup(ctx context.Context, q provider.Query) (provider.Result, error) {
	return f(ctx, q)
}

func TestOutputFailureExitCode(t *testing.T) {
	source := sourceFunc(func(context.Context, provider.Query) (provider.Result, error) {
		return provider.Result{Source: "test", Aircraft: []provider.Aircraft{}}, nil
	})
	for _, format := range []string{"table", "json"} {
		t.Run(format, func(t *testing.T) {
			cmd := NewWithSource(source)
			args := []string{"plane", "N123AB"}
			if format == "json" {
				args = append(args, "--json")
			}
			cmd.SetArgs(args)
			cmd.SetOut(brokenWriter{})
			err := Execute(context.Background(), cmd)
			if !errors.Is(err, io.ErrClosedPipe) || ExitCode(err) != 1 {
				t.Fatalf("expected output failure exit 1, got %v (exit %d)", err, ExitCode(err))
			}
		})
	}
}
