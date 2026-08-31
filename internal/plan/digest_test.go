package plan

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestApprovalDigestBindsPlanningContent(t *testing.T) {
	base := parseTestPlan(t, validCanonicalFiles())
	want := ApprovalDigest(base)

	edits := []struct {
		name string
		edit func(*Plan)
	}{
		{"specification", func(p *Plan) {
			p.Specification.Raw = bytes.Replace(p.Specification.Raw, []byte("Build it."), []byte("Build it safely."), 1)
		}},
		{"implementation", func(p *Plan) {
			p.Implementation.Raw = bytes.Replace(p.Implementation.Raw, []byte("Simple."), []byte("Simple and safe."), 1)
		}},
		{"task id", func(p *Plan) { p.Tasks[0].ID = "T-009" }},
		{"task title", func(p *Plan) { p.Tasks[0].Title = "Changed title" }},
		{"coverage", func(p *Plan) { p.Tasks[0].Covers = []string{"AC-002"} }},
		{"goal", func(p *Plan) { changeSection(&p.Tasks[0], "Goal", "Changed goal.") }},
		{"context", func(p *Plan) { changeSection(&p.Tasks[0], "Context", "Changed context.") }},
		{"deliverable text", func(p *Plan) { p.Tasks[0].Deliverables[0].Text = "Changed deliverable." }},
		{"criterion text", func(p *Plan) { p.Tasks[0].AcceptanceCriteria[0].Text = "Changed criterion." }},
		{"verification", func(p *Plan) { changeSection(&p.Tasks[0], "Verification", "Manual review.") }},
		{"out of scope", func(p *Plan) { changeSection(&p.Tasks[0], "Out of scope", "Another feature.") }},
		{"task order", func(p *Plan) { p.Tasks[0], p.Tasks[1] = p.Tasks[1], p.Tasks[0] }},
	}
	for _, tt := range edits {
		t.Run(tt.name, func(t *testing.T) {
			candidate := clonePlan(base)
			tt.edit(&candidate)
			if got := ApprovalDigest(candidate); got == want {
				t.Fatalf("planning edit did not change digest %s", got)
			}
		})
	}
}

func TestApprovalDigestExcludesExecutionOnlyContent(t *testing.T) {
	base := parseTestPlan(t, validCanonicalFiles())
	want := ApprovalDigest(base)
	edits := []struct {
		name string
		edit func(*Plan)
	}{
		{"approval field", func(p *Plan) { p.Metadata.ApprovalDigest = nil }},
		{"state", func(p *Plan) { p.Tasks[0].State = TaskInProgress }},
		{"deliverable checkbox", func(p *Plan) { p.Tasks[0].Deliverables[0].Checked = true }},
		{"criterion checkbox", func(p *Plan) { p.Tasks[0].AcceptanceCriteria[0].Checked = true }},
		{"evidence", func(p *Plan) { changeSection(&p.Tasks[0], "Evidence", "Different evidence.") }},
	}
	for _, tt := range edits {
		t.Run(tt.name, func(t *testing.T) {
			candidate := clonePlan(base)
			candidate.Metadata.ApprovalDigest = &want
			tt.edit(&candidate)
			if got := ApprovalDigest(candidate); got != want {
				t.Fatalf("execution edit changed digest\nwant %s\n got %s", want, got)
			}
			if findings := Validate(candidate); len(findings) != 0 {
				t.Fatalf("execution edit invalidated plan structure: %#v", findings)
			}
		})
	}
}

func TestApprovalDigestFrozenVector(t *testing.T) {
	plan := parseTestPlan(t, validCanonicalFiles())
	want, err := os.ReadFile(filepath.Join("testdata", "approval_digest.golden"))
	if err != nil {
		t.Fatal(err)
	}
	got := []byte(ApprovalDigest(plan) + "\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("digest compatibility vector changed\nwant %s got %s", want, got)
	}
}

func changeSection(task *Task, heading, body string) {
	for i := range task.Sections {
		if task.Sections[i].Heading == heading {
			task.Sections[i].Body = body
			return
		}
	}
}

func clonePlan(plan Plan) Plan {
	clone := plan
	clone.Metadata = plan.Metadata
	clone.Specification = plan.Specification
	clone.Specification.Raw = bytes.Clone(plan.Specification.Raw)
	clone.Implementation = plan.Implementation
	clone.Implementation.Raw = bytes.Clone(plan.Implementation.Raw)
	clone.Tasks = make([]Task, len(plan.Tasks))
	for i, task := range plan.Tasks {
		clone.Tasks[i] = task
		clone.Tasks[i].Covers = append([]string(nil), task.Covers...)
		clone.Tasks[i].Sections = append([]Section(nil), task.Sections...)
		clone.Tasks[i].Deliverables = append([]ChecklistItem(nil), task.Deliverables...)
		clone.Tasks[i].AcceptanceCriteria = append([]ChecklistItem(nil), task.AcceptanceCriteria...)
		clone.Tasks[i].Raw = bytes.Clone(task.Raw)
	}
	return clone
}
