package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Mickyj70/sky/internal/history"
	"github.com/Mickyj70/sky/internal/storage"
	"github.com/spf13/cobra"
)

func historyQueries(root *cobra.Command, asJSON *bool, resolve func(*cobra.Command) (string, error)) {
	makeQuery := func(export bool) *cobra.Command {
		var registration, hex, since, until, format, state, day string
		var radius float64
		var limit, offset int
		cmd := root
		if export {
			cmd = &cobra.Command{Use: "export", Short: "Export one bounded page of locally collected snapshots"}
		}
		cmd.Args = cobra.MaximumNArgs(1)
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			if export && format != "csv" && format != "json" {
				return fmt.Errorf("format must be csv or json")
			}
			if export && format == "csv" && *asJSON {
				return fmt.Errorf("--json conflicts with --format csv")
			}
			filter := history.Filter{Limit: limit, Offset: offset}
			if cmd.Flags().Changed("registration") {
				filter.Kind = storage.Registration
				filter.Identifier = registration
			}
			if cmd.Flags().Changed("hex") {
				filter.Kind = storage.Hex
				filter.Identifier = hex
			}
			selected := cmd.Flags().Changed("state")
			placeName := state
			if len(args) == 1 {
				if selected {
					return fmt.Errorf("choose a place argument or --state")
				}
				selected, placeName = true, args[0]
			}
			zone := "UTC"
			if selected {
				p, err := namedPlace(placeName)
				if err != nil {
					return err
				}
				if !validRadius(radius) {
					return fmt.Errorf("radius-km must be greater than 0 and at most 463")
				}
				zone = p.zone
				filter.Select = func(record history.Record) bool {
					for _, aircraft := range record.Observation.Result.Aircraft {
						if aircraft.Latitude != nil && aircraft.Longitude != nil &&
							p.contains(*aircraft.Latitude, *aircraft.Longitude, radius) {
							return true
						}
					}
					return false
				}
			} else if cmd.Flags().Changed("radius-km") {
				return fmt.Errorf("radius-km requires a named place")
			}
			now := time.Now().UTC()
			if cmd.Flags().Changed("day") {
				if cmd.Flags().Changed("since") || cmd.Flags().Changed("until") {
					return fmt.Errorf("day cannot be combined with since or until")
				}
				start, end, err := dayBounds(day, zone, now)
				if err != nil {
					return err
				}
				filter.Since, filter.Until = &start, &end
			}
			var err error
			if cmd.Flags().Changed("since") {
				filter.Since, err = historyBound(since, now, true)
				if err != nil {
					return err
				}
			}
			if cmd.Flags().Changed("until") {
				filter.Until, err = historyBound(until, now, false)
				if err != nil {
					return err
				}
			}
			filter, err = filter.Validate()
			if err != nil {
				return err
			}
			dir, err := resolve(cmd)
			if err != nil {
				return err
			}
			s, err := history.Open(cmd.Context(), dir)
			if err != nil {
				return &executionError{err}
			}
			defer s.Close()
			page, err := s.Query(cmd.Context(), filter)
			if err != nil {
				return &executionError{err}
			}
			// Validate/prepare a complete bounded page before writing anything. Errors
			// in stored rows cannot leave a plausible but incomplete JSON/CSV document.
			var out bytes.Buffer
			if export && format == "csv" {
				err = historyCSV(&out, page)
			} else if export || *asJSON {
				encoder := json.NewEncoder(&out)
				encoder.SetIndent("", "  ")
				err = encoder.Encode(page)
			} else {
				err = historyTable(&out, page)
			}
			if err != nil {
				return &executionError{err}
			}
			if _, err = cmd.OutOrStdout().Write(out.Bytes()); err != nil {
				return &executionError{err}
			}
			return nil
		}
		cmd.Flags().StringVar(&state, "state", "", "Named area: Abuja/FCT, Lagos or Heathrow")
		cmd.Flags().StringVar(&day, "day", "", "Whole local day (today or YYYY-MM-DD; UTC without a place)")
		cmd.Flags().Float64Var(&radius, "radius-km", 100, "Radius around the named area")
		cmd.Flags().StringVar(&registration, "registration", "", "Filter saved registration (not an alias for its hex)")
		cmd.Flags().StringVar(&hex, "hex", "", "Filter six-character ICAO hex")
		cmd.MarkFlagsMutuallyExclusive("registration", "hex")
		cmd.Flags().StringVar(&since, "since", "", "Inclusive local encounter bound: RFC3339 or duration such as 24h")
		cmd.Flags().StringVar(&until, "until", "", "Exclusive local encounter bound: RFC3339 with timezone")
		cmd.Flags().IntVar(&limit, "limit", 100, "Snapshots per page (1..1000; maximum 16 MiB payload)")
		cmd.Flags().IntVar(&offset, "offset", 0, "Rows to skip in ascending encounter-time/key order (0..1000000)")
		if export {
			cmd.Flags().StringVar(&format, "format", "json", "Export format: json or csv")
		}
		return cmd
	}
	makeQuery(false)
	root.Example = "  sky history --registration G-ZBKE --since 24h --json\n  sky history export --format csv --limit 100\n  sky history prune --before 2026-10-01T00:00:00Z"
	var before string
	var apply bool
	prune := &cobra.Command{Use: "prune", Short: "Preview old local encounters; delete only with --apply", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		cutoff, err := historyBound(before, time.Now().UTC(), false)
		if err != nil {
			return err
		}
		dir, err := resolve(cmd)
		if err != nil {
			return err
		}
		s, err := history.Open(cmd.Context(), dir)
		if err != nil {
			return &executionError{err}
		}
		defer s.Close()
		result, err := s.Prune(cmd.Context(), *cutoff, apply)
		if err != nil {
			return &executionError{err}
		}
		if *asJSON {
			err = json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		} else {
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Before %s: %d matched; %d deleted (apply=%t).\n", result.Before.Format(time.RFC3339Nano), result.Matched, result.Deleted, result.Applied)
		}
		if err != nil {
			return &executionError{err}
		}
		return nil
	}}
	prune.Flags().StringVar(&before, "before", "", "Exclusive first-local-encounter cutoff in RFC3339 with timezone")
	prune.Flags().BoolVar(&apply, "apply", false, "Delete matches; omitted means preview only")
	_ = prune.MarkFlagRequired("before")
	root.AddCommand(makeQuery(true), prune)
}

