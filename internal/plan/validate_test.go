package plan

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
)

func TestValidateAcceptsCompleteCanonicalPlan(t *testing.T) {
	plan := parseTestPlan(t, validCanonicalFiles())
	if findings := Validate(plan); len(findings) != 0 {
		t.Fatalf("unexpected findings: %#v", findings)
	}
}

func TestValidateFindsWholePlanFailuresDeterministically(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(*CanonicalFiles)
		field string
	}{
		{"placeholder", func(f *CanonicalFiles) {
			f.Specification = bytes.Replace(f.Specification, []byte("Build it."), []byte("<!-- TODO: objective -->"), 1)
		}, "Objective"},
		{"open questions", func(f *CanonicalFiles) {
			f.Specification = bytes.Replace(f.Specification, []byte("## Open questions\n\nNone."), []byte("## Open questions\n\nWhich API?"), 1)
		}, "Open questions"},
		{"missing coverage", func(f *CanonicalFiles) {
			f.Tasks["t-002.md"] = bytes.Replace(f.Tasks["t-002.md"], []byte("covers:\n  - \"AC-002\"\n"), []byte("covers: []\n"), 1)
		}, "covers"},
		{"task gap", func(f *CanonicalFiles) {
			delete(f.Tasks, "t-002.md")
			f.Tasks["t-003.md"] = testTask("T-003", "Second task", TaskOpen, []string{"AC-002"}, false, "Evidence two.")
		}, "id"},
		{"invalid lifecycle", func(f *CanonicalFiles) {
			f.Tasks["t-002.md"] = bytes.Replace(f.Tasks["t-002.md"], []byte("state: \"open\""), []byte("state: \"done\""), 1)
		}, "state"},
		{"unknown task reference", func(f *CanonicalFiles) {
			f.Implementation = bytes.Replace(f.Implementation, []byte("## Out of scope\n\nNone."), []byte("## Out of scope\n\nFollow-up T-999."), 1)
		}, "references"},
		{"malformed approval digest", func(f *CanonicalFiles) {
			f.Metadata = bytes.Replace(f.Metadata, []byte("approval_digest: null"), []byte("approval_digest: \"short\""), 1)
		}, "approval_digest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := validCanonicalFiles()
			tt.edit(&files)
			plan := parseTestPlan(t, files)
			findings := Validate(plan)
			if len(findings) == 0 {
				t.Fatal("expected validation finding")
			}
			found := false
			for _, finding := range findings {
				if finding.Field == tt.field {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing field %q in %#v", tt.field, findings)
			}
			copyOfFindings := append([]Finding(nil), findings...)
			SortFindings(copyOfFindings)
			if !reflect.DeepEqual(findings, copyOfFindings) {
				t.Fatalf("findings are not deterministic: %#v", findings)
			}
		})
	}
}

func TestValidateAggregatesParseFindings(t *testing.T) {
	files := validCanonicalFiles()
	files.Metadata = []byte("schema: bad\n")
	files.Specification = []byte("# broken\n")
	files.Tasks["t-001.md"] = []byte("broken\n")
	_, findings := ParseCanonical(files)
	if len(findings) != 3 {
		t.Fatalf("findings = %#v, want three independent parse failures", findings)
	}
	if findings[0].Path != ".go-plan/plan.yaml" || findings[1].Path != ".go-plan/specification.md" || findings[2].Path != ".go-plan/tasks/t-001.md" {
		t.Fatalf("unexpected finding order: %#v", findings)
	}
}

func parseTestPlan(t *testing.T, files CanonicalFiles) Plan {
	t.Helper()
	plan, findings := ParseCanonical(files)
	if len(findings) != 0 {
		t.Fatalf("parse findings: %#v", findings)
	}
	return plan
}

func validCanonicalFiles() CanonicalFiles {
	specification := []byte("# Specification\n\n## Objective\n\nBuild it.\n\n## Context\n\nContext.\n\n## Users and workflows\n\nUsers.\n\n## Goals\n\nGoals.\n\n## Non-goals\n\nNone.\n\n## Assumptions\n\nNone.\n\n## Requirements\n\nRequirements.\n\n## Constraints\n\nConstraints.\n\n## Acceptance criteria\n\n- AC-001: First works.\n- AC-002: Second works.\n\n## Open questions\n\nNone.\n")
	implementation := []byte("# Implementation plan\n\n## Approach\n\nSimple.\n\n## Architecture\n\nDeep modules.\n\n## Technology and dependencies\n\nGo.\n\n## Interfaces and data flow\n\nTyped calls.\n\n## Change surface\n\nInternal packages.\n\n## Verification strategy\n\nRun tests.\n\n## Decisions and tradeoffs\n\nPrefer determinism.\n\n## Risks and recovery\n\nUse Git.\n\n## Out of scope\n\nNone.\n")
	return CanonicalFiles{
		Metadata:       []byte("schema: \"go-plan/v1\"\ntitle: \"Test plan\"\napproval_digest: null\n"),
		Specification:  specification,
		Implementation: implementation,
		Tasks: map[string][]byte{
			"t-001.md": testTask("T-001", "First task", TaskOpen, []string{"AC-001"}, false, "Evidence one."),
			"t-002.md": testTask("T-002", "Second task", TaskOpen, []string{"AC-002"}, false, "Evidence two."),
		},
	}
}

func testTask(id, title, state string, covers []string, checked bool, evidence string) []byte {
	coverLines := "covers: []\n"
	if len(covers) != 0 {
		coverLines = "covers:\n"
		for _, cover := range covers {
			coverLines += fmt.Sprintf("  - %q\n", cover)
		}
	}
	marker := " "
	if checked {
		marker = "x"
	}
	return []byte(fmt.Sprintf("---\nschema: \"go-plan/v1\"\nid: %q\ntitle: %q\nstate: %q\n%s---\n\n# %s: %s\n\n## Goal\n\nGoal for %s.\n\n## Context\n\nContext for %s.\n\n## Deliverables\n\n- [%s] Deliver %s.\n\n## Acceptance criteria\n\n- [%s] Verify %s.\n\n## Verification\n\n```sh\ngo test ./...\n```\n\n## Evidence\n\n%s\n\n## Out of scope\n\nNone.\n", id, title, state, coverLines, id, title, id, id, marker, id, marker, id, evidence))
}
