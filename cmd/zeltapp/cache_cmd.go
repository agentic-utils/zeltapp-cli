package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func cacheCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage the local TTL cache",
	}
	cmd.AddCommand(cacheListCmd(), cacheClearCmd())
	return cmd
}

func cacheListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show what's in the cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := cacheDir()
			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintln(outWriter, "(empty)")
					return nil
				}
				return err
			}
			rows := make([]any, 0, len(entries))
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				b, err := os.ReadFile(filepath.Join(dir, e.Name()))
				if err != nil {
					continue
				}
				var entry cacheEntry
				if err := json.Unmarshal(b, &entry); err != nil {
					continue
				}
				expiry := "expired"
				if remaining := time.Until(entry.ExpiresAt); remaining > 0 {
					expiry = remaining.Round(time.Second).String()
				}
				rows = append(rows, map[string]any{
					"path":          entry.Path,
					"storedAt":      entry.StoredAt.Format(time.RFC3339),
					"expiresIn":     expiry,
					"size":          len(entry.Body),
				})
			}
			if len(rows) == 0 {
				fmt.Fprintln(outWriter, "(empty)")
				return nil
			}
			return printResult(rows)
		},
	}
}

func cacheClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Remove all cache entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cacheClear(); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "cache cleared")
			return nil
		},
	}
}
