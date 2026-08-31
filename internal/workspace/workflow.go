package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/robertguss/go-plan/internal/plan"
)

type Revision struct {
	Mapping      map[string]string `json:"mapping"`
	ChangedPaths []string          `json:"changed_paths"`
}

var taskRefRE = regexp.MustCompile(`\bT-[0-9]{3,}\b`)

func (w *Workspace) Approve() (string, error) {
	p, err := w.Load()
	if err != nil {
		return "", err
	}
	f := plan.SortedFindings(w.Findings(p))
	if len(f) > 0 {
		return "", &plan.ValidationError{Findings: f}
	}
	d := plan.ApprovalDigest(p)
	if p.Metadata.ApprovalDigest != nil && *p.Metadata.ApprovalDigest == d {
		return d, nil
	}
	p.Metadata.ApprovalDigest = &d
	return d, w.Publish(map[string][]byte{".go-plan/plan.yaml": plan.RenderMetadata(p.Metadata)})
}

func (w *Workspace) AddTask(title string, covers []string, after string, dry bool) (Revision, error) {
	p, err := w.Load()
	if err != nil {
		return Revision{}, err
	}
	if plan.InvalidTitle(title) {
		return Revision{}, fmt.Errorf("title must be a non-empty single line")
	}
	known := map[string]bool{}
	for _, x := range plan.AcceptanceIDs(p) {
		known[x] = true
	}
	seen := map[string]bool{}
	for _, c := range covers {
		if seen[c] || !known[c] {
			return Revision{}, fmt.Errorf("invalid or duplicate coverage %s", c)
		}
		seen[c] = true
	}
	idx := len(p.Tasks)
	if after != "" {
		idx = -1
		for i, t := range p.Tasks {
			if t.Meta.ID == after {
				idx = i + 1
				break
			}
		}
		if idx < 0 {
			return Revision{}, fmt.Errorf("unknown task %s", after)
		}
		for i := idx; i < len(p.Tasks); i++ {
			if p.Tasks[i].Meta.State != plan.StateOpen {
				return Revision{}, fmt.Errorf("cannot renumber completed or active task %s", p.Tasks[i].Meta.ID)
			}
		}
	}
	p.Tasks = append(p.Tasks, plan.Task{})
	copy(p.Tasks[idx+1:], p.Tasks[idx:])
	p.Tasks[idx] = plan.NewTask("", title, covers)
	return w.publishRevision(p, dry)
}

func (w *Workspace) RemoveTask(id string, dry bool) (Revision, error) {
	p, err := w.Load()
	if err != nil {
		return Revision{}, err
	}
	idx := -1
	for i, t := range p.Tasks {
		if t.Meta.ID == id {
			idx = i
			if t.Meta.State != plan.StateOpen {
				return Revision{}, fmt.Errorf("only open tasks may be removed")
			}
			break
		}
	}
	if idx < 0 {
		return Revision{}, fmt.Errorf("unknown task %s", id)
	}
	if taskRefRE.MatchString(p.Specification.Raw) && containsTaskRef(p.Specification.Raw, id) {
		return Revision{}, fmt.Errorf("remove references to %s from specification.md first", id)
	}
	if containsTaskRef(p.Implementation.Raw, id) {
		return Revision{}, fmt.Errorf("remove references to %s from implementation-plan.md first", id)
	}
	for i, t := range p.Tasks {
		if i != idx && containsTaskRef(t.Raw, id) {
			return Revision{}, fmt.Errorf("remove references to %s from %s first", id, t.Path)
		}
	}
	p.Tasks = append(p.Tasks[:idx], p.Tasks[idx+1:]...)
	return w.publishRevision(p, dry)
}

func containsTaskRef(body, id string) bool {
	for _, ref := range taskRefRE.FindAllString(body, -1) {
		if ref == id {
			return true
		}
	}
	return false
}

func (w *Workspace) ReorderTasks(order []string, dry bool) (Revision, error) {
	p, err := w.Load()
	if err != nil {
		return Revision{}, err
	}
	prefix := 0
	for prefix < len(p.Tasks) && p.Tasks[prefix].Meta.State != plan.StateOpen {
		prefix++
	}
	open := p.Tasks[prefix:]
	if len(order) != len(open) {
		return Revision{}, fmt.Errorf("--order must contain every open mutable-suffix task exactly once")
	}
	byID := map[string]plan.Task{}
	for _, t := range open {
		byID[t.Meta.ID] = t
	}
	seen := map[string]bool{}
	next := append([]plan.Task{}, p.Tasks[:prefix]...)
	for _, id := range order {
		t, ok := byID[id]
		if !ok || seen[id] {
			return Revision{}, fmt.Errorf("invalid or duplicate task in --order: %s", id)
		}
		seen[id] = true
		next = append(next, t)
	}
	p.Tasks = next
	return w.publishRevision(p, dry)
}

