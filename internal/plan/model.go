package plan

import "fmt"

const (
	Schema          = "go-plan/v1"
	StateOpen       = "open"
	StateInProgress = "in_progress"
	StateDone       = "done"
)

type Metadata struct {
	Schema         string  `yaml:"schema"`
	Title          string  `yaml:"title"`
	ApprovalDigest *string `yaml:"approval_digest"`
}

type TaskMeta struct {
	Schema string   `yaml:"schema"`
	ID     string   `yaml:"id"`
	Title  string   `yaml:"title"`
	State  string   `yaml:"state"`
	Covers []string `yaml:"covers"`
}

type Document struct {
	Raw      string
	Sections map[string]string
}

type Task struct {
	Meta TaskMeta
	Document
	Path string
}

type Plan struct {
	Metadata       Metadata
	Specification  Document
	Implementation Document
	Tasks          []Task
}

type Finding struct {
	Path    string `json:"path"`
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationError struct{ Findings []Finding }

func (e *ValidationError) Error() string { return "plan validation failed" }

func TaskID(n int) string   { return fmt.Sprintf("T-%03d", n) }
func TaskPath(n int) string { return fmt.Sprintf(".go-plan/tasks/t-%03d.md", n) }

func (p Plan) Task(id string) (Task, bool) {
	for _, t := range p.Tasks {
		if t.Meta.ID == id {
			return t, true
		}
	}
	return Task{}, false
}

type GraphNode struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	State string `json:"state"`
}

type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

func TaskGraph(p Plan) Graph {
	g := Graph{Nodes: []GraphNode{}, Edges: []GraphEdge{}}
	for i, t := range p.Tasks {
		g.Nodes = append(g.Nodes, GraphNode{t.Meta.ID, t.Meta.Title, t.Meta.State})
		if i > 0 {
			g.Edges = append(g.Edges, GraphEdge{From: p.Tasks[i-1].Meta.ID, To: t.Meta.ID})
		}
	}
	return g
}

var SpecificationHeadings = []string{"Objective", "Context", "Users and workflows", "Goals", "Non-goals", "Assumptions", "Requirements", "Constraints", "Acceptance criteria", "Open questions"}
var ImplementationHeadings = []string{"Approach", "Architecture", "Technology and dependencies", "Interfaces and data flow", "Change surface", "Verification strategy", "Decisions and tradeoffs", "Risks and recovery", "Out of scope"}
var TaskHeadings = []string{"Goal", "Context", "Deliverables", "Acceptance criteria", "Verification", "Evidence", "Out of scope"}

// ApprovalTaskHeadings are the task sections that validation requires to be
// populated and that participate in the approval digest. Evidence is excluded
// so execution notes can be recorded without invalidating approval.
var ApprovalTaskHeadings = []string{"Goal", "Context", "Deliverables", "Acceptance criteria", "Verification", "Out of scope"}
