package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/robertguss/go-plan/internal/plan"
)

func repo(t *testing.T) *Workspace {
	t.Helper()
	d := t.TempDir()
	if out, err := exec.Command("git", "-C", d, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	w, err := Discover(d)
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func commit(t *testing.T, root string, paths ...string) {
	t.Helper()
	args := append([]string{"-C", root, "add", "--"}, paths...)
	if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
		t.Fatalf("git add: %v %s", err, out)
	}
	cmd := exec.Command("git", "-C", root, "-c", "user.email=t@t.test", "-c", "user.name=t", "commit", "-q", "-m", "plan")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v %s", err, out)
	}
}

func authorPlan(t *testing.T, w *Workspace) {
	t.Helper()
	replacements := map[string]map[string]string{
		".go-plan/specification.md":       {"TODO": "Content.", "- AC-001: Content.": "- AC-001: Works."},
		".go-plan/implementation-plan.md": {"TODO": "Content."},
	}
	for path, repls := range replacements {
		full := filepath.Join(w.Root, path)
		b, err := os.ReadFile(full)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		for a, z := range repls {
			s = strings.ReplaceAll(s, a, z)
		}
		if err = os.WriteFile(full, []byte(s), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

func authorTask(t *testing.T, w *Workspace, id string) {
	t.Helper()
	path := filepath.Join(w.Root, ".go-plan/tasks", strings.ToLower(id)+".md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ReplaceAll(string(b), "TODO (populate during execution)", "Not recorded yet.")
	s = strings.ReplaceAll(s, "TODO", "Content.")
	s = strings.ReplaceAll(s, "- [ ] Content.", "- [ ] Complete work.")
	if err = os.WriteFile(path, []byte(s), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestInitializePreservesAgentsAndDiscoversNested(t *testing.T) {
	w := repo(t)
	old := []byte("# User instructions\nKeep me.\n")
	if err := os.WriteFile(filepath.Join(w.Root, "AGENTS.md"), old, 0644); err != nil {
		t.Fatal(err)
	}
	paths, err := w.Initialize("Demo")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 5 {
		t.Fatalf("paths: %#v", paths)
	}
	b, _ := os.ReadFile(filepath.Join(w.Root, "AGENTS.md"))
	if !strings.HasPrefix(string(b), string(old)) || validateAgents(b, true) != nil {
		t.Fatal("user AGENTS bytes not preserved")
	}
	nested := filepath.Join(w.Root, "a", "b")
	os.MkdirAll(nested, 0755)
	got, err := Discover(nested)
	if err != nil || got.Root != w.Root {
		t.Fatalf("discover: %#v %v", got, err)
	}
	if _, err = w.Initialize("Again"); err == nil {
		t.Fatal("reinitialized existing plan")
	}
}

func TestAgentsRoundTripPreservesOutsideBytes(t *testing.T) {
	for _, old := range [][]byte{[]byte("no newline"), []byte("one newline\n"), []byte("two newlines\n\n")} {
		installed, err := installAgents(old)
		if err != nil {
			t.Fatal(err)
		}
		got, err := removeAgents(installed)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(old) {
			t.Errorf("outside bytes changed: want %q got %q", old, got)
		}
	}
}

func TestWorkflowAndDigest(t *testing.T) {
	w := repo(t)
	if _, err := w.Initialize("Demo"); err != nil {
		t.Fatal(err)
	}
	authorPlan(t, w)
	if _, err := w.AddTask("Build", []string{"AC-001"}, "", false); err != nil {
		t.Fatal(err)
	}
	authorTask(t, w, "T-001")
	d, err := w.Approve()
	if err != nil {
		t.Fatal(err)
	}
	if len(d) != 64 {
		t.Fatal("bad digest")
	}
	changed, err := w.Start("T-001")
	if err != nil || !changed {
		t.Fatalf("start %t %v", changed, err)
	}
	if changed, err = w.Start("T-001"); err != nil || changed {
		t.Fatalf("retry %t %v", changed, err)
	}
	path := filepath.Join(w.Root, ".go-plan/tasks/t-001.md")
	b, _ := os.ReadFile(path)
	s := strings.ReplaceAll(string(b), "- [ ]", "- [x]")
	s = strings.ReplaceAll(s, "Not recorded yet.", "go test ./... passed")
	os.WriteFile(path, []byte(s), 0644)
	changed, err = w.Complete("T-001")
	if err != nil || !changed {
		t.Fatalf("complete %t %v", changed, err)
	}
	p, err := w.Load()
	if err != nil {
		t.Fatal(err)
	}
	if p.Tasks[0].Meta.State != "done" || !plan.ApprovalFresh(p) {
		t.Fatal("completion changed approval or did not persist")
	}
}

func TestRevisionDryRunAndSymlinkSafety(t *testing.T) {
	w := repo(t)
	w.Initialize("Demo")
	authorPlan(t, w)
	w.AddTask("One", []string{"AC-001"}, "", false)
	w.AddTask("Two", nil, "", false)
	before, _ := os.ReadFile(filepath.Join(w.Root, ".go-plan/tasks/t-001.md"))
	r, err := w.ReorderTasks([]string{"T-002", "T-001"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if r.Mapping["T-002"] != "T-001" {
		t.Fatalf("mapping: %#v", r.Mapping)
	}
	after, _ := os.ReadFile(filepath.Join(w.Root, ".go-plan/tasks/t-001.md"))
	if string(before) != string(after) {
		t.Fatal("dry run mutated files")
	}
	outside := filepath.Join(t.TempDir(), "outside")
	os.WriteFile(outside, []byte("x"), 0644)
	os.Remove(filepath.Join(w.Root, ".go-plan/plan.yaml"))
	if err = os.Symlink(outside, filepath.Join(w.Root, ".go-plan/plan.yaml")); err != nil {
		t.Fatal(err)
	}
	if _, err = w.Load(); err == nil {
		t.Fatal("accepted managed symlink")
	}
}

func TestAddAfterAndRemoveTaskRefs(t *testing.T) {
	w := repo(t)
	if _, err := w.Initialize("Demo"); err != nil {
		t.Fatal(err)
	}
	authorPlan(t, w)
	if _, err := w.AddTask("One", []string{"AC-001"}, "", false); err != nil {
		t.Fatal(err)
	}
	authorTask(t, w, "T-001")
	if _, err := w.AddTask("Two", nil, "T-001", false); err != nil {
		t.Fatal(err)
	}
	authorTask(t, w, "T-002")
	p, err := w.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tasks) != 2 || p.Tasks[1].Meta.Title != "Two" {
		t.Fatalf("insert: %#v", p.Tasks)
	}
	impl := filepath.Join(w.Root, ".go-plan/implementation-plan.md")
	b, err := os.ReadFile(impl)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(impl, []byte(strings.Replace(string(b), "Content.", "See T-002.", 1)), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = w.RemoveTask("T-002", false); err == nil {
		t.Fatal("removed referenced task")
	}
	if err = os.WriteFile(impl, b, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err = w.RemoveTask("T-002", false); err != nil {
		t.Fatal(err)
	}
	p, err = w.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Tasks) != 1 || p.Tasks[0].Meta.ID != "T-001" {
		t.Fatalf("after remove: %#v", p.Tasks)
	}
}

func TestForceRemovalPreservesUserContent(t *testing.T) {
	w := repo(t)
	os.WriteFile(filepath.Join(w.Root, "AGENTS.md"), []byte("User text.\n"), 0644)
	w.Initialize("Demo")
	if err := w.Remove(true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(w.Root, ".go-plan")); !os.IsNotExist(err) {
		t.Fatal("plan remains")
	}
	b, err := os.ReadFile(filepath.Join(w.Root, "AGENTS.md"))
	if err != nil || string(b) != "User text.\n" {
		t.Fatalf("AGENTS: %q %v", b, err)
	}
}

func TestMissingAgentsMakesStatusDraft(t *testing.T) {
	w := repo(t)
	if _, err := w.Initialize("Demo"); err != nil {
		t.Fatal(err)
	}
	authorPlan(t, w)
	if _, err := w.AddTask("Build", []string{"AC-001"}, "", false); err != nil {
		t.Fatal(err)
	}
	authorTask(t, w, "T-001")
	if _, err := w.Approve(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.Root, "AGENTS.md"), []byte("user only\n"), 0644); err != nil {
		t.Fatal(err)
	}
	p, err := w.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := w.Status(p).State; got != "draft" {
		t.Fatalf("status %s", got)
	}
	if r := w.Ready(p); r.Reason != "plan_invalid" {
		t.Fatalf("ready %#v", r)
	}
	if f := w.Check(p); len(f) == 0 {
		t.Fatal("check passed without agents")
	}
}

func TestGitPlanCleanComparesSets(t *testing.T) {
	if samePathSet(map[string]bool{"a": true, "extra": true}, map[string]bool{"a": true, "missing": true}) {
		t.Fatal("equal-size mismatched sets compared equal")
	}
	w := repo(t)
	if _, err := w.Initialize("Demo"); err != nil {
		t.Fatal(err)
	}
	authorPlan(t, w)
	if _, err := w.AddTask("Build", []string{"AC-001"}, "", false); err != nil {
		t.Fatal(err)
	}
	authorTask(t, w, "T-001")
	if err := w.gitPlanClean(); err == nil {
		t.Fatal("accepted untracked plan")
	}
	commit(t, w.Root, ".go-plan")
	if err := w.gitPlanClean(); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultRemovalRequiresCompletedTrackedPlan(t *testing.T) {
	w := repo(t)
	if _, err := w.Initialize("Demo"); err != nil {
		t.Fatal(err)
	}
	if err := w.Remove(false); err == nil {
		t.Fatal("removed incomplete plan")
	}
	authorPlan(t, w)
	if _, err := w.AddTask("Build", []string{"AC-001"}, "", false); err != nil {
		t.Fatal(err)
	}
	authorTask(t, w, "T-001")
	if _, err := w.Approve(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Start("T-001"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(w.Root, ".go-plan/tasks/t-001.md")
	b, _ := os.ReadFile(path)
	s := strings.ReplaceAll(string(b), "- [ ]", "- [x]")
	s = strings.ReplaceAll(s, "Not recorded yet.", "go test ./... passed")
	os.WriteFile(path, []byte(s), 0644)
	if _, err := w.Complete("T-001"); err != nil {
		t.Fatal(err)
	}
	if err := w.Remove(false); err == nil {
		t.Fatal("removed untracked completed plan")
	}
	commit(t, w.Root, ".go-plan")
	if err := w.Remove(false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(w.Root, ".go-plan")); !os.IsNotExist(err) {
		t.Fatal("plan remains")
	}
}

func TestTransactionRollback(t *testing.T) {
	w := repo(t)
	a := filepath.Join(w.Root, "a.txt")
	if err := os.WriteFile(a, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	w.beforePublish = func(i int, _ string) error {
		if i == 1 {
			return os.ErrPermission
		}
		return nil
	}
	if err := w.Publish(map[string][]byte{"a.txt": []byte("new"), "b.txt": []byte("created")}); err == nil {
		t.Fatal("expected injected failure")
	}
	b, err := os.ReadFile(a)
	if err != nil || string(b) != "old" {
		t.Fatalf("first file was not restored: %q %v", b, err)
	}
	if _, err = os.Stat(filepath.Join(w.Root, "b.txt")); !os.IsNotExist(err) {
		t.Fatal("second file was published")
	}
}
