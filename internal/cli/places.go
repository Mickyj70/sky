package cli

import (
	"fmt"
	"math"
	"strings"
	"time"

	_ "time/tzdata"
)

type place struct {
	lat, lon float64
	zone     string
}

func namedPlace(name string) (place, error) {
	switch strings.ToLower(strings.Join(strings.Fields(name), " ")) {
	case "abuja", "fct", "federal capital territory", "abv":
		return place{9.0068, 7.2632, "Africa/Lagos"}, nil
	case "lagos", "lagos state", "los":
		return place{6.5774, 3.3212, "Africa/Lagos"}, nil
	case "heathrow", "lhr":
		return place{51.47, -0.45, "Europe/London"}, nil
	default:
		return place{}, fmt.Errorf("unknown place %q (supported: Abuja/FCT, Lagos, Heathrow)", name)
	}
}

func (p place) contains(lat, lon, radius float64) bool {
	const rad = math.Pi / 180
	a := math.Pow(math.Sin((lat-p.lat)*rad/2), 2) +
		math.Cos(p.lat*rad)*math.Cos(lat*rad)*math.Pow(math.Sin((lon-p.lon)*rad/2), 2)
	return 12742*math.Asin(math.Sqrt(math.Min(a, 1))) <= radius
}

func dayBounds(day, zone string, now time.Time) (time.Time, time.Time, error) {
	location, err := time.LoadLocation(zone)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	if strings.EqualFold(day, "today") {
		day = now.In(location).Format(time.DateOnly)
	}
	start, err := time.ParseInLocation(time.DateOnly, day, location)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("day must be today or YYYY-MM-DD: %w", err)
	}
	return start.UTC(), start.AddDate(0, 0, 1).UTC(), nil
}

func validRadius(radius float64) bool { return radius > 0 && radius <= 463 }
