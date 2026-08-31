package plan

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const planRoot = ".go-plan/"

var (
	taskReferencePattern       = regexp.MustCompile(`\bT-[0-9]{3,}\b`)
	acceptanceReferencePattern = regexp.MustCompile(`\bAC-[0-9]{3,}\b`)
	placeholderPattern         = regexp.MustCompile(`(?i)\bTODO\b|<!--\s*TODO`)
	digestPattern              = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type CanonicalFiles struct {
	Metadata       []byte
	Specification  []byte
	Implementation []byte
	Tasks          map[string][]byte
}

type Plan struct {
	Metadata       Metadata
	Specification  MarkdownDocument
	Implementation MarkdownDocument
	Tasks          []Task
}

func ParseCanonical(files CanonicalFiles) (Plan, []Finding) {
	var plan Plan
	var findings []Finding
	metadata, err := ParseMetadata(files.Metadata)
	if err != nil {
		findings = append(findings, Finding{Path: planRoot + "plan.yaml", Field: "format", Message: err.Error()})
	} else {
		plan.Metadata = metadata
	}
	specification, err := ParseSpecification(files.Specification)
	if err != nil {
		findings = append(findings, Finding{Path: planRoot + "specification.md", Field: "format", Message: err.Error()})
	} else {
		plan.Specification = specification
	}
	implementation, err := ParseImplementationPlan(files.Implementation)
	if err != nil {
		findings = append(findings, Finding{Path: planRoot + "implementation-plan.md", Field: "format", Message: err.Error()})
	} else {
		plan.Implementation = implementation
	}

	filenames := make([]string, 0, len(files.Tasks))
	for filename := range files.Tasks {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)
	for _, filename := range filenames {
		task, err := ParseTask(filename, files.Tasks[filename])
		path := planRoot + "tasks/" + filename
		if err != nil {
			findings = append(findings, Finding{Path: path, Field: "format", Message: err.Error()})
			continue
		}
		task.Path = path
		plan.Tasks = append(plan.Tasks, task)
	}
	sort.Slice(plan.Tasks, func(i, j int) bool { return taskNumber(plan.Tasks[i].ID) < taskNumber(plan.Tasks[j].ID) })
	SortFindings(findings)
	return plan, findings
}

func Validate(plan Plan) []Finding {
	var findings []Finding
	if plan.Metadata.ApprovalDigest != nil {
		if !digestPattern.MatchString(*plan.Metadata.ApprovalDigest) {
			findings = append(findings, Finding{Path: planRoot + "plan.yaml", Field: "approval_digest", Message: "approval digest must be a lowercase SHA-256 value or null"})
		} else if *plan.Metadata.ApprovalDigest != ApprovalDigest(plan) {
			findings = append(findings, Finding{Path: planRoot + "plan.yaml", Field: "approval_digest", Message: "approval is stale for the current planning content"})
		}
	}
	findings = append(findings, validateDocument(planRoot+"specification.md", plan.Specification)...)
	findings = append(findings, validateDocument(planRoot+"implementation-plan.md", plan.Implementation)...)

	acceptanceIDs := make(map[string]struct{}, len(plan.Specification.AcceptanceIDs))
	for _, id := range plan.Specification.AcceptanceIDs {
		acceptanceIDs[id] = struct{}{}
	}
	if len(acceptanceIDs) == 0 {
		findings = append(findings, Finding{Path: planRoot + "specification.md", Field: "Acceptance criteria", Message: "at least one acceptance criterion is required"})
	}
	if strings.TrimSpace(plan.Specification.sectionBody("Open questions")) != "None." {
		findings = append(findings, Finding{Path: planRoot + "specification.md", Field: "Open questions", Message: "open questions must be resolved with None."})
	}

	if len(plan.Tasks) == 0 {
		findings = append(findings, Finding{Path: planRoot + "tasks", Field: "tasks", Message: "at least one task is required"})
	}
	covered := map[string]struct{}{}
	taskIDs := map[string]struct{}{}
	for _, task := range plan.Tasks {
		taskIDs[task.ID] = struct{}{}
	}
	for i, task := range plan.Tasks {
		path := task.Path
		if path == "" {
			path = planRoot + "tasks/" + strings.ToLower(task.ID) + ".md"
		}
		expected := fmt.Sprintf("T-%03d", i+1)
		if task.ID != expected {
			findings = append(findings, Finding{Path: path, Field: "id", Message: fmt.Sprintf("expected contiguous task ID %s, found %s", expected, task.ID)})
		}
		findings = append(findings, validateTask(path, task)...)
		seenCoverage := map[string]struct{}{}
		for _, id := range task.Covers {
			if _, duplicate := seenCoverage[id]; duplicate {
				findings = append(findings, Finding{Path: path, Field: "covers", Message: fmt.Sprintf("duplicate coverage ID %s", id)})
				continue
			}
			seenCoverage[id] = struct{}{}
			if _, exists := acceptanceIDs[id]; !exists {
				findings = append(findings, Finding{Path: path, Field: "covers", Message: fmt.Sprintf("unknown acceptance criterion %s", id)})
				continue
			}
			covered[id] = struct{}{}
		}
	}
	for _, id := range plan.Specification.AcceptanceIDs {
		if _, exists := covered[id]; !exists {
			findings = append(findings, Finding{Path: planRoot + "specification.md", Field: "covers", Message: fmt.Sprintf("acceptance criterion %s is not covered by a task", id)})
		}
	}
	findings = append(findings, validateLifecycle(plan.Tasks)...)
	findings = append(findings, validateReferences(plan, taskIDs, acceptanceIDs)...)
	SortFindings(findings)
	return findings
}

func validateDocument(path string, document MarkdownDocument) []Finding {
	var findings []Finding
	for _, section := range document.Sections {
		body := strings.TrimSpace(section.Body)
		if body == "" {
			findings = append(findings, Finding{Path: path, Field: section.Heading, Message: "section must not be empty"})
		}
		if placeholderPattern.MatchString(body) {
			findings = append(findings, Finding{Path: path, Field: section.Heading, Message: "section contains a template placeholder"})
		}
	}
	return findings
}

func validateTask(path string, task Task) []Finding {
	var findings []Finding
	for _, section := range task.Sections {
		if section.Heading == "Evidence" {
			continue
		}
		body := strings.TrimSpace(section.Body)
		if body == "" {
			findings = append(findings, Finding{Path: path, Field: section.Heading, Message: "section must not be empty"})
		}
		if placeholderPattern.MatchString(body) {
			findings = append(findings, Finding{Path: path, Field: section.Heading, Message: "section contains a template placeholder"})
		}
	}
	if len(task.Deliverables) == 0 {
		findings = append(findings, Finding{Path: path, Field: "Deliverables", Message: "at least one deliverable is required"})
	}
	if len(task.AcceptanceCriteria) == 0 {
		findings = append(findings, Finding{Path: path, Field: "Acceptance criteria", Message: "at least one task acceptance criterion is required"})
	}
	return findings
}

func validateLifecycle(tasks []Task) []Finding {
	var findings []Finding
	phase := TaskDone
	activeSeen := false
	for _, task := range tasks {
		switch task.State {
		case TaskDone:
			if phase != TaskDone {
				findings = append(findings, Finding{Path: task.Path, Field: "state", Message: "done tasks must form an immutable prefix"})
			}
		case TaskInProgress:
			if activeSeen || phase == TaskOpen {
				findings = append(findings, Finding{Path: task.Path, Field: "state", Message: "at most one task may be in_progress after the done prefix"})
			}
			activeSeen = true
			phase = TaskInProgress
		case TaskOpen:
			phase = TaskOpen
		}
	}
	return findings
}

func validateReferences(plan Plan, taskIDs, acceptanceIDs map[string]struct{}) []Finding {
	var findings []Finding
	type source struct {
		path string
		raw  []byte
	}
	sources := []source{
		{planRoot + "specification.md", plan.Specification.Raw},
		{planRoot + "implementation-plan.md", plan.Implementation.Raw},
	}
	for _, task := range plan.Tasks {
		sources = append(sources, source{task.Path, task.Raw})
	}
	for _, source := range sources {
		for _, id := range uniqueMatches(taskReferencePattern, source.raw) {
			if _, exists := taskIDs[id]; !exists {
				findings = append(findings, Finding{Path: source.path, Field: "references", Message: fmt.Sprintf("unknown task reference %s", id)})
			}
		}
		for _, id := range uniqueMatches(acceptanceReferencePattern, source.raw) {
			if _, exists := acceptanceIDs[id]; !exists {
				findings = append(findings, Finding{Path: source.path, Field: "references", Message: fmt.Sprintf("unknown acceptance reference %s", id)})
			}
		}
	}
	return findings
}

func uniqueMatches(pattern *regexp.Regexp, data []byte) []string {
	seen := map[string]struct{}{}
	var matches []string
	for _, match := range pattern.FindAllString(string(data), -1) {
		if _, exists := seen[match]; exists {
			continue
		}
		seen[match] = struct{}{}
		matches = append(matches, match)
	}
	sort.Strings(matches)
	return matches
}

func taskNumber(id string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(id, "T-"))
	return n
}
