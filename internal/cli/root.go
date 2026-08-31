package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/robertguss/go-plan/internal/plan"
	"github.com/robertguss/go-plan/internal/workspace"
	"github.com/spf13/cobra"
)

type options struct {
	repo     string
	json     bool
	command  string
	out, err io.Writer
}
type appError struct {
	Code, Message string
	Details       []plan.Finding
}

func (e *appError) Error() string { return e.Message }

type usageError struct{ error }
type envelope struct {
	Schema  string     `json:"schema"`
	Command string     `json:"command"`
	OK      bool       `json:"ok"`
	Result  any        `json:"result,omitempty"`
	Error   *errorBody `json:"error,omitempty"`
}
type errorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details []plan.Finding `json:"details"`
}

func commandName(c *cobra.Command) string {
	parts := strings.Fields(c.CommandPath())
	if len(parts) <= 1 {
		return "root"
	}
	return strings.Join(parts[1:], " ")
}
func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
func (o *options) success(c *cobra.Command, result any, human func(io.Writer)) error {
	if o.json {
		return writeJSON(o.out, envelope{Schema: plan.Schema, Command: commandName(c), OK: true, Result: result})
	}
	human(o.out)
	return nil
}
func (o *options) ws() (*workspace.Workspace, error) {
	w, err := workspace.Discover(o.repo)
	if err != nil {
		return nil, &appError{"repository_not_found", err.Error(), []plan.Finding{}}
	}
	return w, nil
}
func domain(err error) *appError {
	var ve *plan.ValidationError
	if errors.As(err, &ve) {
		return &appError{"plan_invalid", "plan validation failed", ve.Findings}
	}
	return &appError{"operation_failed", err.Error(), []plan.Finding{}}
}

func NewRoot(out, errOut io.Writer) *cobra.Command {
	o := &options{out: out, err: errOut}
	root := &cobra.Command{Use: "gp", Short: "Git-native sequential planning", Long: "gp installs and enforces one deterministic, offline implementation plan in a Git repository.", Example: "  gp init --title \"Add offline planning\"\n  gp status\n  gp ready --json", SilenceUsage: true, SilenceErrors: true}
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentPreRun = func(c *cobra.Command, _ []string) { o.command = commandName(c) }
	root.Annotations = map[string]string{}
	root.SetOut(out)
	root.SetErr(errOut)
	root.PersistentFlags().StringVar(&o.repo, "repo", "", "repository path")
	root.PersistentFlags().BoolVar(&o.json, "json", false, "emit stable go-plan/v1 JSON")
	root.SetFlagErrorFunc(func(c *cobra.Command, e error) error { return &usageError{e} })
	root.AddCommand(initCmd(o), checkCmd(o), statusCmd(o), approveCmd(o), readyCmd(o), graphCmd(o), removeCmd(o), taskCmd(o))
	root.PersistentPostRun = func(_ *cobra.Command, _ []string) { root.Annotations["command"] = o.command }
	return root
}

func Execute() int {
	root := NewRoot(os.Stdout, os.Stderr)
	err := root.Execute()
	if err == nil {
		return 0
	}
	oJSON, _ := root.Flags().GetBool("json")
	if !oJSON {
		oJSON, _ = root.PersistentFlags().GetBool("json")
	}
	var ue *usageError
	if errors.As(err, &ue) {
		if oJSON {
			_ = writeJSON(os.Stdout, envelope{Schema: plan.Schema, Command: commandFromArgs(os.Args[1:]), OK: false, Error: &errorBody{"invalid_usage", ue.Error(), []plan.Finding{}}})
		} else {
			fmt.Fprintln(os.Stderr, ue.Error())
		}
		return 2
	}
	ae := domain(err)
	if errors.As(err, &ae) {
	}
	if oJSON {
		command := root.Annotations["command"]
		if command == "" {
			command = commandFromArgs(os.Args[1:])
		}
		_ = writeJSON(os.Stdout, envelope{Schema: plan.Schema, Command: command, OK: false, Error: &errorBody{ae.Code, ae.Message, ae.Details}})
	} else {
		fmt.Fprintln(os.Stderr, "Error:", ae.Message)
		for _, d := range ae.Details {
			fmt.Fprintf(os.Stderr, "  %s [%s]: %s\n", d.Path, d.Field, d.Message)
		}
	}
	return 1
}

func commandFromArgs(args []string) string {
	var parts []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--repo" {
			i++
			continue
		}
		if strings.HasPrefix(args[i], "-") {
			continue
		}
		parts = append(parts, args[i])
		if len(parts) == 2 || parts[0] != "task" {
			break
		}
	}
	if len(parts) == 0 {
		return "root"
	}
	return strings.Join(parts, " ")
}

func exactArgs(n int) cobra.PositionalArgs {
	return func(c *cobra.Command, args []string) error {
		if len(args) != n {
			return &usageError{fmt.Errorf("%s requires %d argument(s)", c.CommandPath(), n)}
		}
		return nil
	}
}
