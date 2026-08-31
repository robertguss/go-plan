package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/robertguss/go-plan/internal/plan"
	"github.com/robertguss/go-plan/internal/workspace"
	"github.com/spf13/cobra"
)

func initCmd(o *options) *cobra.Command {
	var title string
	c := &cobra.Command{
		Use:     "init",
		Short:   "Initialize a draft plan",
		Example: "  gp init --title \"Add offline planning\"",
		Args:    exactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			if title == "" {
				return &usageError{fmt.Errorf("--title is required")}
			}
			w, e := o.ws()
			if e != nil {
				return e
			}
			paths, e := w.Initialize(title)
			if e != nil {
				return domain(e)
			}
			return o.success(c, map[string]any{"title": title, "paths": paths}, func(out io.Writer) {
				fmt.Fprintf(out, "Initialized plan %q\n", title)
				for _, p := range paths {
					fmt.Fprintln(out, p)
				}
			})
		},
	}
	c.Flags().StringVar(&title, "title", "", "plan title (required)")
	return c
}

func checkCmd(o *options) *cobra.Command {
	return &cobra.Command{
		Use:     "check",
		Short:   "Validate the active plan",
		Example: "  gp check\n  gp check --json",
		Args:    exactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			w, p, e := o.loadPlan()
			if e != nil {
				return e
			}
			f := w.Findings(p)
			if p.Metadata.ApprovalDigest != nil && !plan.ApprovalFresh(p) {
				f = append(f, plan.Finding{Path: ".go-plan/plan.yaml", Field: "approval_digest", Message: "approval is stale"})
			}
			f = plan.SortedFindings(f)
			if len(f) > 0 {
				return &appError{"plan_invalid", "plan validation failed", f}
			}
			return o.success(c, map[string]any{"valid": true, "findings": []plan.Finding{}}, func(out io.Writer) {
				fmt.Fprintln(out, "Plan is valid")
			})
		},
	}
}

func statusCmd(o *options) *cobra.Command {
	return &cobra.Command{
		Use:     "status",
		Short:   "Show derived plan status",
		Example: "  gp status\n  gp status --json",
		Args:    exactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			_, p, e := o.loadPlan()
			if e != nil {
				return e
			}
			s := plan.DeriveStatus(p)
			return o.success(c, s, func(out io.Writer) {
				fmt.Fprintf(out, "Status: %s\nTasks: %d total, %d open, %d active, %d done\nApproval current: %t\n", s.State, s.Total, s.Open, s.InProgress, s.Done, s.ApprovalFresh)
				if s.ActiveTask != nil {
					fmt.Fprintln(out, "Active:", *s.ActiveTask)
				}
				if s.NextTask != nil {
					fmt.Fprintln(out, "Next:", *s.NextTask)
				}
			})
		},
	}
}

func approveCmd(o *options) *cobra.Command {
	return &cobra.Command{
		Use:     "approve",
		Short:   "Approve current planning content",
		Example: "  gp approve",
		Args:    exactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			w, e := o.ws()
			if e != nil {
				return e
			}
			d, e := w.Approve()
			if e != nil {
				return domain(e)
			}
			return o.success(c, map[string]string{"approval_digest": d}, func(out io.Writer) {
				fmt.Fprintln(out, "Approved plan", d)
			})
		},
	}
}

func readyCmd(o *options) *cobra.Command {
	return &cobra.Command{
		Use:     "ready",
		Short:   "Show the single task allowed to start",
		Example: "  gp ready\n  gp ready --json",
		Args:    exactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			_, p, e := o.loadPlan()
			if e != nil {
				return e
			}
			r := plan.Ready(p)
			return o.success(c, r, func(out io.Writer) {
				if r.Task == nil {
					fmt.Fprintln(out, "No task ready:", r.Reason)
				} else {
					fmt.Fprintf(out, "%s %s\n", r.Task.ID, r.Task.Title)
				}
			})
		},
	}
}

