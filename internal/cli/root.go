package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Mickyj70/sky/internal/provider"
	"github.com/Mickyj70/sky/internal/tracking"
	"github.com/Mickyj70/sky/internal/version"
	"github.com/spf13/cobra"
)

var hexPattern = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)
var registrationPattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9-]{1,11}$`)
var callsignPattern = regexp.MustCompile(`^[A-Z0-9]{2,8}$`)

func New() *cobra.Command {
	return NewWithSource(&provider.Client{HTTP: &http.Client{}, BaseURL: provider.BaseURL, APIKey: os.Getenv("SKY_API_KEY")})
}

// NewWithSource allows embedding and offline integration tests without a
// production environment override that could send API credentials elsewhere.
func NewWithSource(source provider.Source) *cobra.Command { return newWithClock(source, nil) }

func newWithClock(source provider.Source, clock tracking.Clock) *cobra.Command {
	var asJSON bool
	var timeout time.Duration
	root := &cobra.Command{
		Use: "sky", Short: "Track observed aircraft from your terminal",
		Long:    "Sky looks up observed aircraft using adsb.lol.\nPositions may be missing or stale; no observation does not mean landed.\nData: adsb.lol, ODbL 1.0 — https://www.adsb.lol/docs/open-data/api",
		Version: version.Version, SilenceErrors: true, SilenceUsage: true,
	}
	root.PersistentFlags().BoolVar(&asJSON, "json", false, "Write structured JSON instead of a table")
	root.PersistentFlags().DurationVar(&timeout, "timeout", 15*time.Second, "HTTP request timeout")
	fetch := func(cmd *cobra.Command, query provider.Query, prefix string, airborne bool) error {
		if timeout <= 0 {
			return fmt.Errorf("timeout must be positive")
		}
		ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
		defer cancel()
		result, err := source.Lookup(ctx, query)
		if err != nil {
			return &executionError{err}
		}
		filtered := make([]provider.Aircraft, 0, len(result.Aircraft))
		for _, a := range result.Aircraft {
			if prefix != "" && !strings.HasPrefix(a.Callsign, prefix) {
				continue
			}
			if airborne {
				// Only a numeric altitude establishes an airborne observation here.
				var altitude float64
				if len(a.Altitude) == 0 || string(a.Altitude) == "null" || json.Unmarshal(a.Altitude, &altitude) != nil {
					continue
				}
			}
			filtered = append(filtered, a)
		}
		result.Aircraft = filtered
		sort.Slice(result.Aircraft, func(i, j int) bool { return result.Aircraft[i].Hex < result.Aircraft[j].Hex })
		if asJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(result); err != nil {
				return &executionError{err}
			}
			return nil
		}
		if err := render(cmd.OutOrStdout(), result); err != nil {
			return &executionError{err}
		}
		return nil
	}
	var hex bool
	plane := &cobra.Command{Use: "plane REGISTRATION", Short: "Find an observed aircraft by registration or ICAO hex", Args: cobra.ExactArgs(1), Example: "  sky plane G-VBEL\n  sky plane --hex 4073ce --json", RunE: func(cmd *cobra.Command, args []string) error {
		id := strings.ToUpper(strings.TrimSpace(args[0]))
		kind := provider.ByRegistration
		if hex {
			if !hexPattern.MatchString(id) {
				return fmt.Errorf("ICAO hex must contain exactly six hexadecimal characters")
			}
			id = strings.ToLower(id)
			kind = provider.ByHex
		} else if !registrationPattern.MatchString(id) {
			return fmt.Errorf("invalid registration; use --hex for a six-character ICAO address")
		}
		return fetch(cmd, provider.Query{Kind: kind, Identifier: id}, "", false)
	}}
	plane.Flags().BoolVar(&hex, "hex", false, "Interpret the identifier as an ICAO hex address")
	callsign := &cobra.Command{Use: "callsign CALLSIGN", Short: "Look up a broadcast callsign (for example BAW123)", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		id := strings.ToUpper(strings.TrimSpace(args[0]))
		if !callsignPattern.MatchString(id) {
			return fmt.Errorf("callsign must contain 2–8 letters or digits")
		}
		return fetch(cmd, provider.Query{Kind: provider.ByCallsign, Identifier: id}, "", false)
	}}
	// Airline filtering is deliberately local to a requested area. A callsign
	// prefix identifies observed operator activity, not an authoritative fleet.
	area := func(airline bool) *cobra.Command {
		var lat, lon, radius float64
		var airborne bool
		use, short := "nearby [PLACE]", "Find observed aircraft around a named place or coordinates"
		long := ""
		example := "  sky nearby --lat 6.58 --lon 3.32 --radius-km 100\n  sky nearby --lat 51.47 --lon -0.45 --radius-km 100 --airborne --json"
		if airline {
			use = "airline ICAO_CODE"
			short = "Filter nearby traffic by a three-letter operator callsign prefix"
			long = "Show aircraft observed in the requested region whose broadcast callsigns begin with the given three-letter ICAO operator prefix."
			example = "  sky airline BAW --lat 51.47 --lon -0.45 --radius-km 100 --airborne --json"
		}
		cmd := &cobra.Command{Use: use, Short: short, Long: long, Example: example, Args: cobra.NoArgs}
		if airline {
			cmd.Args = cobra.ExactArgs(1)
		} else {
			cmd.Args = cobra.MaximumNArgs(1)
		}
		cmd.Flags().Float64Var(&lat, "lat", 0, "Latitude (-90 to 90)")
		cmd.Flags().Float64Var(&lon, "lon", 0, "Longitude (-180 to 180)")
		cmd.Flags().Float64Var(&radius, "radius-km", 100, "Search radius in kilometres (maximum 463)")
		cmd.Flags().BoolVar(&airborne, "airborne", false, "Include only aircraft reporting numeric barometric altitude")
		var name string
		cmd.Flags().StringVar(&name, "state", "", "Named area: Abuja/FCT, Lagos or Heathrow")
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			selected := cmd.Flags().Changed("state")
			placeName := name
			if !airline && len(args) == 1 {
				if selected {
					return fmt.Errorf("choose a place argument or --state")
				}
				selected, placeName = true, args[0]
			}
			hasLat, hasLon := cmd.Flags().Changed("lat"), cmd.Flags().Changed("lon")
			latitude, longitude := lat, lon
			if selected {
				if hasLat || hasLon {
					return fmt.Errorf("choose a named place or --lat and --lon")
				}
				p, err := namedPlace(placeName)
				if err != nil {
					return err
				}
				latitude, longitude = p.lat, p.lon
			} else if !hasLat || !hasLon {
				return fmt.Errorf("supply a named place or both --lat and --lon")
			}
			if math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90 {
				return fmt.Errorf("latitude must be between -90 and 90")
			}
			if math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180 {
				return fmt.Errorf("longitude must be between -180 and 180")
			}
			if !validRadius(radius) {
				return fmt.Errorf("radius-km must be greater than 0 and at most 463")
			}
			prefix := ""
			if airline {
				prefix = strings.ToUpper(args[0])
				if !regexp.MustCompile(`^[A-Z]{3}$`).MatchString(prefix) {
					return fmt.Errorf("use a three-letter ICAO operator code, for example UAL or BAW")
				}
			}
			return fetch(cmd, provider.Query{Kind: provider.Nearby, Latitude: latitude, Longitude: longitude, RadiusKM: radius}, prefix, airborne)
		}
		return cmd
	}
	root.AddCommand(historyCommand(&asJSON), plane, callsign, area(false), area(true), watchCommand(&asJSON, source, &timeout, clock))
	return root
}

func render(w io.Writer, result provider.Result) error {
	snapshot := "unknown"
	if result.SnapshotAt != nil {
		snapshot = result.SnapshotAt.Format(time.RFC3339)
	}
	if _, err := fmt.Fprintf(w, "Source: %s | Snapshot: %s | Retrieved: %s\n", result.Source, snapshot, result.RetrievedAt.Format(time.RFC3339)); err != nil {
		return err
	}
	if len(result.Aircraft) == 0 {
		_, err := fmt.Fprintln(w, "No matching aircraft currently observed. This does not establish flight status.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "HEX\tREGISTRATION\tCALLSIGN\tTYPE\tALT FT\tSPEED KT\tTRACK DEG\tLAT\tLON\tSEEN S\tPOS AGE S")
	number := func(v *float64) string {
		if v == nil {
			return "—"
		}
		return strconv.FormatFloat(*v, 'f', 1, 64)
	}
	coordinate := func(v *float64) string {
		if v == nil {
			return "—"
		}
		return strconv.FormatFloat(*v, 'f', 5, 64)
	}
	label := func(s string) string {
		if s == "" {
			return "—"
		}
		return strings.Map(func(r rune) rune {
			if r < 32 || r == 127 {
				return -1
			}
			return r
		}, s)
	}
	for _, a := range result.Aircraft {
		alt := "—"
		if len(a.Altitude) > 0 && string(a.Altitude) != "null" {
			alt = strings.Trim(string(a.Altitude), `"`)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", label(a.Hex), label(a.Registration), label(a.Callsign), label(a.Type), label(alt), number(a.Speed), number(a.Track), coordinate(a.Latitude), coordinate(a.Longitude), number(a.Seen), number(a.PositionSeen))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, "Data: adsb.lol (ODbL 1.0). Seen/position age are seconds before the provider snapshot; — means unavailable.")
	return err
}