func (w *Workspace) publishRevision(p plan.Plan, dry bool) (Revision, error) {
	mapping := map[string]string{}
	oldPaths := map[string]bool{}
	for i := range p.Tasks {
		old := p.Tasks[i].Meta.ID
		newID := plan.TaskID(i + 1)
		if old != "" && old != newID {
			mapping[old] = newID
		}
		if p.Tasks[i].Path != "" {
			oldPaths[p.Tasks[i].Path] = true
		}
		p.Tasks[i].Meta.ID = newID
		p.Tasks[i].Path = plan.TaskPath(i + 1)
	}
	rewrite := func(s string) string {
		return taskRefRE.ReplaceAllStringFunc(s, func(x string) string {
			if n, ok := mapping[x]; ok {
				return n
			}
			return x
		})
	}
	p.Specification.Raw = rewrite(p.Specification.Raw)
	p.Implementation.Raw = rewrite(p.Implementation.Raw)
	files := map[string][]byte{".go-plan/specification.md": []byte(p.Specification.Raw), ".go-plan/implementation-plan.md": []byte(p.Implementation.Raw)}
	changed := []string{".go-plan/specification.md", ".go-plan/implementation-plan.md"}
	for i := range p.Tasks {
		for h, v := range p.Tasks[i].Sections {
			p.Tasks[i].Sections[h] = rewrite(v)
		}
		files[p.Tasks[i].Path] = plan.RenderTask(p.Tasks[i])
		changed = append(changed, p.Tasks[i].Path)
		delete(oldPaths, p.Tasks[i].Path)
	}
	for path := range oldPaths {
		files[path] = nil
		changed = append(changed, path)
	}
	sort.Strings(changed)
	r := Revision{Mapping: mapping, ChangedPaths: changed}
	if dry {
		return r, nil
	}
	return r, w.Publish(files)
}

func (w *Workspace) Start(id string) (bool, error) {
	p, err := w.Load()
	if err != nil {
		return false, err
	}
	for _, t := range p.Tasks {
		if t.Meta.ID == id && t.Meta.State == plan.StateInProgress && plan.ApprovalFresh(p) {
			return false, nil
		}
	}
	r := plan.Ready(p)
	if r.Task == nil {
		return false, fmt.Errorf("task cannot start: %s", r.Reason)
	}
	if r.Task.ID != id {
		return false, fmt.Errorf("only %s is ready", r.Task.ID)
	}
	for i := range p.Tasks {
		if p.Tasks[i].Meta.ID == id {
			if p.Tasks[i].Meta.State == plan.StateInProgress {
				return false, nil
			}
			p.Tasks[i].Meta.State = plan.StateInProgress
			return true, w.Publish(map[string][]byte{p.Tasks[i].Path: plan.RenderTask(p.Tasks[i])})
		}
	}
	return false, fmt.Errorf("unknown task %s", id)
}

var mdLinkRE = regexp.MustCompile(`\[[^\]]*\]\(([^)]+)\)`)

func (w *Workspace) validateLinks(body string) error {
	return w.validateLinksFrom(body, ".go-plan/tasks")
}

func (w *Workspace) validateLinksFrom(body, base string) error {
	for _, m := range mdLinkRE.FindAllStringSubmatch(body, -1) {
		target := strings.Split(m[1], "#")[0]
		if target == "" || strings.HasPrefix(target, "https:") {
			continue
		}
		if strings.Contains(target, "://") || filepath.IsAbs(target) {
			return fmt.Errorf("unsafe local link %q", target)
		}
		p := filepath.Clean(filepath.Join(base, target))
		if _, err := w.safe(p, false); err != nil {
			return fmt.Errorf("invalid local link %q: %w", target, err)
		}
	}
	return nil
}

