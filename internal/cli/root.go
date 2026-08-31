package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

const (
	ExitOK      = 0
	ExitRuntime = 1
	ExitUsage   = 2
)

type Detail struct {
	Path    string `json:"path"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

type commandError struct {
	Command string
	Code    string
	Message string
	Details []Detail
}

func (e *commandError) Error() string { return e.Message }

type successEnvelope struct {
	Schema  string `json:"schema"`
	Command string `json:"command"`
	OK      bool   `json:"ok"`
	Result  any    `json:"result"`
}

type errorBody struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []Detail `json:"details"`
}

type errorEnvelope struct {
	Schema  string    `json:"schema"`
	Command string    `json:"command"`
	OK      bool      `json:"ok"`
	Error   errorBody `json:"error"`
}

func marshalSuccess(command string, result any) []byte {
	b, _ := json.Marshal(successEnvelope{Schema: "go-plan/v1", Command: command, OK: true, Result: result})
	return append(b, '\n')
}

func marshalError(command, code, message string, details []Detail) []byte {
	if details == nil {
		details = []Detail{}
	}
	b, _ := json.Marshal(errorEnvelope{
		Schema:  "go-plan/v1",
		Command: command,
		OK:      false,
		Error:   errorBody{Code: code, Message: message, Details: details},
	})
	return append(b, '\n')
}

// New constructs the complete v1 command tree. Domain behavior is added by
// later implementation slices; every command is present now so help is stable.
func New() *cobra.Command {
	root := &cobra.Command{
		Use:           "gp",
		Short:         "Manage a deterministic Git-native implementation plan",
		SilenceErrors: true,
		SilenceUsage:  true,
		Example: strings.Join([]string{
			`  gp init --title "Add offline planning"`,
			"  gp status",
			"  gp check",
			"  gp approve",
			"  gp ready",
			"  gp graph",
			"  gp remove --dry-run",
			"  gp task --help",
		}, "\n"),
	}
	root.PersistentFlags().String("repo", "", "repository path (defaults to the current Git worktree)")
	root.PersistentFlags().Bool("json", false, "write one stable go-plan/v1 JSON object")

	root.AddCommand(
		stubCommand("init", "Initialize a draft plan", `gp init --title "Add offline planning"`, withStringFlag("title", "plan title", true)),
		stubCommand("status", "Show derived plan status", "gp status"),
		stubCommand("check", "Validate the active plan", "gp check"),
		stubCommand("approve", "Approve current planning content", "gp approve"),
		stubCommand("ready", "Show the one task that may start", "gp ready"),
		stubCommand("graph", "Render the live task graph", "gp graph --json"),
		stubCommand("remove", "Preview or remove the active plan", "gp remove --dry-run",
			withBoolFlag("dry-run", "preview without changing files"),
			withBoolFlag("yes", "confirm removal"),
			withBoolFlag("force", "bypass validity, completion, and Git-cleanliness checks")),
		newTaskCommand(),
	)
	return root
}

type commandOption func(*cobra.Command)

func withStringFlag(name, usage string, required bool) commandOption {
	return func(cmd *cobra.Command) {
		cmd.Flags().String(name, "", usage)
		if required {
			_ = cmd.MarkFlagRequired(name)
		}
	}
}

func withStringSliceFlag(name, usage string) commandOption {
	return func(cmd *cobra.Command) { cmd.Flags().StringSlice(name, nil, usage) }
}

func withBoolFlag(name, usage string) commandOption {
	return func(cmd *cobra.Command) { cmd.Flags().Bool(name, false, usage) }
}

func withArgs(args cobra.PositionalArgs) commandOption {
	return func(cmd *cobra.Command) { cmd.Args = args }
}

func stubCommand(use, short, example string, options ...commandOption) *cobra.Command {
	cmd := &cobra.Command{
		Use:     use,
		Short:   short,
		Example: "  " + example,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return &commandError{
				Command: cmd.CommandPath()[len("gp "):],
				Code:    "not_implemented",
				Message: cmd.CommandPath() + " is not implemented",
				Details: []Detail{},
			}
		},
	}
	for _, option := range options {
		option(cmd)
	}
	return cmd
}

func newTaskCommand() *cobra.Command {
	task := &cobra.Command{
		Use:   "task",
		Short: "Manage the sequential task set",
		Example: strings.Join([]string{
			`  gp task add --title "Implement parser"`,
			"  gp task list",
			"  gp task show T-001",
			"  gp task start T-001",
			"  gp task complete T-001",
			"  gp task reorder --order T-003,T-002",
			"  gp task remove T-003 --dry-run",
		}, "\n"),
	}
	task.AddCommand(
		stubCommand("add", "Add a task to the mutable suffix", `gp task add --title "Implement parser" --cover AC-001`,
			withStringFlag("title", "task title", true),
			withStringSliceFlag("cover", "acceptance criterion covered (repeatable)"),
			withStringFlag("after", "insert after task ID", false)),
		stubCommand("list", "List tasks in numeric order", "gp task list"),
		stubCommand("show ID", "Show a parsed task", "gp task show T-001", withArgs(cobra.ExactArgs(1))),
		stubCommand("start ID", "Start the one ready task", "gp task start T-001", withArgs(cobra.ExactArgs(1))),
		stubCommand("complete ID", "Complete the active task", "gp task complete T-001", withArgs(cobra.ExactArgs(1))),
		stubCommand("reorder", "Reorder the mutable task suffix", "gp task reorder --order T-003,T-002 --dry-run",
			withStringFlag("order", "comma-separated mutable task IDs", true),
			withBoolFlag("dry-run", "preview without changing files")),
		stubCommand("remove ID", "Remove an open task", "gp task remove T-003 --dry-run",
			withArgs(cobra.ExactArgs(1)),
			withBoolFlag("dry-run", "preview without changing files")),
	)
	return task
}

// Execute runs the command tree using caller-owned streams and returns a v1
// process exit code without terminating the process.
func Execute(args []string, stdout, stderr io.Writer) int {
	cmd := New()
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	err := cmd.Execute()
	if err == nil {
		return ExitOK
	}

	var domainErr *commandError
	exitCode := ExitUsage
	code := "invalid_usage"
	command := commandName(args)
	details := []Detail{}
	message := err.Error()
	if errors.As(err, &domainErr) {
		exitCode = ExitRuntime
		code = domainErr.Code
		command = domainErr.Command
		details = domainErr.Details
		message = domainErr.Message
	}

	if slices.Contains(args, "--json") {
		_, _ = stdout.Write(marshalError(command, code, message, details))
	} else {
		_, _ = fmt.Fprintf(stderr, "error: %s\n", message)
	}
	return exitCode
}

func commandName(args []string) string {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "-") {
			return arg
		}
	}
	return "gp"
}
