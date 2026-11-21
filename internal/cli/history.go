package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Mickyj70/sky/internal/history"
	"github.com/Mickyj70/sky/internal/storage"
	"github.com/spf13/cobra"
)

func historyCommand(asJSON *bool) *cobra.Command {
	var dataDir string
	root := &cobra.Command{Use: "history", Short: "Manage locally recorded snapshots (offline; separate from watchlists)"}
	root.PersistentFlags().StringVar(&dataDir, "data-dir", "", "History data directory (overrides SKY_DATA_DIR and platform default)")
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
	emit := func(cmd *cobra.Command, value any, text string) error {
		var err error
		if *asJSON {
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			err = encoder.Encode(value)
		} else {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), text)
		}
		if err != nil {
			return &executionError{err}
		}
		return nil
	}
	status := &cobra.Command{Use: "status", Short: "Show stored snapshot count and allocated database bytes", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := resolve(cmd)
		if err != nil {
			return err
		}
		s, err := history.Open(cmd.Context(), dir)
		if err != nil {
			return &executionError{err}
		}
		defer s.Close()
		stats, err := s.Stats(cmd.Context())
		if err != nil {
			return &executionError{err}
		}
		return emit(cmd, stats, fmt.Sprintf("%d locally recorded snapshot(s); %d allocated bytes.\nDatabase: %s", stats.Count, stats.Bytes, stats.Database))
	}}
	backup := &cobra.Command{Use: "backup DESTINATION", Short: "Snapshot history only; destination must not exist", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(args[0]) == "" {
			return fmt.Errorf("backup destination must not be blank")
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
		path, err := s.Backup(cmd.Context(), args[0])
		if err != nil {
			return &executionError{err}
		}
		return emit(cmd, struct {
			Backup string `json:"backup"`
		}{path}, "History backup: "+path)
	}}
	restore := &cobra.Command{Use: "restore BACKUP", Short: "Restore history without overwriting existing history", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(args[0]) == "" {
			return fmt.Errorf("backup source must not be blank")
		}
		dir, err := resolve(cmd)
		if err != nil {
			return err
		}
		path, count, err := history.Restore(cmd.Context(), args[0], dir)
		if err != nil {
			return &executionError{err}
		}
		return emit(cmd, struct {
			Database string `json:"database"`
			Count    int64  `json:"count"`
		}{path, count}, fmt.Sprintf("Restored %d history snapshot(s).\nDatabase: %s", count, path))
	}}
	root.AddCommand(status, backup, restore)
	historyQueries(root, asJSON, resolve)
	return root
}
