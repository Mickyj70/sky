// Package provider retrieves observed aircraft from adsb.lol.
package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Mickyj70/sky/internal/version"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const BaseURL = "https://api.adsb.lol"

// Pointers preserve the difference between an absent reading and a real zero.
// Altitude is raw JSON because the provider returns a number or "ground".
type Aircraft struct {
	Hex          string          `json:"hex"`
	Callsign     string          `json:"flight,omitempty"`
	Registration string          `json:"r,omitempty"`
	Type         string          `json:"t,omitempty"`
	Altitude     json.RawMessage `json:"alt_baro,omitempty"`
	Speed        *float64        `json:"gs,omitempty"`
	Track        *float64        `json:"track,omitempty"`
	Latitude     *float64        `json:"lat,omitempty"`
	Longitude    *float64        `json:"lon,omitempty"`
	Seen         *float64        `json:"seen,omitempty"`
	PositionSeen *float64        `json:"seen_pos,omitempty"`
}

type Result struct {
	Source      string     `json:"source"`
	RetrievedAt time.Time  `json:"retrieved_at"`
	SnapshotAt  *time.Time `json:"snapshot_at,omitempty"`
	Aircraft    []Aircraft `json:"aircraft"`
}

type Client struct {
	HTTP    *http.Client
	BaseURL string
	APIKey  string
}

func (c *Client) Lookup(ctx context.Context, query Query) (Result, error) {
	path, err := query.path()
	if err != nil {
		return Result{}, &Error{Kind: Rejected, Message: err.Error(), Cause: err}
	}
	return c.fetch(ctx, path)
}

func (c *Client) fetch(ctx context.Context, path string) (Result, error) {
	result := Result{Source: "adsb.lol", Aircraft: []Aircraft{}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(c.BaseURL, "/")+path, nil)
	if err != nil {
		return result, &Error{Kind: Rejected, Message: "create request: " + err.Error(), Cause: err}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", version.UserAgent)
	if c.APIKey != "" {
		req.Header.Set("x-api-key", c.APIKey)
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	// Reject redirects at the provider boundary. Clone caller-owned configuration so
	// redirect rejection also applies to injected clients without mutating them.
	requestClient := *client
	requestClient.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := requestClient.Do(req)
	if err != nil {
		return result, transportError("request aircraft data", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		hint := ""
		if retry := resp.Header.Get("Retry-After"); retry != "" {
			hint = " (Retry-After: " + retry + ")"
		}
		return result, &Error{Kind: RateLimited, StatusCode: resp.StatusCode, RetryAt: retryAt(resp.Header.Get("Retry-After"), time.Now()), Message: fmt.Sprintf("adsb.lol rate limit reached%s; wait before retrying or configure SKY_API_KEY", hint)}
	}
	if resp.StatusCode != http.StatusOK {
		kind := Rejected
		switch {
		case resp.StatusCode == 401 || resp.StatusCode == 403:
			kind = Unauthorized
		case resp.StatusCode >= 500 || resp.StatusCode == 408 || resp.StatusCode == 425:
			kind = Unavailable
		}
		return result, &Error{Kind: kind, StatusCode: resp.StatusCode, RetryAt: retryAt(resp.Header.Get("Retry-After"), time.Now()), Message: fmt.Sprintf("adsb.lol returned HTTP %d", resp.StatusCode)}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024+1))
	if err != nil {
		failure := transportError("read response", err)
		failure.StatusCode = resp.StatusCode
		return result, failure
	}
	if len(body) > 16*1024*1024 {
		return result, invalidResponse("aircraft response exceeds 16 MiB", nil)
	}
	var payload struct {
		Aircraft []Aircraft `json:"ac"`
		Now      int64      `json:"now"`
		Message  string     `json:"msg"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return result, invalidResponse("decode aircraft response: "+err.Error(), err)
	}
	if payload.Message != "" && payload.Message != "No error" {
		return result, &Error{Kind: Rejected, StatusCode: resp.StatusCode, Message: "adsb.lol: " + payload.Message}
	}
	// A missing ac field is a schema error; an empty array is a successful lookup.
	if payload.Aircraft == nil {
		return result, invalidResponse("aircraft response is missing the ac array", nil)
	}
	if payload.Now < 0 || payload.Now > 253402300799999 {
		return result, invalidResponse("aircraft snapshot timestamp is outside the supported UTC range", nil)
	}
	for i := range payload.Aircraft {
		if err := validateAircraft(&payload.Aircraft[i]); err != nil {
			return result, invalidResponse(fmt.Sprintf("invalid aircraft at index %d: %v", i, err), err)
		}
	}
	result.Aircraft = payload.Aircraft
	for i := range result.Aircraft {
		result.Aircraft[i].Callsign = strings.TrimSpace(result.Aircraft[i].Callsign)
	}
	result.RetrievedAt = time.Now().UTC()
	if payload.Now > 0 {
		t := time.UnixMilli(payload.Now).UTC()
		result.SnapshotAt = &t
	}
	return result, nil
}

func invalidResponse(message string, cause error) *Error {
	return &Error{Kind: InvalidResponse, StatusCode: http.StatusOK, Message: message, Cause: cause}
}

func transportError(operation string, err error) *Error {
	kind := Unavailable
	var networkError net.Error
	switch {
	case errors.Is(err, context.Canceled):
		kind = Canceled
	case errors.Is(err, context.DeadlineExceeded):
		kind = Timeout
	case errors.As(err, &networkError) && networkError.Timeout():
		kind = Timeout
	}
	return &Error{Kind: kind, Message: operation + ": " + err.Error(), Cause: err}
}

func validateAircraft(a *Aircraft) error {
	if !aircraftHexPattern.MatchString(a.Hex) {
		return fmt.Errorf("missing or invalid hex address")
	}
	if len(a.Altitude) > 0 {
		var altitude any
		if err := json.Unmarshal(a.Altitude, &altitude); err != nil {
			return fmt.Errorf("alt_baro must be a number, ground, or null")
		}
		switch value := altitude.(type) {
		case nil:
			a.Altitude = nil
		case float64:
		case string:
			if value != "ground" {
				return fmt.Errorf("alt_baro must be a number, ground, or null")
			}
			a.Altitude = json.RawMessage(`"ground"`)
		default:
			return fmt.Errorf("alt_baro must be a number, ground, or null")
		}
	}
	if a.Latitude != nil && (*a.Latitude < -90 || *a.Latitude > 90) {
		return fmt.Errorf("latitude outside -90..90")
	}
	if a.Longitude != nil && (*a.Longitude < -180 || *a.Longitude > 180) {
		return fmt.Errorf("longitude outside -180..180")
	}
	for _, v := range []*float64{a.Seen, a.PositionSeen, a.Speed} {
		if v != nil && *v < 0 {
			return fmt.Errorf("age and speed readings must be nonnegative")
		}
	}
	if a.Track != nil && (*a.Track < 0 || *a.Track > 360) {
		return fmt.Errorf("track outside 0..360")
	}
	return nil
}
