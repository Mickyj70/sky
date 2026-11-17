package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetch(t *testing.T) {
	cases := []struct {
		name, body, want string
		status           int
	}{
		{"mixed altitudes", `{"ac":[{"hex":"abc123","flight":"UAL1  ","alt_baro":"ground","gs":0},{"hex":"def456","alt_baro":32000}],"now":1700000000000,"msg":"No error"}`, "", 200},
		{"no observations", `{"ac":[],"msg":"No error"}`, "", 200},
		{"rate limited", "", "rate limit reached (Retry-After: 60)", 429},
		{"upstream unavailable", "unavailable", "HTTP 503", 503},
		{"invalid json", `<html>bad gateway</html>`, "decode aircraft response", 200},
		{"schema changed", `{"aircraft":[]}`, "missing the ac array", 200},
		{"provider error", `{"ac":[],"msg":"Invalid query"}`, "Invalid query", 200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/v2/hex/abc123" {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				if r.Header.Get("x-api-key") != "test-key" {
					t.Error("API key missing")
				}
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			c := Client{BaseURL: server.URL, HTTP: server.Client(), APIKey: "test-key"}
			result, err := c.fetch(context.Background(), "/v2/hex/abc123")
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("got %v, want %s", err, tc.want)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.Aircraft == nil {
				t.Fatal("empty results must encode as an array")
			}
			if tc.name == "mixed altitudes" {
				if result.Aircraft[0].Callsign != "UAL1" {
					t.Error("callsign not trimmed")
				}
				if result.Aircraft[0].Speed == nil || *result.Aircraft[0].Speed != 0 {
					t.Error("zero speed must be preserved")
				}
				if result.Aircraft[1].Speed != nil {
					t.Error("missing speed must stay absent")
				}
				if result.SnapshotAt == nil || result.SnapshotAt.UnixMilli() != 1700000000000 {
					t.Error("incorrect snapshot timestamp")
				}
			}
		})
	}
}

func TestFetchTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	c := Client{BaseURL: server.URL, HTTP: &http.Client{Timeout: 20 * time.Millisecond}}
	if _, err := c.fetch(context.Background(), "/v2/hex/abc123"); err == nil {
		t.Fatal("expected request timeout")
	}
}
