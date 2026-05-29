package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

// Config is AgentGuard's small persisted settings file
// (~/.agentguard/config/config.json). It holds the handful of knobs the
// dashboard settings page and `agentguard config` both read/write: retention
// window, telemetry opt-in, dashboard theme, and cloud-sync status
// (requirements.md §3.10 settings, database.md §7 retention).
type Config struct {
	RetentionDays int    `json:"retention_days"`
	Telemetry     string `json:"telemetry"`  // "off" | "anonymous"
	Theme         string `json:"theme"`      // "system" | "dark" | "light"
	CloudSync     bool   `json:"cloud_sync"` // off by default; opt-in
}

// DefaultConfig is the secure-by-default settings block: 30-day retention, no
// telemetry, system theme, no cloud.
func DefaultConfig() Config {
	return Config{RetentionDays: 30, Telemetry: "off", Theme: "system", CloudSync: false}
}

// configPath returns the config file location under ~/.agentguard/config.
func configPath() (string, error) {
	p, err := Paths()
	if err != nil {
		return "", err
	}
	return filepath.Join(p.Config, "config.json"), nil
}

// LoadConfig reads the config file, returning DefaultConfig() if it doesn't
// exist yet (so the tool works before `init` writes one).
func LoadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path) //nolint:gosec // path is the fixed config location
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return Config{}, err
	}
	cfg := DefaultConfig()
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, fmt.Errorf("config: parse %s: %w", path, err)
	}
	return cfg, nil
}

// SaveConfig atomically writes cfg to path (write-temp-then-rename).
func SaveConfig(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// asMap renders the config as a sorted key→string map for `config list` and
// `config get`. Centralising the key names here keeps get/set/list in sync.
func (c Config) asMap() map[string]string {
	return map[string]string{
		"retention.days": strconv.Itoa(c.RetentionDays),
		"telemetry":      c.Telemetry,
		"theme":          c.Theme,
		"cloud.sync":     strconv.FormatBool(c.CloudSync),
	}
}

// set applies a key=value, validating the value. Returns an error for unknown
// keys or invalid values so a typo never silently no-ops.
func (c *Config) set(key, value string) error {
	switch key {
	case "retention.days":
		n, err := strconv.Atoi(value)
		if err != nil || n < 0 {
			return fmt.Errorf("config: retention.days must be a non-negative integer, got %q", value)
		}
		c.RetentionDays = n
	case "telemetry":
		if value != "off" && value != "anonymous" {
			return fmt.Errorf("config: telemetry must be 'off' or 'anonymous', got %q", value)
		}
		c.Telemetry = value
	case "theme":
		if value != "system" && value != "dark" && value != "light" {
			return fmt.Errorf("config: theme must be system|dark|light, got %q", value)
		}
		c.Theme = value
	case "cloud.sync":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("config: cloud.sync must be true|false, got %q", value)
		}
		c.CloudSync = b
	default:
		return fmt.Errorf("config: unknown key %q (valid: retention.days, telemetry, theme, cloud.sync)", key)
	}
	return nil
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Get and set AgentGuard settings (retention, telemetry, theme, cloud sync)",
	}
	cmd.AddCommand(newConfigListCmd(), newConfigGetCmd(), newConfigSetCmd())
	return cmd
}

func newConfigListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Print all settings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := configPath()
			if err != nil {
				return err
			}
			cfg, err := LoadConfig(path)
			if err != nil {
				return err
			}
			m := cfg.asMap()
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Fprintf(cmd.OutOrStdout(), "%-16s %s\n", k, m[k])
			}
			return nil
		},
	}
}

func newConfigGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Print one setting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := configPath()
			if err != nil {
				return err
			}
			cfg, err := LoadConfig(path)
			if err != nil {
				return err
			}
			v, ok := cfg.asMap()[args[0]]
			if !ok {
				return fmt.Errorf("config: unknown key %q", args[0])
			}
			fmt.Fprintln(cmd.OutOrStdout(), v)
			return nil
		},
	}
}

func newConfigSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Change one setting",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := configPath()
			if err != nil {
				return err
			}
			cfg, err := LoadConfig(path)
			if err != nil {
				return err
			}
			if err := cfg.set(args[0], args[1]); err != nil {
				return err
			}
			if err := SaveConfig(path, cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "set %s = %s\n", args[0], args[1])
			return nil
		},
	}
}
