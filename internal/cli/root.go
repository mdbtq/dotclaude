// Package cli wires the dotclaude commands.
package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/mdbtq/dotclaude/internal/claude"
	"github.com/mdbtq/dotclaude/internal/lint"
)

// errFindings makes `dotclaude lint` exit non-zero without printing an error.
var errFindings = fmt.Errorf("lint found errors")

type pathFlags struct {
	configDir    string
	globalConfig string
	managedDir   string
	memoryDirs   []string
}

func (f *pathFlags) register(cmd *cobra.Command) {
	pf := cmd.PersistentFlags()
	pf.StringVar(&f.configDir, "config-dir", "", "Claude Code config directory (default ~/.claude, or $CLAUDE_CONFIG_DIR)")
	pf.StringVar(&f.globalConfig, "global-config", "", "Claude Code global config file (default ~/.claude.json)")
	pf.StringVar(&f.managedDir, "managed-dir", "", "managed settings directory (default /Library/Application Support/ClaudeCode)")
	pf.StringArrayVar(&f.memoryDirs, "memory-dir", nil, "extra memory directory to include (repeatable; default <config-dir>/memory when it exists)")
}

func (f *pathFlags) paths() (claude.Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return claude.Paths{}, err
	}
	configDir := f.configDir
	if configDir == "" {
		configDir = os.Getenv("CLAUDE_CONFIG_DIR")
	}
	p := claude.DefaultPaths(home, "")
	p = claude.DefaultPaths(home, p.Expand(configDir))
	if f.globalConfig != "" {
		p.GlobalConfig = p.Expand(f.globalConfig)
	}
	if f.managedDir != "" {
		p.ManagedDir = p.Expand(f.managedDir)
	}
	if len(f.memoryDirs) > 0 {
		p.MemoryDirs = f.memoryDirs
	}
	return p, nil
}

// Execute runs the command line.
func Execute(version string) error {
	var flags pathFlags
	root := &cobra.Command{
		Use:           "dotclaude",
		Short:         "See and check your Claude Code configuration",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	flags.register(root)
	root.AddCommand(serveCmd(&flags), lintCmd(&flags), showCmd(&flags))
	err := root.Execute()
	if err == errFindings {
		os.Exit(1)
	}
	return err
}

func showCmd(flags *pathFlags) *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Print the resolved configuration as JSON",
		Long: "Print the resolved configuration as JSON: settings with their origin, " +
			"instruction files, agents, skills, commands, plugins, hooks, MCP servers and memory.\n" +
			"Secrets are left out: MCP arguments, environment and headers are never read, and " +
			"sensitive settings values are redacted.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := flags.paths()
			if err != nil {
				return err
			}
			c, err := claude.Load(p, project)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), c)
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "resolve for this project directory")
	return cmd
}

func lintCmd(flags *pathFlags) *cobra.Command {
	var (
		project string
		asJSON  bool
		minimum string
	)
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Check the configuration for problems",
		Long:  "Check the configuration for problems. Exits 1 when there are errors.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := flags.paths()
			if err != nil {
				return err
			}
			c, err := claude.Load(p, project)
			if err != nil {
				return err
			}
			findings := filterSeverity(lint.Run(c, lint.DefaultOptions), lint.Severity(minimum))
			if asJSON {
				if err := writeJSON(cmd.OutOrStdout(), findings); err != nil {
					return err
				}
			} else {
				printFindings(cmd.OutOrStdout(), findings)
			}
			for _, f := range findings {
				if f.Severity == lint.Error {
					return errFindings
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "lint the configuration as it applies in this project directory")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print findings as JSON")
	cmd.Flags().StringVar(&minimum, "min", "warning", "lowest severity to report: error, warning or info")
	return cmd
}

func filterSeverity(findings []lint.Finding, minimum lint.Severity) []lint.Finding {
	rank := map[lint.Severity]int{lint.Error: 0, lint.Warning: 1, lint.Info: 2}
	limit, ok := rank[minimum]
	if !ok {
		limit = rank[lint.Warning]
	}
	var out []lint.Finding
	for _, f := range findings {
		if rank[f.Severity] <= limit {
			out = append(out, f)
		}
	}
	return out
}

func printFindings(w io.Writer, findings []lint.Finding) {
	if len(findings) == 0 {
		fmt.Fprintln(w, "no findings")
		return
	}
	counts := map[lint.Severity]int{}
	for _, f := range findings {
		counts[f.Severity]++
		loc := f.Path
		if f.Line > 0 {
			loc = fmt.Sprintf("%s:%d", loc, f.Line)
		}
		fmt.Fprintf(w, "%-7s %-32s %s\n", f.Severity, f.Rule, f.Message)
		if loc != "" {
			fmt.Fprintf(w, "        %s\n", loc)
		}
	}
	fmt.Fprintf(w, "\n%d errors, %d warnings, %d info\n", counts[lint.Error], counts[lint.Warning], counts[lint.Info])
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
