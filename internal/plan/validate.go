package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var acLineRE = regexp.MustCompile(`(?m)^- (AC-[0-9]{3,}):\s+(.+)$`)
var acIDRE = regexp.MustCompile(`^AC-[0-9]{3,}$`)
var taskIDRE = regexp.MustCompile(`^T-[0-9]{3,}$`)
var checkboxRE = regexp.MustCompile(`(?m)^- \[([ xX])\]\s+(.+)$`)

func AcceptanceIDs(p Plan) []string {
	ms := acLineRE.FindAllStringSubmatch(p.Specification.Sections["Acceptance criteria"], -1)
	r := make([]string, 0, len(ms))
	for _, m := range ms {
		r = append(r, m[1])
	}
	return r
}

func Validate(p Plan) []Finding {
	var f []Finding
	add := func(path, field, message string) { f = append(f, Finding{path, field, message}) }
	if strings.TrimSpace(p.Metadata.Title) == "" || strings.ContainsAny(p.Metadata.Title, "\r\n") {
		add(".go-plan/plan.yaml", "title", "must be a non-empty single line")
	}
	for path, d := range map[string]Document{".go-plan/specification.md": p.Specification, ".go-plan/implementation-plan.md": p.Implementation} {
		for h, body := range d.Sections {
			if strings.TrimSpace(body) == "" || HasPlaceholder(body) {
				add(path, h, "contains a template placeholder")
			}
		}
	}
	if q := strings.TrimSpace(p.Specification.Sections["Open questions"]); q != "None." {
		add(".go-plan/specification.md", "Open questions", `must be exactly "None." before approval`)
	}
	acs := AcceptanceIDs(p)
	known := map[string]bool{}
	for _, id := range acs {
		if known[id] {
			add(".go-plan/specification.md", "Acceptance criteria", "duplicate "+id)
		}
		known[id] = true
	}
	if len(acs) == 0 {
		add(".go-plan/specification.md", "Acceptance criteria", "must contain at least one AC-NNN item")
	}
	for _, line := range strings.Split(p.Specification.Sections["Acceptance criteria"], "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "- AC-") && !acLineRE.MatchString(line) {
			add(".go-plan/specification.md", "Acceptance criteria", "malformed acceptance criterion: "+line)
		}
	}
	covered := map[string]bool{}
	active := 0
	phase := 0 // done prefix, optional active, open suffix
	for i, t := range p.Tasks {
		path := t.Path
		expected := TaskID(i + 1)
		if !taskIDRE.MatchString(t.Meta.ID) || t.Meta.ID != expected {
			add(path, "id", fmt.Sprintf("expected %s", expected))
		}
		if t.Path != TaskPath(i+1) {
			add(path, "filename", "expected "+TaskPath(i+1))
		}
		if t.Meta.State != "open" && t.Meta.State != "in_progress" && t.Meta.State != "done" {
			add(path, "state", "expected open, in_progress, or done")
		}
		if strings.TrimSpace(t.Meta.Title) == "" || strings.ContainsAny(t.Meta.Title, "\r\n") {
			add(path, "title", "must be a non-empty single line")
		}
		switch t.Meta.State {
		case "done":
			if phase != 0 {
				add(path, "state", "done tasks must form a contiguous prefix")
			}
		case "in_progress":
			active++
			if phase != 0 {
				add(path, "state", "active task must immediately follow the completed prefix")
			}
			phase = 1
		case "open":
			phase = 2
		}
		for _, c := range t.Meta.Covers {
			if !acIDRE.MatchString(c) || !known[c] {
				add(path, "covers", "unknown acceptance criterion "+c)
			} else {
				covered[c] = true
			}
		}
		for _, h := range []string{"Goal", "Context", "Deliverables", "Acceptance criteria", "Verification", "Out of scope"} {
			if HasPlaceholder(t.Sections[h]) || strings.TrimSpace(t.Sections[h]) == "" {
				add(path, h, "contains a template placeholder")
			}
		}
		for _, h := range []string{"Deliverables", "Acceptance criteria"} {
			if len(checkboxRE.FindAllStringSubmatch(t.Sections[h], -1)) == 0 {
				add(path, h, "must contain a Markdown checklist")
			}
		}
	}
	if len(p.Tasks) == 0 {
		add(".go-plan/tasks", "tasks", "at least one task is required")
	}
	if active > 1 {
		add(".go-plan/tasks", "state", "at most one task may be in_progress")
	}
	for _, id := range acs {
		if !covered[id] {
			add(".go-plan/specification.md", "Acceptance criteria", id+" is not covered by a task")
		}
	}
	return SortedFindings(f)
}