func historyBound(value string, now time.Time, allowDuration bool) (*time.Time, error) {
	value = strings.TrimSpace(value)
	stamp, err := time.Parse(time.RFC3339Nano, value)
	if err != nil && allowDuration {
		duration, e := time.ParseDuration(value)
		if e == nil && duration > 0 {
			stamp = now.Add(-duration)
			err = nil
		}
	}
	if err != nil || stamp.IsZero() || !time.Unix(0, stamp.UnixNano()).Equal(stamp) {
		return nil, fmt.Errorf("invalid history time %q; use RFC3339 with timezone%s", value, map[bool]string{true: " or a positive duration", false: ""}[allowDuration])
	}
	stamp = stamp.UTC()
	return &stamp, nil
}

func historyTable(out *bytes.Buffer, page history.Page) error {
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "ENCOUNTER UTC\tKIND\tIDENTIFIER\tSTATE\tSOURCE\tAIRCRAFT")
	for _, record := range page.Records {
		o := record.Observation
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%d\n", o.At.Format(time.RFC3339Nano), o.Target.Kind, o.Target.Identifier, o.State, terminalText(o.Result.Source), len(o.Result.Aircraft))
	}
	if err := w.Flush(); err != nil {
		return err
	}
	fmt.Fprintf(out, "%d locally collected snapshot(s); more=%t", len(page.Records), page.HasMore)
	if page.NextOffset != nil {
		fmt.Fprintf(out, "; next --offset %d", *page.NextOffset)
	}
	fmt.Fprintln(out)
	for _, a := range page.Attribution {
		fmt.Fprintf(out, "Data: %s (%s) %s\n", terminalText(a.Source), a.License, a.URL)
	}
	return nil
}

// Prefix formula-like text even after leading spaces/control characters/BOM.
// Numeric values and aircraft JSON are generated internally, not spreadsheet code.
func spreadsheetText(value string) string {
	trimmed := strings.TrimLeftFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) || r == '\ufeff' })
	first, _ := utf8.DecodeRuneInString(trimmed)
	if trimmed != "" && strings.ContainsRune("=+-@", first) {
		return "'" + value
	}
	if strings.HasPrefix(value, "\t") || strings.HasPrefix(value, "\r") || strings.HasPrefix(value, "\n") {
		return "'" + value
	}
	return value
}
func historyCSV(out *bytes.Buffer, page history.Page) error {
	w := csv.NewWriter(out)
	header := []string{"key", "collection", "target_kind", "target_identifier", "encounter_at_utc", "snapshot_at_utc", "retrieved_at_utc", "state", "position_age_seconds", "source", "source_license", "attribution_url", "aircraft_json", "page_has_more", "page_next_offset"}
	if err := w.Write(header); err != nil {
		return err
	}
	for _, record := range page.Records {
		o := record.Observation
		result := o.Result
		attribution := history.SourceAttribution(result.Source)
		snapshot, age, next := "", "", ""
		if result.SnapshotAt != nil {
			snapshot = result.SnapshotAt.Format(time.RFC3339Nano)
		}
		if o.PositionAgeSeconds != nil {
			age = strconv.FormatFloat(*o.PositionAgeSeconds, 'g', -1, 64)
		}
		if page.NextOffset != nil {
			next = strconv.Itoa(*page.NextOffset)
		}
		aircraft, err := json.Marshal(result.Aircraft)
		if err != nil {
			return err
		}
		row := []string{record.Key, "local", string(o.Target.Kind), o.Target.Identifier, o.At.Format(time.RFC3339Nano), snapshot, result.RetrievedAt.Format(time.RFC3339Nano), string(o.State), age, spreadsheetText(result.Source), attribution.License, attribution.URL, string(aircraft), strconv.FormatBool(page.HasMore), next}
		if err = w.Write(row); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}
