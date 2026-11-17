package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/Mickyj70/sky/internal/storage"
	"github.com/spf13/cobra"
)

func watchCommand(asJSON *bool) *cobra.Command {
	var dataDir string
	watch := &cobra.Command{Use: "watch", Short: "Manage saved aircraft locally (no automatic polling yet)"}
	watch.PersistentFlags().StringVar(&dataDir, "data-dir", "", "Watchlist data directory (overrides SKY_DATA_DIR and platform default)")
	resolve := func(cmd *cobra.Command) (string, error) {
		if cmd.Flags().Changed("data-dir") && strings.TrimSpace(dataDir) == "" {
			return "", fmt.Errorf("data-dir must not be blank")
		}
		dir, err := storage.DataDir(dataDir)
		if err != nil {
			return "", &executionError{err}
		}
		return dir, nil
	}
	emit := func(cmd *cobra.Command, value any) error {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(value); err != nil {
			return &executionError{err}
		}
		return nil
	}
	open := func(cmd *cobra.Command) (*storage.Store, error) {
		dir, err := resolve(cmd)
		if err != nil {
			return nil, err
		}
		s, err := storage.Open(cmd.Context(), dir)
		if err != nil {
			return nil, &executionError{err}
		}
		return s, nil
	}
	mutation := func(remove bool) *cobra.Command {
		name, short := "add", "Save an aircraft registration or ICAO hex"
		if remove {
			name = "remove"
			short = "Remove a saved registration or ICAO hex"
		}
		var registration, hex string
		cmd := &cobra.Command{Use: name, Short: short, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
			kind, id := storage.Registration, registration
			if cmd.Flags().Changed("hex") {
				kind, id = storage.Hex, hex
			}
			id, err := storage.Normalize(kind, id)
			if err != nil {
				return err
			}
			s, err := open(cmd)
			if err != nil {
				return err
			}
			defer s.Close()
			if remove {
				changed, err := s.Remove(cmd.Context(), kind, id)
				if err != nil {
					return &executionError{err}
				}
				if *asJSON {
					return emit(cmd, struct {
						Database   string       `json:"database"`
						Kind       storage.Kind `json:"kind"`
						Identifier string       `json:"identifier"`
						Changed    bool         `json:"changed"`
					}{s.Path(), kind, id, changed})
				}
				message := "Not saved"
				if changed {
					message = "Removed"
				}
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\nDatabase: %s\n", message, kind, id, s.Path()); err != nil {
					return &executionError{err}
				}
				return nil
			}
			target, changed, err := s.Add(cmd.Context(), kind, id)
			if err != nil {
				return &executionError{err}
			}
			if *asJSON {
				return emit(cmd, struct {
					Database string         `json:"database"`
					Target   storage.Target `json:"target"`
					Changed  bool           `json:"changed"`
				}{s.Path(), target, changed})
			}
			message := "Already saved"
			if changed {
				message = "Saved"
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s\nDatabase: %s\n", message, kind, id, s.Path()); err != nil {
				return &executionError{err}
			}
			return nil
		}}
		cmd.Flags().StringVar(&registration, "registration", "", "Aircraft registration, for example G-ZBKE")
		cmd.Flags().StringVar(&hex, "hex", "", "Six-character ICAO hex address, for example 406d7b")
		cmd.MarkFlagsMutuallyExclusive("registration", "hex")
		cmd.MarkFlagsOneRequired("registration", "hex")
		return cmd
	}
	list := &cobra.Command{Use: "list", Short: "List saved targets without querying a provider", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		s, err := open(cmd)
		if err != nil {
			return err
		}
		defer s.Close()
		saved, err := s.List(cmd.Context())
		if err != nil {
			return &executionError{err}
		}
		if *asJSON {
			return emit(cmd, struct {
				Database string           `json:"database"`
				Targets  []storage.Target `json:"targets"`
			}{s.Path(), saved})
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "KIND\tIDENTIFIER\tCREATED UTC")
		for _, target := range saved {
			fmt.Fprintf(w, "%s\t%s\t%s\n", target.Kind, target.Identifier, target.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
		}
		if err := w.Flush(); err != nil {
			return &executionError{err}
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%d saved target(s). Database: %s\n", len(saved), s.Path()); err != nil {
			return &executionError{err}
		}
		return nil
	}}
	backup := &cobra.Command{Use: "backup DESTINATION", Short: "Create a consistent SQLite snapshot; destination must not exist", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(args[0]) == "" {
			return fmt.Errorf("backup destination must not be blank")
		}
		s, err := open(cmd)
		if err != nil {
			return err
		}
		defer s.Close()
		path, err := s.Backup(cmd.Context(), args[0])
		if err != nil {
			return &executionError{err}
		}
		if *asJSON {
			return emit(cmd, struct {
				Backup string `json:"backup"`
			}{path})
		}
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Backup:", path); err != nil {
			return &executionError{err}
		}
		return nil
	}}
	restore := &cobra.Command{Use: "restore BACKUP", Short: "Restore into a new data directory; never overwrite existing data", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(args[0]) == "" {
			return fmt.Errorf("backup source must not be blank")
		}
		dir, err := resolve(cmd)
		if err != nil {
			return err
		}
		path, count, err := storage.Restore(cmd.Context(), args[0], dir)
		if err != nil {
			return &executionError{err}
		}
		if *asJSON {
			return emit(cmd, struct {
				Database string `json:"database"`
				Count    int    `json:"count"`
			}{path, count})
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Restored %d target(s). Database: %s\nUse --data-dir %q or SKY_DATA_DIR to select the restored watchlist.\n", count, path, dir); err != nil {
			return &executionError{err}
		}
		return nil
	}}
	watch.AddCommand(mutation(false), mutation(true), list, backup, restore)
	return watch
}
