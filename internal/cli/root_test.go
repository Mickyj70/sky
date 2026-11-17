package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func TestCoordinateHelpExamples(t *testing.T) {
	cases := []struct {
		command string
		want    string
	}{
		{"nearby", "sky nearby --lat 51.47 --lon -0.45 --radius-km 100 --airborne --json"},
		{"airline", "sky airline BAW --lat 51.47 --lon -0.45 --radius-km 100 --airborne --json"},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			cmd := New()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetArgs([]string{tc.command, "--help"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Errorf("help is missing the example %q", tc.want)
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
		fmt.Fprint(w, `{"ac":[{"hex":"000001","flight":"BAW123  ","alt_baro":10000},{"hex":"000002","flight":"BAW456","alt_baro":"ground"},{"hex":"000003","flight":"UAL1","alt_baro":20000},{"hex":"000004","flight":"BAW789"}],"now":1700000000000}`)
	}))
	defer server.Close()
	cmd := newCommand(server.URL)
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
	if len(result.Aircraft) != 1 || result.Aircraft[0].Callsign != "BAW123" {
		t.Fatalf("unexpected operator filtering: %+v", result.Aircraft)
	}
}
