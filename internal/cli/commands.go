package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/robertguss/go-plan/internal/plan"
	"github.com/robertguss/go-plan/internal/workspace"
	"github.com/spf13/cobra"
)

type initResult struct {
	Title string   `json:"title"`
	Paths []string `json:"paths"`
}

type checkResult struct {
	Valid    bool           `json:"valid"`
	Findings []plan.Finding `json:"findings"`
}

type approveResult struct {
	ApprovalDigest string `json:"approval_digest"`
}

type removeResult struct {
	DryRun  bool     `json:"dry_run,omitempty"`
	Removed bool     `json:"removed,omitempty"`
	Paths   []string `json:"paths"`
}

type taskListResult struct {
	Tasks []plan.TaskSummary `json:"tasks"`
}

type taskShowResult struct {
	Task                plan.TaskSummary `json:"task"`
	DeliverablesChecked bool             `json:"deliverables_checked"`
	AcceptanceChecked   bool             `json:"acceptance_checked"`
	ApprovalFresh       bool             `json:"approval_fresh"`
}

type taskLifecycleResult struct {
	Task    string `json:"task"`
	Changed bool   `json:"changed"`
}

type revisionResult struct {
	DryRun   bool               `json:"dry_run"`
	Revision workspace.Revision `json:"revision"`
}

func initCmd(o *options) *cobra.Command {
	var title string
	c := &cobra.Command{Use: "init", Short: "Initialize a draft plan", Example: "  gp init --title \"Add offline planning\"", Args: exactArgs(0), RunE: func(c *cobra.Command, _ []string) error {
		if title == "" {
			return annotate(c, &usageError{err: fmt.Errorf("--title is required")})
		}
		return o.do(c, func(w *workspace.Workspace) (any, func(io.Writer), error) {
			paths, err := w.Initialize(title)
			if err != nil {
				return nil, nil, err
			}
			return initResult{Title: title, Paths: paths}, func(out io.Writer) {
				fmt.Fprintf(out, "Initialized plan %q\n", title)
				for _, p := range paths {
					fmt.Fprintln(out, p)
				}
			}, nil
		})
	}}
	c.Flags().StringVar(&title, "title", "", "plan title (required)")
	return c
}

func checkCmd(o *options) *cobra.Command {
	return &cobra.Command{Use: "check", Short: "Validate the active plan", Example: "  gp check\n  gp check --json", Args: exactArgs(0), RunE: func(c *cobra.Command, _ []string) error {
		return o.do(c, func(w *workspace.Workspace) (any, func(io.Writer), error) {
			p, err := w.Load()
			if err != nil {
				return nil, nil, err
			}
			f := w.Check(p)
			if len(f) > 0 {
				return nil, nil, &plan.ValidationError{Findings: f}
			}
			return checkResult{Valid: true, Findings: []plan.Finding{}}, func(out io.Writer) { fmt.Fprintln(out, "Plan is valid") }, nil
		})
	}}
}

func statusCmd(o *options) *cobra.Command {
	return &cobra.Command{Use: "status", Short: "Show derived plan status", Example: "  gp status\n  gp status --json", Args: exactArgs(0), RunE: func(c *cobra.Command, _ []string) error {
		return o.do(c, func(w *workspace.Workspace) (any, func(io.Writer), error) {
			p, err := w.Load()
			if err != nil {
				return nil, nil, err
			}
			s := w.Status(p)
			return s, func(out io.Writer) {
				fmt.Fprintf(out, "Status: %s\nTasks: %d total, %d open, %d active, %d done\nApproval current: %t\n", s.State, s.Total, s.Open, s.InProgress, s.Done, s.ApprovalFresh)
				if s.ActiveTask != nil {
					fmt.Fprintln(out, "Active:", *s.ActiveTask)
				}
				if s.NextTask != nil {
					fmt.Fprintln(out, "Next:", *s.NextTask)
				}
			}, nil
		})
	}}
}

func approveCmd(o *options) *cobra.Command {
	return &cobra.Command{Use: "approve", Short: "Approve current planning content", Example: "  gp approve", Args: exactArgs(0), RunE: func(c *cobra.Command, _ []string) error {
		return o.do(c, func(w *workspace.Workspace) (any, func(io.Writer), error) {
			d, err := w.Approve()
			if err != nil {
				return nil, nil, err
			}
			return approveResult{ApprovalDigest: d}, func(out io.Writer) { fmt.Fprintln(out, "Approved plan", d) }, nil
		})
	}}
}

func readyCmd(o *options) *cobra.Command {
	return &cobra.Command{Use: "ready", Short: "Show the single task allowed to start", Example: "  gp ready\n  gp ready --json", Args: exactArgs(0), RunE: func(c *cobra.Command, _ []string) error {
		return o.do(c, func(w *workspace.Workspace) (any, func(io.Writer), error) {
			p, err := w.Load()
			if err != nil {
				return nil, nil, err
			}
			r := w.Ready(p)
			return r, func(out io.Writer) {
				if r.Task == nil {
					fmt.Fprintln(out, "No task ready:", r.Reason)
				} else {
					fmt.Fprintf(out, "%s %s\n", r.Task.ID, r.Task.Title)
				}
			}, nil
		})
	}}
}

