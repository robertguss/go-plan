package workspace

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/robertguss/go-plan/internal/plan"
)

const agentsStart = "<!-- go-plan:managed:start schema=go-plan/v1 -->"
const agentsStartPrefix = "<!-- go-plan:managed:start schema=go-plan/v1"
const agentsEnd = "<!-- go-plan:managed:end -->"

//go:embed templates/agents.md
var AgentsBlock string

type Workspace struct {
	Root          string
	beforePublish func(index int, path string) error
}

func Discover(path string) (*Workspace, error) {
	if path == "" {
		var err error
		path, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	out, err := exec.Command("git", "-C", abs, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return nil, fmt.Errorf("not inside a Git worktree: %s", abs)
	}
	return &Workspace{Root: strings.TrimSpace(string(out))}, nil
}

func (w *Workspace) safe(rel string, allowMissing bool) (string, error) {
	if filepath.IsAbs(rel) || rel == "" {
		return "", fmt.Errorf("unsafe managed path %q", rel)
	}
	clean := filepath.Clean(rel)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("managed path escapes repository: %q", rel)
	}
	p := filepath.Join(w.Root, clean)
	cur := w.Root
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		cur = filepath.Join(cur, part)
		fi, err := os.Lstat(cur)
		if errors.Is(err, os.ErrNotExist) && allowMissing {
			continue
		}
		if err != nil {
			return "", err
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("managed path contains symlink: %s", clean)
		}
	}
	return p, nil
}

func (w *Workspace) PlanExists() bool {
	_, err := os.Lstat(filepath.Join(w.Root, ".go-plan"))
	return err == nil
}

func validateAgents(data []byte, require bool) error {
	s := string(data)
	starts := strings.Count(s, agentsStartPrefix)
	ends := strings.Count(s, agentsEnd)
	if starts != ends || starts > 1 {
		return fmt.Errorf("AGENTS.md has malformed or duplicate go-plan markers")
	}
	if starts == 1 {
		if _, _, _, err := parseAgentsBlock(s); err != nil {
			return err
		}
	}
	if require && starts != 1 {
		return fmt.Errorf("AGENTS.md managed block is missing")
	}
	return nil
}

func renderedAgentsBlock(prefix int) string {
	if prefix == 0 {
		return AgentsBlock
	}
	start := strings.Replace(agentsStart, " -->", fmt.Sprintf(" prefix=%d -->", prefix), 1)
	return strings.Replace(AgentsBlock, agentsStart, start, 1)
}

func parseAgentsBlock(s string) (block string, index, prefix int, err error) {
	i := strings.Index(s, agentsStartPrefix)
	if i < 0 {
		return "", -1, 0, fmt.Errorf("AGENTS.md managed block is missing")
	}
	lineEnd := strings.Index(s[i:], "\n")
	if lineEnd < 0 {
		return "", i, 0, fmt.Errorf("AGENTS.md has malformed go-plan start marker")
	}
	line := s[i : i+lineEnd]
	switch line {
	case agentsStart:
		prefix = 0
	case strings.Replace(agentsStart, " -->", " prefix=1 -->", 1):
		prefix = 1
	case strings.Replace(agentsStart, " -->", " prefix=2 -->", 1):
		prefix = 2
	default:
		return "", i, 0, fmt.Errorf("AGENTS.md has malformed go-plan start marker")
	}
	block = renderedAgentsBlock(prefix)
	if !strings.HasPrefix(s[i:], block) {
		return "", i, prefix, fmt.Errorf("AGENTS.md managed block is modified")
	}
	return block, i, prefix, nil
}

func installAgents(old []byte) ([]byte, error) {
	if err := validateAgents(old, false); err != nil {
		return nil, err
	}
	if strings.Contains(string(old), agentsStartPrefix) {
		return nil, fmt.Errorf("AGENTS.md already contains a managed go-plan block")
	}
	if len(old) == 0 {
		return []byte(AgentsBlock), nil
	}
	prefix := 1
	if old[len(old)-1] != '\n' {
		prefix = 2
	}
	out := append([]byte{}, old...)
	out = append(out, strings.Repeat("\n", prefix)...)
	out = append(out, renderedAgentsBlock(prefix)...)
	return out, nil
}

