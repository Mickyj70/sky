package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLookupDoesNotFollowRedirects(t *testing.T) {
	for _, status := range []int{301, 302, 303, 307, 308} {
		for _, injected := range []bool{false, true} {
			t.Run(fmt.Sprintf("status-%d/injected-%v", status, injected), func(t *testing.T) {
				var destinations, callbacks, origins atomic.Int32
				destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { destinations.Add(1); fmt.Fprint(w, `{"ac":[]}`) }))
				defer destination.Close()
				// Different host as well as port; synthetic key, no external network.
				location := strings.Replace(destination.URL, "127.0.0.1", "localhost", 1)
				origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { origins.Add(1); http.Redirect(w, r, location, status) }))
				defer origin.Close()
				c := Client{BaseURL: origin.URL, APIKey: "synthetic-test-key"}
				if injected {
					c.HTTP = &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { callbacks.Add(1); return nil }}
				}
				_, err := c.Lookup(context.Background(), Query{Kind: ByHex, Identifier: "abc123"})
				var upstream *Error
				if !errors.As(err, &upstream) || upstream.Kind != Rejected || upstream.StatusCode != status {
					t.Fatalf("expected rejected redirect %d, got %v", status, err)
				}
				if destinations.Load() != 0 || origins.Load() != 1 || callbacks.Load() != 0 {
					t.Fatalf("unexpected HTTP calls: origins=%d destinations=%d callbacks=%d", origins.Load(), destinations.Load(), callbacks.Load())
				}
				if injected {
					// The caller's client/policy remains reusable outside this adapter.
					if c.HTTP.CheckRedirect == nil || c.HTTP.Timeout != time.Second {
						t.Fatal("caller client was modified")
					}
					c.HTTP.CheckRedirect(nil, nil)
					if callbacks.Load() != 1 {
						t.Fatal("caller redirect policy was replaced")
					}
				}
			})
		}
	}
}

func TestLookupDoesNotFollowSameOriginRedirects(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Redirect(w, r, "/redirected", http.StatusFound)
	}))
	defer server.Close()
	c := Client{BaseURL: server.URL, HTTP: server.Client()}
	if _, err := c.Lookup(context.Background(), Query{Kind: ByHex, Identifier: "abc123"}); err == nil {
		t.Fatal("expected redirect rejection")
	}
	if calls.Load() != 1 {
		t.Fatalf("lookup made %d requests", calls.Load())
	}
}