func (w *Workspace) ValidateLinks(p plan.Plan) []plan.Finding {
	var findings []plan.Finding
	check := func(path, body string) {
		if err := w.validateLinksFrom(body, filepath.Dir(path)); err != nil {
			findings = append(findings, plan.Finding{Path: path, Field: "links", Message: err.Error()})
		}
	}
	check(".go-plan/specification.md", p.Specification.Raw)
	check(".go-plan/implementation-plan.md", p.Implementation.Raw)
	for _, t := range p.Tasks {
		check(t.Path, t.Raw)
	}
	return plan.SortedFindings(findings)
}

func (w *Workspace) Findings(p plan.Plan) []plan.Finding {
	f := append(plan.Validate(p), w.ValidateLinks(p)...)
	if err := w.CheckAgents(); err != nil {
		f = append(f, plan.Finding{Path: "AGENTS.md", Field: "managed_block", Message: err.Error()})
	}
	return f
}

func (w *Workspace) Complete(id string) (bool, error) {
	p, err := w.Load()
	if err != nil {
		return false, err
	}
	if !plan.ApprovalFresh(p) {
		return false, fmt.Errorf("current approval is required")
	}
	for i := range p.Tasks {
		t := &p.Tasks[i]
		if t.Meta.ID != id {
			continue
		}
		if t.Meta.State == plan.StateDone {
			return false, nil
		}
		if t.Meta.State != plan.StateInProgress {
			return false, fmt.Errorf("task %s is not active", id)
		}
		if !plan.AllChecked(t.Sections["Deliverables"]) || !plan.AllChecked(t.Sections["Acceptance criteria"]) {
			return false, fmt.Errorf("all deliverables and acceptance criteria must be checked")
		}
		for _, h := range []string{"Verification", "Evidence"} {
			v := strings.TrimSpace(t.Sections[h])
			if v == "" || plan.HasPlaceholder(v) {
				return false, fmt.Errorf("%s must contain recorded details", h)
			}
		}
		if err := w.validateLinks(t.Raw); err != nil {
			return false, err
		}
		t.Meta.State = plan.StateDone
		return true, w.Publish(map[string][]byte{t.Path: plan.RenderTask(*t)})
	}
	return false, fmt.Errorf("unknown task %s", id)
}

func (w *Workspace) RemovalPreview() ([]string, error) {
	if _, err := w.safe(".go-plan", false); err != nil {
		return nil, err
	}
	agents, err := os.ReadFile(filepath.Join(w.Root, "AGENTS.md"))
	if err != nil {
		return nil, err
	}
	if _, err = removeAgents(agents); err != nil {
		return nil, err
	}
	var paths []string
	err = filepath.WalkDir(filepath.Join(w.Root, ".go-plan"), func(path string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("managed plan contains symlink: %s", path)
		}
		if !d.IsDir() {
			rel, _ := filepath.Rel(w.Root, path)
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(paths)
	paths = append(paths, "AGENTS.md (managed block)")
	return paths, err
}

func (w *Workspace) Remove(force bool) error {
	preview, err := w.RemovalPreview()
	if err != nil {
		return err
	}
	if !force {
		p, err := w.Load()
		if err != nil {
			return err
		}
		if len(w.ValidateLinks(p)) > 0 || plan.DeriveStatus(p).State != "completed" {
			return fmt.Errorf("default removal requires a valid completed plan")
		}
		if err = w.gitPlanClean(preview); err != nil {
			return err
		}
	}
	agentPath := filepath.Join(w.Root, "AGENTS.md")
	old, err := os.ReadFile(agentPath)
	if err != nil {
		return err
	}
	next, err := removeAgents(old)
	if err != nil {
		return err
	}
	return w.withLock(func() error {
		planPath := filepath.Join(w.Root, ".go-plan")
		backup := filepath.Join(w.Root, ".gp-remove-backup")
		if _, e := os.Lstat(backup); e == nil {
			return fmt.Errorf("removal backup path already exists")
		}
		if err := os.Rename(planPath, backup); err != nil {
			return err
		}
		rollback := func() { _ = os.Rename(backup, planPath); _ = os.WriteFile(agentPath, old, 0644) }
		var writeErr error
		if len(next) == 0 {
			writeErr = os.Remove(agentPath)
		} else {
			writeErr = os.WriteFile(agentPath, next, 0644)
		}
		if writeErr != nil {
			rollback()
			return writeErr
		}
		if err := os.RemoveAll(backup); err != nil {
			rollback()
			return err
		}
		return nil
	})
}