type graphNode struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	State string `json:"state"`
}
type graphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func graphCmd(o *options) *cobra.Command {
	return &cobra.Command{
		Use:     "graph",
		Short:   "Render the live task graph",
		Example: "  gp graph\n  gp graph --json",
		Args:    exactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			_, p, e := o.loadPlan()
			if e != nil {
				return e
			}
			nodes := []graphNode{}
			edges := []graphEdge{}
			for i, t := range p.Tasks {
				nodes = append(nodes, graphNode{t.Meta.ID, t.Meta.Title, t.Meta.State})
				if i > 0 {
					edges = append(edges, graphEdge{p.Tasks[i-1].Meta.ID, t.Meta.ID})
				}
			}
			return o.success(c, map[string]any{"nodes": nodes, "edges": edges}, func(out io.Writer) {
				if len(nodes) == 0 {
					fmt.Fprintln(out, "(no tasks)")
					return
				}
				for i, n := range nodes {
					if i > 0 {
						fmt.Fprintln(out, "  |")
					}
					fmt.Fprintf(out, "%s [%s] %s\n", n.ID, n.State, n.Title)
				}
			})
		},
	}
}

func removeCmd(o *options) *cobra.Command {
	var dry, yes, force bool
	c := &cobra.Command{
		Use:     "remove",
		Short:   "Safely remove a completed plan",
		Example: "  gp remove --dry-run\n  gp remove --yes\n  gp remove --yes --force",
		Args:    exactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			w, e := o.ws()
			if e != nil {
				return e
			}
			if !dry && !yes {
				return &usageError{fmt.Errorf("--yes is required for removal")}
			}
			paths, e := w.RemovalPreview()
			if e != nil {
				return domain(e)
			}
			if dry {
				return o.success(c, map[string]any{"dry_run": true, "paths": paths}, func(out io.Writer) {
					for _, p := range paths {
						fmt.Fprintln(out, "remove", p)
					}
				})
			}
			if e = w.Remove(force); e != nil {
				return domain(e)
			}
			return o.success(c, map[string]any{"removed": true, "paths": paths}, func(out io.Writer) {
				fmt.Fprintln(out, "Removed active plan")
			})
		},
	}
	c.Flags().BoolVar(&dry, "dry-run", false, "preview without changing files")
	c.Flags().BoolVar(&yes, "yes", false, "confirm destructive removal")
	c.Flags().BoolVar(&force, "force", false, "bypass validity, completion, and Git cleanliness checks")
	return c
}

func taskCmd(o *options) *cobra.Command {
	c := &cobra.Command{
		Use:     "task",
		Short:   "Create, inspect, and transition tasks",
		Example: "  gp task list\n  gp task add --title \"Implement parser\" --cover AC-001",
	}
	c.AddCommand(taskAddCmd(o), taskListCmd(o), taskShowCmd(o), taskStartCmd(o), taskCompleteCmd(o), taskReorderCmd(o), taskRemoveCmd(o))
	return c
}

func taskAddCmd(o *options) *cobra.Command {
	var title, after string
	var covers []string
	var dry bool
	c := &cobra.Command{
		Use:     "add",
		Short:   "Append or insert a task",
		Example: "  gp task add --title \"Implement parser\" --cover AC-001\n  gp task add --title \"Add tests\" --after T-001 --dry-run",
		Args:    exactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			if title == "" {
				return &usageError{fmt.Errorf("--title is required")}
			}
			w, e := o.ws()
			if e != nil {
				return e
			}
			r, e := w.AddTask(title, covers, after, dry)
			if e != nil {
				return domain(e)
			}
			return revisionOutput(o, c, r, dry)
		},
	}
	c.Flags().StringVar(&title, "title", "", "task title (required)")
	c.Flags().StringArrayVar(&covers, "cover", nil, "acceptance criterion covered (repeatable)")
	c.Flags().StringVar(&after, "after", "", "insert after T-NNN")
	c.Flags().BoolVar(&dry, "dry-run", false, "preview insertion and renumbering")
	return c
}

func taskListCmd(o *options) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List tasks in execution order",
		Example: "  gp task list\n  gp task list --json",
		Args:    exactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			_, p, e := o.loadPlan()
			if e != nil {
				return e
			}
			items := []plan.TaskSummary{}
			for _, t := range p.Tasks {
				items = append(items, plan.Summary(t))
			}
			return o.success(c, map[string]any{"tasks": items}, func(out io.Writer) {
				for _, t := range items {
					fmt.Fprintf(out, "%s [%s] %s", t.ID, t.State, t.Title)
					if len(t.Covers) > 0 {
						fmt.Fprintf(out, " (%s)", strings.Join(t.Covers, ", "))
					}
					fmt.Fprintln(out)
				}
			})
		},
	}
}

func findTask(p plan.Plan, id string) (plan.Task, bool) {
	for _, t := range p.Tasks {
		if t.Meta.ID == id {
			return t, true
		}
	}
	return plan.Task{}, false
}

