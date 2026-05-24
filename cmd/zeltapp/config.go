package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func configCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "CLI configuration (session, cache)",
	}
	cmd.AddCommand(configViewCmd(), cacheGroupCmd())
	return cmd
}

func configViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view",
		Short: "Show current user, session state, and config locations",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			return withClient(func(c *client) error {
				var me any
				_ = c.do("GET", "/apiv2/auth/me", nil, &me)
				summary := map[string]any{
					"baseURL":     c.baseURL,
					"sessionFile": sessionPath(),
					"cacheDir":    cacheDir(),
					"user":        nil,
				}
				if c.session != nil {
					summary["user"] = map[string]any{
						"displayName": c.session.DisplayName,
						"email":       c.session.Email,
						"userId":      c.session.UserID,
						"companyId":   c.session.CompanyID,
					}
				}
				if m, ok := me.(map[string]any); ok {
					if u, ok := m["user"].(map[string]any); ok {
						if sc, ok := u["scopes2"].([]any); ok {
							summary["scopes"] = sc
						}
					}
				}
				return emit(&resourceView{raw: summary})
			})
		},
	}
}

func cacheGroupCmd() *cobra.Command {
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
		Short: "Show cached entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOutput(); err != nil {
				return err
			}
			dir := cacheDir()
			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					return emit(&resourceView{rows: nil})
				}
				return err
			}
			rows := make([]map[string]any, 0, len(entries))
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
					"path":      entry.Path,
					"storedAt":  entry.StoredAt.Format(time.RFC3339),
					"expiresIn": expiry,
					"sizeBytes": len(entry.Body),
				})
			}
			return emit(&resourceView{
				columns:    []string{"path", "expiresIn", "sizeBytes"},
				rows:       rows,
				nameColumn: "path",
			})
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
			fmt.Fprintln(stderr, "cache cleared")
			return nil
		},
	}
}
