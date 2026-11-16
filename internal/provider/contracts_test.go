package provider

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLookupQueries(t *testing.T) {
	fixture, err := os.ReadFile("testdata/aircraft.json")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		query Query
		path  string
	}{
		{Query{Kind: ByRegistration, Identifier: " g-test "}, "/v2/reg/G-TEST"},
		{Query{Kind: ByHex, Identifier: "ABC123"}, "/v2/hex/abc123"},
		{Query{Kind: ByCallsign, Identifier: " baw123 "}, "/v2/callsign/BAW123"},
		{Query{Kind: Nearby, Latitude: 0, Longitude: 0, RadiusKM: 100}, "/v2/point/0/0/53.995680"},
		{Query{Kind: Nearby, Latitude: -90, Longitude: 180, RadiusKM: 463}, "/v2/point/-90/180/250.000000"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					t.Errorf("got path %s, want %s", r.URL.Path, tc.path)
				}
				if r.Header.Get("Accept") != "application/json" {
					t.Error("missing JSON accept header")
				}
				if r.Header.Get("x-api-key") != "" {
					t.Error("unexpected API credential")
				}
				w.Write(fixture)
			}))
			defer server.Close()
			c := Client{BaseURL: server.URL, HTTP: server.Client()}
			result, err := c.Lookup(context.Background(), tc.query)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Aircraft) != 5 {
				t.Fatalf("unexpected result: %+v", result)
			}
			if result.Aircraft[0].Callsign != "UAL123" {
				t.Error("untrimmed callsign")
			}
			if result.Aircraft[0].Latitude == nil || *result.Aircraft[0].Latitude != 0 {
				t.Error("zero coordinate lost")
			}
			if result.Aircraft[2].Altitude != nil || result.Aircraft[2].Speed != nil {
				t.Error("unknown readings must stay absent")
			}
			if result.SnapshotAt == nil || result.RetrievedAt.IsZero() {
				t.Error("missing result timestamps")
			}
		})
	}
}

func TestInvalidQueryMakesNoRequest(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer server.Close()
	c := Client{BaseURL: server.URL, HTTP: server.Client()}
	for _, query := range []Query{{}, {Kind: ByHex, Identifier: "../"}, {Kind: ByRegistration, Identifier: "a/b"}, {Kind: Nearby, RadiusKM: 464}} {
		_, err := c.Lookup(context.Background(), query)
		var e *Error
		if !errors.As(err, &e) || e.Kind != Rejected {
			t.Fatalf("expected rejected query, got %v", err)
		}
	}
	if calls != 0 {
		t.Fatalf("invalid queries made %d network calls", calls)
	}
}

func TestErrorClassification(t *testing.T) {
	for _, tc := range []struct {
		status    int
		kind      ErrorKind
		retryable bool
	}{
		{401, Unauthorized, false}, {403, Unauthorized, false}, {400, Rejected, false},
		{429, RateLimited, true}, {408, Unavailable, true}, {503, Unavailable, true},
	} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", "60")
				w.WriteHeader(tc.status)
			}))
			defer server.Close()
			c := Client{BaseURL: server.URL, HTTP: server.Client()}
			before := time.Now()
			_, err := c.Lookup(context.Background(), Query{Kind: ByHex, Identifier: "abc123"})
			var e *Error
			if !errors.As(err, &e) || e.Kind != tc.kind || e.StatusCode != tc.status || e.Retryable() != tc.retryable {
				t.Fatalf("unexpected error: %#v", err)
			}
			if (tc.status == 429 || tc.status == 503) && (e.RetryAt == nil || e.RetryAt.Before(before.Add(59*time.Second))) {
				t.Error("missing cooldown deadline")
			}
		})
	}
}

func TestInvalidResponses(t *testing.T) {
	cases := []string{
		`null`, `{"ac":null}`, `{"ac":{}}`, `{"ac":[null]}`, `{"ac":[{}]}`,
		`{"ac":[{"hex":"abc123","alt_baro":"unknown"}]}`,
		`{"ac":[{"hex":"abc123","gs":"fast"}]}`,
		`{"ac":[{"hex":"abc123","lat":91}]}`,
		`{"ac":[{"hex":"abc123","lon":-181}]}`,
		`{"ac":[{"hex":"abc123","seen":-1}]}`,
		`{"ac":[],"now":-1}`, `{"ac":[],"now":9223372036854775807}`, `{"ac":[]} trailing`,
	}
	for _, body := range cases {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer server.Close()
			c := Client{BaseURL: server.URL, HTTP: server.Client()}
			_, err := c.Lookup(context.Background(), Query{Kind: ByHex, Identifier: "abc123"})
			var e *Error
			if !errors.As(err, &e) || e.Kind != InvalidResponse || e.Retryable() {
				t.Fatalf("expected nonretryable invalid response, got %#v", err)
			}
		})
	}
}

func TestOversizedResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, strings.Repeat(" ", 16*1024*1024+1)) }))
	defer server.Close()
	c := Client{BaseURL: server.URL, HTTP: server.Client()}
	_, err := c.Lookup(context.Background(), Query{Kind: ByHex, Identifier: "abc123"})
	var e *Error
	if !errors.As(err, &e) || e.Kind != InvalidResponse || !strings.Contains(err.Error(), "16 MiB") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInFlightCancellation(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-r.Context().Done() }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	go func() {
		select {
		case <-started:
			cancel()
		case <-ctx.Done():
		}
	}()
	c := Client{BaseURL: server.URL, HTTP: server.Client()}
	_, err := c.Lookup(ctx, Query{Kind: ByHex, Identifier: "abc123"})
	var e *Error
	if !errors.Is(err, context.Canceled) || !errors.As(err, &e) || e.Kind != Canceled || e.Retryable() {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (failingBody) Close() error             { return nil }

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestBodyReadFailure(t *testing.T) {
	c := Client{BaseURL: "https://example.invalid", HTTP: &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Body: failingBody{}, Header: make(http.Header)}, nil
	})}}
	_, err := c.Lookup(context.Background(), Query{Kind: ByHex, Identifier: "abc123"})
	var e *Error
	if !errors.As(err, &e) || e.Kind != Unavailable || e.StatusCode != 200 || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("unexpected read failure: %v", err)
	}
}

func TestRetryAfter(t *testing.T) {
	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		header string
		want   *time.Time
	}{
		{"60", timePointer(now.Add(time.Minute))}, {"0", timePointer(now)},
		{now.Add(2 * time.Minute).Format(http.TimeFormat), timePointer(now.Add(2 * time.Minute))},
		{now.Add(-time.Minute).Format(http.TimeFormat), timePointer(now)},
		{"-1", nil}, {"garbage", nil}, {"1.5", nil}, {"9223372036854775807", nil}, {"", nil},
	} {
		t.Run(tc.header, func(t *testing.T) {
			got := retryAt(tc.header, now)
			if (got == nil) != (tc.want == nil) || (got != nil && !got.Equal(*tc.want)) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}
func timePointer(t time.Time) *time.Time { return &t }

func TestEscapedGroundAltitude(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"ac":[{"hex":"abc123","alt_baro":"gr\u006fund"}]}`)
	}))
	defer server.Close()
	c := Client{BaseURL: server.URL, HTTP: server.Client()}
	result, err := c.Lookup(context.Background(), Query{Kind: ByHex, Identifier: "abc123"})
	if err != nil {
		t.Fatal(err)
	}
	if string(result.Aircraft[0].Altitude) != `"ground"` {
		t.Fatalf("unexpected altitude: %s", result.Aircraft[0].Altitude)
	}
}