func taskShowCmd(o *options) *cobra.Command {
	return &cobra.Command{
		Use:     "show T-NNN",
		Short:   "Show one parsed task",
		Example: "  gp task show T-001\n  gp task show T-001 --json",
		Args:    exactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			_, p, e := o.loadPlan()
			if e != nil {
				return e
			}
			t, ok := findTask(p, args[0])
			if !ok {
				return &appError{"task_not_found", "unknown task " + args[0], []plan.Finding{}}
			}
			result := map[string]any{
				"task":                 plan.Summary(t),
				"deliverables_checked": plan.AllChecked(t.Sections["Deliverables"]),
				"acceptance_checked":   plan.AllChecked(t.Sections["Acceptance criteria"]),
				"approval_fresh":       plan.ApprovalFresh(p),
			}
			return o.success(c, result, func(out io.Writer) {
				fmt.Fprintf(out, "%s [%s] %s\nCovers: %s\n", t.Meta.ID, t.Meta.State, t.Meta.Title, strings.Join(t.Meta.Covers, ", "))
			})
		},
	}
}

func taskStartCmd(o *options) *cobra.Command {
	return &cobra.Command{
		Use:     "start T-NNN",
		Short:   "Start the only ready task",
		Example: "  gp task start T-001",
		Args:    exactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			w, e := o.ws()
			if e != nil {
				return e
			}
			changed, e := w.Start(args[0])
			if e != nil {
				return domain(e)
			}
			return o.success(c, map[string]any{"task": args[0], "changed": changed}, func(out io.Writer) {
				if changed {
					fmt.Fprintln(out, "Started", args[0])
				} else {
					fmt.Fprintln(out, args[0], "already started")
				}
			})
		},
	}
}

func taskCompleteCmd(o *options) *cobra.Command {
	return &cobra.Command{
		Use:     "complete T-NNN",
		Short:   "Complete the active documented task",
		Example: "  gp task complete T-001",
		Args:    exactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			w, e := o.ws()
			if e != nil {
				return e
			}
			changed, e := w.Complete(args[0])
			if e != nil {
				return domain(e)
			}
			return o.success(c, map[string]any{"task": args[0], "changed": changed}, func(out io.Writer) {
				if changed {
					fmt.Fprintln(out, "Completed", args[0])
				} else {
					fmt.Fprintln(out, args[0], "already complete")
				}
			})
		},
	}
}

func revisionOutput(o *options, c *cobra.Command, r workspace.Revision, dry bool) error {
	return o.success(c, map[string]any{"dry_run": dry, "revision": r}, func(out io.Writer) {
		if dry {
			fmt.Fprintln(out, "Dry run:")
		} else {
			fmt.Fprintln(out, "Updated task sequence")
		}
		fmt.Fprintln(out, "Review with gp task list")
	})
}

func taskReorderCmd(o *options) *cobra.Command {
	var value string
	var dry bool
	c := &cobra.Command{
		Use:     "reorder",
		Short:   "Reorder the mutable open suffix",
		Example: "  gp task reorder --order T-003,T-002 --dry-run\n  gp task reorder --order T-003,T-002",
		Args:    exactArgs(0),
		RunE: func(c *cobra.Command, _ []string) error {
			if value == "" {
				return &usageError{fmt.Errorf("--order is required")}
			}
			ids := strings.Split(value, ",")
			w, e := o.ws()
			if e != nil {
				return e
			}
			r, e := w.ReorderTasks(ids, dry)
			if e != nil {
				return domain(e)
			}
			return revisionOutput(o, c, r, dry)
		},
	}
	c.Flags().StringVar(&value, "order", "", "comma-separated open task IDs (required)")
	c.Flags().BoolVar(&dry, "dry-run", false, "preview without changing files")
	return c
}

func taskRemoveCmd(o *options) *cobra.Command {
	var dry bool
	c := &cobra.Command{
		Use:     "remove T-NNN",
		Short:   "Remove an open task and renumber",
		Example: "  gp task remove T-003 --dry-run\n  gp task remove T-003",
		Args:    exactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			w, e := o.ws()
			if e != nil {
				return e
			}
			r, e := w.RemoveTask(args[0], dry)
			if e != nil {
				return domain(e)
			}
			return revisionOutput(o, c, r, dry)
		},
	}
	c.Flags().BoolVar(&dry, "dry-run", false, "preview without changing files")
	return c
}
