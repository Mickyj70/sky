package provider

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Source is the boundary used by the CLI and future tracking scheduler.
// Implementations must honor context cancellation and return typed *Error values.
type Source interface {
	Lookup(context.Context, Query) (Result, error)
}

type QueryKind string

const (
	ByRegistration QueryKind = "registration"
	ByHex          QueryKind = "hex"
	ByCallsign     QueryKind = "callsign"
	Nearby         QueryKind = "nearby"
)

type Query struct {
	Kind                          QueryKind
	Identifier                    string
	Latitude, Longitude, RadiusKM float64
}

var (
	registrationPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{1,11}$`)
	hexPattern          = regexp.MustCompile(`^[a-f0-9]{6}$`)
	callsignPattern     = regexp.MustCompile(`^[A-Z0-9]{2,8}$`)
	aircraftHexPattern  = regexp.MustCompile(`^~?[a-fA-F0-9]{6}$`)
)

func (q Query) path() (string, error) {
	id := strings.ToUpper(strings.TrimSpace(q.Identifier))
	switch q.Kind {
	case ByRegistration:
		if registrationPattern.MatchString(id) {
			return "/v2/reg/" + id, nil
		}
	case ByHex:
		id = strings.ToLower(id)
		if hexPattern.MatchString(id) {
			return "/v2/hex/" + id, nil
		}
	case ByCallsign:
		if callsignPattern.MatchString(id) {
			return "/v2/callsign/" + id, nil
		}
	case Nearby:
		finite := func(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }
		if finite(q.Latitude) && finite(q.Longitude) && finite(q.RadiusKM) && q.Latitude >= -90 && q.Latitude <= 90 && q.Longitude >= -180 && q.Longitude <= 180 && q.RadiusKM > 0 && q.RadiusKM <= 463 {
			return fmt.Sprintf("/v2/point/%s/%s/%s", strconv.FormatFloat(q.Latitude, 'f', -1, 64), strconv.FormatFloat(q.Longitude, 'f', -1, 64), strconv.FormatFloat(q.RadiusKM/1.852, 'f', 6, 64)), nil
		}
	}
	return "", fmt.Errorf("invalid %q aircraft query", q.Kind)
}
