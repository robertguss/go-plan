package plan

import (
	"strings"
	"testing"
)

func completeDoc(headings []string) Document {
	var b strings.Builder
	b.WriteString("# Title\n")
	for _, h := range headings {
		b.WriteString("\n## " + h + "\n\nContent.\n")
	}
	d, err := ParseDocument([]byte(b.String()), headings)
	if err != nil {
		panic(err)
	}
	return d
}

func validPlan() Plan {
	s := completeDoc(SpecificationHeadings)
	s.Sections["Acceptance criteria"] = "- AC-001: Works."
	s.Sections["Open questions"] = "None."
	s.Raw = strings.Replace(s.Raw, "## Acceptance criteria\n\nContent.", "## Acceptance criteria\n\n- AC-001: Works.", 1)
	s.Raw = strings.Replace(s.Raw, "## Open questions\n\nContent.", "## Open questions\n\nNone.", 1)
	task := NewTask("T-001", "Build", []string{"AC-001"})
	for _, h := range TaskHeadings {
		task.Sections[h] = "Content."
	}
	task.Sections["Deliverables"] = "- [ ] Deliver it."
	task.Sections["Acceptance criteria"] = "- [ ] It works."
	task.Sections["Evidence"] = "Not recorded yet."
	task.Path = TaskPath(1)
	return Plan{Metadata: Metadata{Schema: Schema, Title: "Demo"}, Specification: s, Implementation: completeDoc(ImplementationHeadings), Tasks: []Task{task}}
}

func TestStrictYAML(t *testing.T) {
	good := []byte("schema: \"go-plan/v1\"\ntitle: \"Demo\"\napproval_digest: null\n")
	if _, err := ParseMetadata(good); err != nil {
		t.Fatal(err)
	}
	cases := []string{
		"schema: go-plan/v1\ntitle: a\ntitle: b\napproval_digest: null\n",
		"schema: go-plan/v1\ntitle: a\nextra: x\napproval_digest: null\n",
		"schema: &s go-plan/v1\ntitle: a\napproval_digest: null\n",
		"schema: go-plan/v1\ntitle: [a]\napproval_digest: null\n",
		"schema: go-plan/v1\ntitle: a\napproval_digest: null\n---\ntitle: b\n",
	}
	for _, in := range cases {
		if _, err := ParseMetadata([]byte(in)); err == nil {
			t.Errorf("accepted malformed YAML: %q", in)
		}
	}
}

func TestTaskRenderRoundTrip(t *testing.T) {
	p := validPlan()
	got, err := ParseTask(RenderTask(p.Tasks[0]), TaskPath(1))
	if err != nil {
		t.Fatal(err)
	}
	if got.Meta.ID != "T-001" || got.Sections["Goal"] != "Content." {
		t.Fatalf("unexpected round trip: %#v", got)
	}
	if b := RenderTask(got); len(b) == 0 || b[len(b)-1] != '\n' {
		t.Fatal("render must end in one newline")
	}
}

func TestValidateAndStatus(t *testing.T) {
	p := validPlan()
	if f := Validate(p); len(f) != 0 {
		t.Fatalf("unexpected findings: %#v", f)
	}
	if got := DeriveStatus(p, false).State; got != "review_required" {
		t.Fatalf("state %s", got)
	}
	d := ApprovalDigest(p)
	p.Metadata.ApprovalDigest = &d
	if got := DeriveStatus(p, false).State; got != "approved" {
		t.Fatalf("state %s", got)
	}
	p.Tasks[0].Meta.State = "in_progress"
	if !ApprovalFresh(p) {
		t.Fatal("state changed digest")
	}
	p.Tasks[0].Sections["Deliverables"] = "- [x] Deliver it."
	if !ApprovalFresh(p) {
		t.Fatal("checkbox changed digest")
	}
	p.Tasks[0].Sections["Evidence"] = "go test passed"
	if !ApprovalFresh(p) {
		t.Fatal("evidence changed digest")
	}
	p.Tasks[0].Sections["Goal"] = "Changed."
	if ApprovalFresh(p) {
		t.Fatal("planning change did not invalidate digest")
	}
}

func TestReadyAndLifecycleValidation(t *testing.T) {
	p := validPlan()
	d := ApprovalDigest(p)
	p.Metadata.ApprovalDigest = &d
	if r := Ready(p, false); r.Task == nil || r.Task.ID != "T-001" {
		t.Fatalf("ready: %#v", r)
	}
	t2 := p.Tasks[0]
	t2.Meta.ID = "T-002"
	t2.Path = TaskPath(2)
	p.Tasks = append(p.Tasks, t2)
	p.Tasks[0].Meta.State = "open"
	p.Tasks[1].Meta.State = "in_progress"
	if len(Validate(p)) == 0 {
		t.Fatal("accepted active task after open predecessor")
	}
}

func TestInvalidStatusAndReady(t *testing.T) {
	p := validPlan()
	if got := DeriveStatus(p, true).State; got != "draft" {
		t.Fatalf("state %s", got)
	}
	if r := Ready(p, true); r.Task != nil || r.Reason != "plan_invalid" {
		t.Fatalf("ready: %#v", r)
	}
	g := TaskGraph(p)
	if len(g.Nodes) != 1 || g.Nodes[0].ID != "T-001" || len(g.Edges) != 0 {
		t.Fatalf("graph: %#v", g)
	}
	if _, ok := p.Task("T-001"); !ok {
		t.Fatal("missing T-001")
	}
	if stale, ok := StaleApproval(p); ok || stale.Field != "" {
		t.Fatalf("unexpected stale: %#v", stale)
	}
}

func TestHasPlaceholderShapes(t *testing.T) {
	if !HasPlaceholder("TODO") || !HasPlaceholder("- [ ] TODO") || !HasPlaceholder("- [x] TODO") || !HasPlaceholder("Note: TODO") || !HasPlaceholder("TODO (populate during execution)") {
		t.Fatal("missed placeholder")
	}
	if HasPlaceholder("Content.") || HasPlaceholder("- [ ] Complete work.") {
		t.Fatal("false placeholder")
	}
}

func FuzzParseTask(f *testing.F) {
	f.Add(string(RenderTask(validPlan().Tasks[0])))
	f.Fuzz(func(t *testing.T, s string) { _, _ = ParseTask([]byte(s), TaskPath(1)) })
}
