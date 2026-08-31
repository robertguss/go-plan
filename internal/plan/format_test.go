package plan

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseMetadataStrict(t *testing.T) {
	valid := []byte("schema: \"go-plan/v1\"\ntitle: \"Example\"\napproval_digest: null\n")
	got, err := ParseMetadata(valid)
	if err != nil {
		t.Fatal(err)
	}
	if got.Schema != SchemaV1 || got.Title != "Example" || got.ApprovalDigest != nil {
		t.Fatalf("unexpected metadata: %#v", got)
	}

	tests := map[string]string{
		"duplicate":          "schema: go-plan/v1\nschema: go-plan/v1\ntitle: x\napproval_digest: null\n",
		"alias":              "schema: &schema go-plan/v1\ntitle: *schema\napproval_digest: null\n",
		"custom tag":         "schema: !custom go-plan/v1\ntitle: x\napproval_digest: null\n",
		"unknown":            "schema: go-plan/v1\ntitle: x\napproval_digest: null\nextra: value\n",
		"wrong scalar type":  "schema: go-plan/v1\ntitle: true\napproval_digest: null\n",
		"wrong digest type":  "schema: go-plan/v1\ntitle: x\napproval_digest: 42\n",
		"multiple documents": "schema: go-plan/v1\ntitle: x\napproval_digest: null\n---\nschema: go-plan/v1\n",
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseMetadata([]byte(input)); err == nil {
				t.Fatal("expected parse failure")
			}
		})
	}
}

func TestParseSpecificationAndImplementationPlan(t *testing.T) {
	spec := []byte("# Specification\n\n## Objective\n\nBuild it.\n\n## Context\n\nContext.\n\n## Users and workflows\n\nUsers.\n\n## Goals\n\nGoals.\n\n## Non-goals\n\nNone.\n\n## Assumptions\n\nNone.\n\n## Requirements\n\nRequirements.\n\n## Constraints\n\nConstraints.\n\n## Acceptance criteria\n\n- AC-001: Works.\n- AC-002: Stays deterministic.\n\n## Open questions\n\nNone.\n")
	got, err := ParseSpecification(spec)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.AcceptanceIDs, []string{"AC-001", "AC-002"}) {
		t.Fatalf("acceptance IDs = %#v", got.AcceptanceIDs)
	}

	badOrder := bytes.Replace(spec, []byte("## Objective"), []byte("## Context"), 1)
	if _, err := ParseSpecification(badOrder); err == nil {
		t.Fatal("expected duplicate/out-of-order heading failure")
	}
	badID := bytes.Replace(spec, []byte("AC-002"), []byte("AC-2"), 1)
	if _, err := ParseSpecification(badID); err == nil {
		t.Fatal("expected malformed acceptance ID failure")
	}

	implementation := []byte("# Implementation plan\n\n## Approach\n\nA.\n\n## Architecture\n\nB.\n\n## Technology and dependencies\n\nC.\n\n## Interfaces and data flow\n\nD.\n\n## Change surface\n\nE.\n\n## Verification strategy\n\nF.\n\n## Decisions and tradeoffs\n\nG.\n\n## Risks and recovery\n\nH.\n\n## Out of scope\n\nI.\n")
	if _, err := ParseImplementationPlan(implementation); err != nil {
		t.Fatal(err)
	}
}

func TestParseTaskFrontmatterAndChecklists(t *testing.T) {
	input := []byte("---\nschema: \"go-plan/v1\"\nid: \"T-001\"\ntitle: \"Create parser\"\nstate: \"open\"\ncovers:\n  - \"AC-001\"\n---\n\n# T-001: Create parser\n\n## Goal\n\nParse.\n\n## Context\n\nContext.\n\n## Deliverables\n\n- [ ] Parser.\n- [x] Tests.\n\n## Acceptance criteria\n\n- [ ] Reject invalid input.\n\n## Verification\n\n```sh\ngo test ./...\n```\n\n## Evidence\n\n<!-- TODO -->\n\n## Out of scope\n\nNone.\n")
	got, err := ParseTask("t-001.md", input)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "T-001" || got.State != TaskOpen || !reflect.DeepEqual(got.Covers, []string{"AC-001"}) {
		t.Fatalf("unexpected task metadata: %#v", got)
	}
	if len(got.Deliverables) != 2 || got.Deliverables[0].Checked || !got.Deliverables[1].Checked {
		t.Fatalf("deliverables = %#v", got.Deliverables)
	}
	if _, err := ParseTask("t-002.md", input); err == nil {
		t.Fatal("expected filename/ID mismatch")
	}
	for _, replacement := range []string{"T-01", "X-001", "T-000"} {
		bad := bytes.Replace(input, []byte("T-001"), []byte(replacement), 1)
		if _, err := ParseTask("t-001.md", bad); err == nil {
			t.Fatalf("expected invalid ID %q to fail", replacement)
		}
	}
}

func TestRenderMetadataDeterministic(t *testing.T) {
	digest := strings.Repeat("a", 64)
	metadata := Metadata{Schema: SchemaV1, Title: "Quoted: \"title\"", ApprovalDigest: &digest}
	first, err := RenderMetadata(metadata)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderMetadata(metadata)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) || !bytes.HasSuffix(first, []byte("\n")) || bytes.HasSuffix(first, []byte("\n\n")) {
		t.Fatalf("non-canonical render: %q", first)
	}
	parsed, err := ParseMetadata(first)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(parsed, metadata) {
		t.Fatalf("round trip = %#v, want %#v", parsed, metadata)
	}
}

func TestTemplatesGoldenAndRoundTrip(t *testing.T) {
	files, err := RenderInitial("Example plan")
	if err != nil {
		t.Fatal(err)
	}
	wantPaths := []string{"plan.yaml", "specification.md", "implementation-plan.md", "tasks/t-001.md"}
	for _, path := range wantPaths {
		got, ok := files[path]
		if !ok {
			t.Fatalf("missing %s", path)
		}
		if !bytes.HasSuffix(got, []byte("\n")) || bytes.HasSuffix(got, []byte("\n\n")) {
			t.Errorf("%s does not have exactly one trailing newline", path)
		}
		golden, err := os.ReadFile(filepath.Join("testdata", filepath.Base(path)+".golden"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, golden) {
			t.Errorf("%s differs from golden\nwant:\n%s\ngot:\n%s", path, golden, got)
		}
	}
	if _, err := ParseMetadata(files["plan.yaml"]); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseSpecification(files["specification.md"]); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseImplementationPlan(files["implementation-plan.md"]); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseTask("t-001.md", files["tasks/t-001.md"]); err != nil {
		t.Fatal(err)
	}
}