func ApprovalDigest(p Plan) string {
	h := sha256.New()
	h.Write([]byte("go-plan/v1\x00"))
	h.Write([]byte(p.Specification.Raw))
	h.Write([]byte{0})
	h.Write([]byte(p.Implementation.Raw))
	for _, t := range p.Tasks {
		fmt.Fprintf(h, "\x00%s\x00%s\x00%s", t.Meta.ID, t.Meta.Title, strings.Join(t.Meta.Covers, "\x00"))
		for _, name := range []string{"Goal", "Context", "Deliverables", "Acceptance criteria", "Verification", "Out of scope"} {
			body := checkboxRE.ReplaceAllString(t.Sections[name], "- [ ] $2")
			h.Write([]byte{0})
			h.Write([]byte(name))
			h.Write([]byte{0})
			h.Write([]byte(strings.TrimSpace(body)))
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func ApprovalFresh(p Plan) bool {
	return p.Metadata.ApprovalDigest != nil && *p.Metadata.ApprovalDigest == ApprovalDigest(p)
}

type Status struct {
	State         string  `json:"state"`
	Total         int     `json:"total"`
	Open          int     `json:"open"`
	InProgress    int     `json:"in_progress"`
	Done          int     `json:"done"`
	ActiveTask    *string `json:"active_task"`
	NextTask      *string `json:"next_task"`
	ApprovalFresh bool    `json:"approval_fresh"`
}

func DeriveStatus(p Plan) Status {
	s := Status{Total: len(p.Tasks), ApprovalFresh: ApprovalFresh(p)}
	for _, t := range p.Tasks {
		switch t.Meta.State {
		case "open":
			s.Open++
			if s.NextTask == nil {
				x := t.Meta.ID
				s.NextTask = &x
			}
		case "in_progress":
			s.InProgress++
			x := t.Meta.ID
			s.ActiveTask = &x
		case "done":
			s.Done++
		}
	}
	if len(Validate(p)) > 0 {
		s.State = "draft"
	} else if !s.ApprovalFresh {
		s.State = "review_required"
	} else if s.Done == s.Total {
		s.State = "completed"
	} else if s.Done > 0 || s.InProgress > 0 {
		s.State = "executing"
	} else {
		s.State = "approved"
	}
	return s
}

type ReadyResult struct {
	Task   *TaskSummary `json:"task"`
	Reason string       `json:"reason,omitempty"`
}
type TaskSummary struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	State  string   `json:"state"`
	Covers []string `json:"covers"`
}

func Summary(t Task) TaskSummary {
	c := append([]string(nil), t.Meta.Covers...)
	if c == nil {
		c = []string{}
	}
	return TaskSummary{t.Meta.ID, t.Meta.Title, t.Meta.State, c}
}
func Ready(p Plan) ReadyResult {
	if len(Validate(p)) > 0 {
		return ReadyResult{Reason: "plan_invalid"}
	}
	if !ApprovalFresh(p) {
		return ReadyResult{Reason: "approval_required"}
	}
	for _, t := range p.Tasks {
		if t.Meta.State == "in_progress" {
			return ReadyResult{Reason: "task_active"}
		}
		if t.Meta.State == "open" {
			x := Summary(t)
			return ReadyResult{Task: &x}
		}
	}
	return ReadyResult{Reason: "plan_completed"}
}

func AllChecked(body string) bool {
	ms := checkboxRE.FindAllStringSubmatch(body, -1)
	if len(ms) == 0 {
		return false
	}
	for _, m := range ms {
		if m[1] == " " {
			return false
		}
	}
	return true
}

func HasPlaceholder(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == Placeholder || line == "- [ ] "+Placeholder || line == "- [x] "+Placeholder || line == "- [X] "+Placeholder || strings.HasSuffix(line, ": "+Placeholder) || line == "TODO (populate during execution)" {
			return true
		}
	}
	return false
}
func SortedIDs(m map[string]bool) []string {
	var r []string
	for k := range m {
		r = append(r, k)
	}
	sort.Strings(r)
	return r
}