func removeAgents(old []byte) ([]byte, error) {
	if err := validateAgents(old, true); err != nil {
		return nil, err
	}
	s := string(old)
	block, i, prefix, err := parseAgentsBlock(s)
	if err != nil {
		return nil, err
	}
	start := i - prefix
	if start < 0 || s[start:i] != strings.Repeat("\n", prefix) {
		return nil, fmt.Errorf("AGENTS.md managed separator is modified")
	}
	out := s[:start] + s[i+len(block):]
	return []byte(out), nil
}

func (w *Workspace) Initialize(title string) ([]string, error) {
	if plan.InvalidTitle(title) {
		return nil, fmt.Errorf("title must be a non-empty single line")
	}
	if w.PlanExists() {
		return nil, fmt.Errorf(".go-plan already exists")
	}
	agentPath := filepath.Join(w.Root, "AGENTS.md")
	old, err := os.ReadFile(agentPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	agents, err := installAgents(old)
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{".go-plan/plan.yaml": plan.RenderMetadata(plan.Metadata{Schema: plan.Schema, Title: title}), ".go-plan/specification.md": plan.SpecificationTemplate(title), ".go-plan/implementation-plan.md": plan.ImplementationTemplate(title), "AGENTS.md": agents}
	if err = os.MkdirAll(filepath.Join(w.Root, ".go-plan/tasks"), 0755); err != nil {
		return nil, err
	}
	if err = w.Publish(files); err != nil {
		_ = os.RemoveAll(filepath.Join(w.Root, ".go-plan"))
		return nil, err
	}
	paths := []string{".go-plan/plan.yaml", ".go-plan/specification.md", ".go-plan/implementation-plan.md", ".go-plan/tasks/", "AGENTS.md"}
	return paths, nil
}

func (w *Workspace) Load() (plan.Plan, error) {
	var p plan.Plan
	var findings []plan.Finding
	read := func(rel string) ([]byte, error) {
		path, err := w.safe(rel, false)
		if err != nil {
			return nil, err
		}
		return os.ReadFile(path)
	}
	add := func(path, message string) {
		findings = append(findings, plan.Finding{Path: path, Field: "format", Message: message})
	}
	b, err := read(".go-plan/plan.yaml")
	if err != nil {
		add(".go-plan/plan.yaml", err.Error())
	} else if p.Metadata, err = plan.ParseMetadata(b); err != nil {
		add(".go-plan/plan.yaml", err.Error())
	}
	b, err = read(".go-plan/specification.md")
	if err != nil {
		add(".go-plan/specification.md", err.Error())
	} else if p.Specification, err = plan.ParseDocument(b, plan.SpecificationHeadings); err != nil {
		add(".go-plan/specification.md", err.Error())
	}
	b, err = read(".go-plan/implementation-plan.md")
	if err != nil {
		add(".go-plan/implementation-plan.md", err.Error())
	} else if p.Implementation, err = plan.ParseDocument(b, plan.ImplementationHeadings); err != nil {
		add(".go-plan/implementation-plan.md", err.Error())
	}
	dir, err := w.safe(".go-plan/tasks", false)
	if err != nil {
		add(".go-plan/tasks", err.Error())
		return p, &plan.ValidationError{Findings: plan.SortedFindings(findings)}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		add(".go-plan/tasks", err.Error())
		return p, &plan.ValidationError{Findings: plan.SortedFindings(findings)}
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			add(filepath.ToSlash(filepath.Join(".go-plan/tasks", e.Name())), "unexpected canonical task entry")
			continue
		}
		rel := filepath.ToSlash(filepath.Join(".go-plan/tasks", e.Name()))
		b, err := read(rel)
		if err != nil {
			add(rel, err.Error())
			continue
		}
		t, err := plan.ParseTask(b, rel)
		if err != nil {
			add(rel, err.Error())
			continue
		}
		p.Tasks = append(p.Tasks, t)
	}
	sort.Slice(p.Tasks, func(i, j int) bool { return p.Tasks[i].Meta.ID < p.Tasks[j].Meta.ID })
	if len(findings) > 0 {
		return p, &plan.ValidationError{Findings: plan.SortedFindings(findings)}
	}
	return p, nil
}

func (w *Workspace) CheckAgents() error {
	p, err := w.safe("AGENTS.md", false)
	if err != nil {
		return err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	return validateAgents(b, true)
}