func graphCmd(o *options) *cobra.Command {
	return &cobra.Command{Use: "graph", Short: "Render the live task graph", Example: "  gp graph\n  gp graph --json", Args: exactArgs(0), RunE: func(c *cobra.Command, _ []string) error {
		return o.do(c, func(w *workspace.Workspace) (any, func(io.Writer), error) {
			p, err := w.Load()
			if err != nil {
				return nil, nil, err
			}
			g := plan.TaskGraph(p)
			return g, func(out io.Writer) {
				if len(g.Nodes) == 0 {
					fmt.Fprintln(out, "(no tasks)")
					return
				}
				for i, n := range g.Nodes {
					if i > 0 {
						fmt.Fprintln(out, "  |")
					}
					fmt.Fprintf(out, "%s [%s] %s\n", n.ID, n.State, n.Title)
				}
			}, nil
		})
	}}
}

func removeCmd(o *options) *cobra.Command {
	var dry, yes, force bool
	c := &cobra.Command{Use: "remove", Short: "Safely remove a completed plan", Example: "  gp remove --dry-run\n  gp remove --yes\n  gp remove --yes --force", Args: exactArgs(0), RunE: func(c *cobra.Command, _ []string) error {
		if !dry && !yes {
			return annotate(c, &usageError{err: fmt.Errorf("--yes is required for removal")})
		}
		return o.do(c, func(w *workspace.Workspace) (any, func(io.Writer), error) {
			paths, err := w.RemovalPreview()
			if err != nil {
				return nil, nil, err
			}
			if dry {
				return removeResult{DryRun: true, Paths: paths}, func(out io.Writer) {
					for _, p := range paths {
						fmt.Fprintln(out, "remove", p)
					}
				}, nil
			}
			if err = w.Remove(force); err != nil {
				return nil, nil, err
			}
			return removeResult{Removed: true, Paths: paths}, func(out io.Writer) { fmt.Fprintln(out, "Removed active plan") }, nil
		})
	}}
	c.Flags().BoolVar(&dry, "dry-run", false, "preview without changing files")
	c.Flags().BoolVar(&yes, "yes", false, "confirm destructive removal")
	c.Flags().BoolVar(&force, "force", false, "bypass validity, completion, and Git cleanliness checks")
	return c
}

func taskCmd(o *options) *cobra.Command {
	c := &cobra.Command{Use: "task", Short: "Create, inspect, and transition tasks", Example: "  gp task list\n  gp task add --title \"Implement parser\" --cover AC-001"}
	c.AddCommand(taskAddCmd(o), taskListCmd(o), taskShowCmd(o), taskStartCmd(o), taskCompleteCmd(o), taskReorderCmd(o), taskRemoveCmd(o))
	return c
}

func taskAddCmd(o *options) *cobra.Command {
	var title, after string
	var covers []string
	var dry bool
	c := &cobra.Command{Use: "add", Short: "Append or insert a task", Example: "  gp task add --title \"Implement parser\" --cover AC-001\n  gp task add --title \"Add tests\" --after T-001 --dry-run", Args: exactArgs(0), RunE: func(c *cobra.Command, _ []string) error {
		if title == "" {
			return annotate(c, &usageError{err: fmt.Errorf("--title is required")})
		}
		return o.do(c, func(w *workspace.Workspace) (any, func(io.Writer), error) {
			r, err := w.AddTask(title, covers, after, dry)
			if err != nil {
				return nil, nil, err
			}
			return revisionResult{DryRun: dry, Revision: r}, revisionHuman(r, dry), nil
		})
	}}
	c.Flags().StringVar(&title, "title", "", "task title (required)")
	c.Flags().StringArrayVar(&covers, "cover", nil, "acceptance criterion covered (repeatable)")
	c.Flags().StringVar(&after, "after", "", "insert after T-NNN")
	c.Flags().BoolVar(&dry, "dry-run", false, "preview insertion and renumbering")
	return c
}

func taskListCmd(o *options) *cobra.Command {
	return &cobra.Command{Use: "list", Short: "List tasks in execution order", Example: "  gp task list\n  gp task list --json", Args: exactArgs(0), RunE: func(c *cobra.Command, _ []string) error {
		return o.do(c, func(w *workspace.Workspace) (any, func(io.Writer), error) {
			p, err := w.Load()
			if err != nil {
				return nil, nil, err
			}
			items := []plan.TaskSummary{}
			for _, t := range p.Tasks {
				items = append(items, plan.Summary(t))
			}
			return taskListResult{Tasks: items}, func(out io.Writer) {
				for _, t := range items {
					fmt.Fprintf(out, "%s [%s] %s", t.ID, t.State, t.Title)
					if len(t.Covers) > 0 {
						fmt.Fprintf(out, " (%s)", strings.Join(t.Covers, ", "))
					}
					fmt.Fprintln(out)
				}
			}, nil
		})
	}}
}

