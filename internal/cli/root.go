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
	repo string
	json bool
	out  io.Writer
}

type appError struct {
	Command string
	Code    string
	Message string
	Details []plan.Finding
}

func (e *appError) Error() string { return e.Message }

type usageError struct {
	Command string
	err     error
}

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

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

func findings(f []plan.Finding) []plan.Finding {
	if f == nil {
		return []plan.Finding{}
	}
	return f
}

func annotate(c *cobra.Command, err error) error {
	if err == nil {
		return nil
	}
	cmd := commandName(c)
	var ue *usageError
	if errors.As(err, &ue) {
		if ue.Command == "" {
			ue.Command = cmd
		}
		return ue
	}
	var ae *appError
	if errors.As(err, &ae) {
		if ae.Command == "" {
			ae.Command = cmd
		}
		ae.Details = findings(ae.Details)
		return ae
	}
	var ve *plan.ValidationError
	if errors.As(err, &ve) {
		return &appError{Command: cmd, Code: "plan_invalid", Message: "plan validation failed", Details: findings(ve.Findings)}
	}
	return &appError{Command: cmd, Code: "operation_failed", Message: err.Error(), Details: []plan.Finding{}}
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
		return nil, &appError{Code: "repository_not_found", Message: err.Error(), Details: []plan.Finding{}}
	}
	return w, nil
}

func (o *options) do(c *cobra.Command, fn func(*workspace.Workspace) (any, func(io.Writer), error)) error {
	w, err := o.ws()
	if err != nil {
		return annotate(c, err)
	}
	result, human, err := fn(w)
	if err != nil {
		return annotate(c, err)
	}
	return o.success(c, result, human)
}

func NewRoot(out, errOut io.Writer) *cobra.Command {
	o := &options{out: out}
	root := &cobra.Command{
		Use:           "goplan",
		Short:         "Git-native sequential planning",
		Long:          "goplan installs and enforces one deterministic, offline implementation plan in a Git repository.",
		Example:       "  goplan init --title \"Add offline planning\"\n  goplan status\n  goplan ready --json",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetOut(out)
	root.SetErr(errOut)
	root.PersistentFlags().StringVar(&o.repo, "repo", "", "repository path")
	root.PersistentFlags().BoolVar(&o.json, "json", false, "emit stable go-plan/v1 JSON")
	root.SetFlagErrorFunc(func(c *cobra.Command, e error) error {
		return &usageError{Command: commandName(c), err: e}
	})
	root.AddCommand(initCmd(o), checkCmd(o), statusCmd(o), approveCmd(o), readyCmd(o), graphCmd(o), removeCmd(o), taskCmd(o))
	return root
}

func Execute() int {
	return Run(os.Stdout, os.Stderr, os.Args[1:])
}

func Run(out, errOut io.Writer, args []string) int {
	root := NewRoot(out, errOut)
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		return 0
	}
	jsonOut := oJSON(root, args)
	var ue *usageError
	if errors.As(err, &ue) {
		command := ue.Command
		if command == "" {
			command = "root"
		}
		if jsonOut {
			_ = writeJSON(out, envelope{Schema: plan.Schema, Command: command, OK: false, Error: &errorBody{"invalid_usage", ue.Error(), []plan.Finding{}}})
		} else {
			fmt.Fprintln(errOut, ue.Error())
		}
		return 2
	}
	ae := &appError{Code: "operation_failed", Message: err.Error(), Details: []plan.Finding{}}
	errors.As(err, &ae)
	ae.Details = findings(ae.Details)
	command := ae.Command
	if command == "" {
		command = "root"
	}
	if jsonOut {
		_ = writeJSON(out, envelope{Schema: plan.Schema, Command: command, OK: false, Error: &errorBody{ae.Code, ae.Message, ae.Details}})
	} else {
		fmt.Fprintln(errOut, "Error:", ae.Message)
		for _, d := range ae.Details {
			fmt.Fprintf(errOut, "  %s [%s]: %s\n", d.Path, d.Field, d.Message)
		}
	}
	return 1
}

func oJSON(root *cobra.Command, args []string) bool {
	if v, err := root.PersistentFlags().GetBool("json"); err == nil && v {
		return true
	}
	for _, a := range args {
		if a == "--json" {
			return true
		}
	}
	return false
}

func exactArgs(n int) cobra.PositionalArgs {
	return func(c *cobra.Command, args []string) error {
		if len(args) != n {
			return &usageError{Command: commandName(c), err: fmt.Errorf("%s requires %d argument(s)", c.CommandPath(), n)}
		}
		return nil
	}
}
