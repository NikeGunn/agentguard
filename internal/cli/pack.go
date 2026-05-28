package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/agentguard/agentguard/internal/policy"
)

func newPackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pack",
		Short: "Manage rule packs (built-in and user)",
	}
	cmd.AddCommand(newPackListCmd())
	cmd.AddCommand(newPackShowCmd())
	cmd.AddCommand(newPackVerifyCmd())
	return cmd
}

func newPackListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List builtin and user rule packs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()
			builtin, err := policy.ListBuiltin()
			if err != nil {
				return err
			}
			fmt.Fprintln(out, "Built-in packs:")
			for _, name := range builtin {
				_, rules, err := policy.LoadBuiltin(name)
				count := "?"
				if err == nil {
					count = fmt.Sprintf("%d rules", len(rules))
				}
				fmt.Fprintf(out, "  builtin/%-12s  %s\n", name, count)
			}

			user, _ := listUserPacks()
			if len(user) > 0 {
				fmt.Fprintln(out, "\nUser packs (~/.agentguard/packs):")
				for _, p := range user {
					fmt.Fprintf(out, "  user/%s\n", p)
				}
			}
			return nil
		},
	}
}

func newPackShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Print a pack's parsed rules",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			data, err := readPackBytes(name)
			if err != nil {
				return err
			}
			pack, rules, err := policy.CompilePack(data)
			if err != nil {
				return fmt.Errorf("compile: %w", err)
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Pack: %s  (version=%d, %d rules)\n", pack.Name, pack.Version, len(rules))
			for i, r := range rules {
				fmt.Fprintf(out, "  %2d. %-30s  action=%s\n", i+1, r.ID, r.Action)
			}
			return nil
		},
	}
}

func newPackVerifyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "verify <name>",
		Short: "Compile a pack and report any errors",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			data, err := readPackBytes(name)
			if err != nil {
				return err
			}
			pack, rules, err := policy.CompilePack(data)
			if err != nil {
				return fmt.Errorf("verify: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "OK  %s v%d  %d rules\n", pack.Name, pack.Version, len(rules))
			return nil
		},
	}
}

// readPackBytes resolves a name like "default", "builtin/default", or
// "user/foo" (looked up in ~/.agentguard/packs).
func readPackBytes(name string) ([]byte, error) {
	switch {
	case strings.HasPrefix(name, "user/"):
		p := strings.TrimPrefix(name, "user/")
		paths, err := Paths()
		if err != nil {
			return nil, err
		}
		return os.ReadFile(filepath.Join(paths.Packs, p+".yaml"))
	case strings.HasPrefix(name, "builtin/"):
		n := strings.TrimPrefix(name, "builtin/")
		_, rules, err := policy.LoadBuiltin(n)
		if err != nil {
			return nil, err
		}
		_ = rules
		// We need raw bytes for re-compile; round-trip via the loader's
		// internal reader.
		return policy.LoadBuiltinBytes(n)
	default:
		// Try builtin first, then user.
		if data, err := policy.LoadBuiltinBytes(name); err == nil {
			return data, nil
		}
		paths, err := Paths()
		if err != nil {
			return nil, err
		}
		return os.ReadFile(filepath.Join(paths.Packs, name+".yaml"))
	}
}

func listUserPacks() ([]string, error) {
	paths, err := Paths()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(paths.Packs)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".yaml") {
			out = append(out, strings.TrimSuffix(name, ".yaml"))
		}
	}
	sort.Strings(out)
	return out, nil
}