func taskShowCmd(o *options) *cobra.Command {
	return &cobra.Command{Use: "show T-NNN", Short: "Show one parsed task", Example: "  gp task show T-001\n  gp task show T-001 --json", Args: exactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		return o.do(c, func(w *workspace.Workspace) (any, func(io.Writer), error) {
			p, err := w.Load()
			if err != nil {
				return nil, nil, err
			}
			t, ok := p.Task(args[0])
			if !ok {
				return nil, nil, &appError{Code: "task_not_found", Message: "unknown task " + args[0], Details: []plan.Finding{}}
			}
			result := taskShowResult{Task: plan.Summary(t), DeliverablesChecked: plan.AllChecked(t.Sections["Deliverables"]), AcceptanceChecked: plan.AllChecked(t.Sections["Acceptance criteria"]), ApprovalFresh: plan.ApprovalFresh(p)}
			return result, func(out io.Writer) {
				fmt.Fprintf(out, "%s [%s] %s\nCovers: %s\n", t.Meta.ID, t.Meta.State, t.Meta.Title, strings.Join(t.Meta.Covers, ", "))
			}, nil
		})
	}}
}

func taskStartCmd(o *options) *cobra.Command {
	return &cobra.Command{Use: "start T-NNN", Short: "Start the only ready task", Example: "  gp task start T-001", Args: exactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		return o.do(c, func(w *workspace.Workspace) (any, func(io.Writer), error) {
			changed, err := w.Start(args[0])
			if err != nil {
				return nil, nil, err
			}
			return taskLifecycleResult{Task: args[0], Changed: changed}, func(out io.Writer) {
				if changed {
					fmt.Fprintln(out, "Started", args[0])
				} else {
					fmt.Fprintln(out, args[0], "already started")
				}
			}, nil
		})
	}}
}

func taskCompleteCmd(o *options) *cobra.Command {
	return &cobra.Command{Use: "complete T-NNN", Short: "Complete the active documented task", Example: "  gp task complete T-001", Args: exactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		return o.do(c, func(w *workspace.Workspace) (any, func(io.Writer), error) {
			changed, err := w.Complete(args[0])
			if err != nil {
				return nil, nil, err
			}
			return taskLifecycleResult{Task: args[0], Changed: changed}, func(out io.Writer) {
				if changed {
					fmt.Fprintln(out, "Completed", args[0])
				} else {
					fmt.Fprintln(out, args[0], "already complete")
				}
			}, nil
		})
	}}
}

func revisionHuman(r workspace.Revision, dry bool) func(io.Writer) {
	return func(out io.Writer) {
		if dry {
			fmt.Fprintln(out, "Dry run:")
		} else {
			fmt.Fprintln(out, "Updated task sequence")
		}
		keys := make([]string, 0, len(r.Mapping))
		for k := range r.Mapping {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(out, "%s -> %s\n", k, r.Mapping[k])
		}
		for _, p := range r.ChangedPaths {
			fmt.Fprintln(out, p)
		}
	}
}

func taskReorderCmd(o *options) *cobra.Command {
	var value string
	var dry bool
	c := &cobra.Command{Use: "reorder", Short: "Reorder the mutable open suffix", Example: "  gp task reorder --order T-003,T-002 --dry-run\n  gp task reorder --order T-003,T-002", Args: exactArgs(0), RunE: func(c *cobra.Command, _ []string) error {
		if value == "" {
			return annotate(c, &usageError{err: fmt.Errorf("--order is required")})
		}
		return o.do(c, func(w *workspace.Workspace) (any, func(io.Writer), error) {
			r, err := w.ReorderTasks(strings.Split(value, ","), dry)
			if err != nil {
				return nil, nil, err
			}
			return revisionResult{DryRun: dry, Revision: r}, revisionHuman(r, dry), nil
		})
	}}
	c.Flags().StringVar(&value, "order", "", "comma-separated open task IDs (required)")
	c.Flags().BoolVar(&dry, "dry-run", false, "preview without changing files")
	return c
}

func taskRemoveCmd(o *options) *cobra.Command {
	var dry bool
	c := &cobra.Command{Use: "remove T-NNN", Short: "Remove an open task and renumber", Example: "  gp task remove T-003 --dry-run\n  gp task remove T-003", Args: exactArgs(1), RunE: func(c *cobra.Command, args []string) error {
		return o.do(c, func(w *workspace.Workspace) (any, func(io.Writer), error) {
			r, err := w.RemoveTask(args[0], dry)
			if err != nil {
				return nil, nil, err
			}
			return revisionResult{DryRun: dry, Revision: r}, revisionHuman(r, dry), nil
		})
	}}
	c.Flags().BoolVar(&dry, "dry-run", false, "preview without changing files")
	return c
}
