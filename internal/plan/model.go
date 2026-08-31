package plan

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

const (
	SchemaV1       = "go-plan/v1"
	TaskOpen       = "open"
	TaskInProgress = "in_progress"
	TaskDone       = "done"
)

var (
	acceptanceIDPattern = regexp.MustCompile(`^AC-[0-9]{3,}$`)
	taskIDPattern       = regexp.MustCompile(`^T-[0-9]{3,}$`)
)

type Metadata struct {
	Schema         string  `yaml:"schema"`
	Title          string  `yaml:"title"`
	ApprovalDigest *string `yaml:"approval_digest"`
}

type Task struct {
	Schema             string          `yaml:"schema"`
	ID                 string          `yaml:"id"`
	Title              string          `yaml:"title"`
	State              string          `yaml:"state"`
	Covers             []string        `yaml:"covers"`
	Sections           []Section       `yaml:"-"`
	Deliverables       []ChecklistItem `yaml:"-"`
	AcceptanceCriteria []ChecklistItem `yaml:"-"`
	Raw                []byte          `yaml:"-"`
	Path               string          `yaml:"-"`
}

type ChecklistItem struct {
	Checked bool
	Text    string
}

func ParseMetadata(data []byte) (Metadata, error) {
	var metadata Metadata
	if err := strictYAML(data, &metadata, map[string]yamlKind{
		"schema":          kindString,
		"title":           kindString,
		"approval_digest": kindNullableString,
	}); err != nil {
		return Metadata{}, fmt.Errorf("parse plan metadata: %w", err)
	}
	if metadata.Schema != SchemaV1 {
		return Metadata{}, fmt.Errorf("parse plan metadata: schema must be %q", SchemaV1)
	}
	if metadata.Title == "" {
		return Metadata{}, fmt.Errorf("parse plan metadata: title must not be empty")
	}
	return metadata, nil
}

func RenderMetadata(metadata Metadata) ([]byte, error) {
	if metadata.Schema != SchemaV1 {
		return nil, fmt.Errorf("render plan metadata: schema must be %q", SchemaV1)
	}
	if metadata.Title == "" {
		return nil, fmt.Errorf("render plan metadata: title must not be empty")
	}
	digest := "null"
	if metadata.ApprovalDigest != nil {
		digest = strconv.Quote(*metadata.ApprovalDigest)
	}
	return []byte("schema: " + strconv.Quote(metadata.Schema) + "\n" +
		"title: " + strconv.Quote(metadata.Title) + "\n" +
		"approval_digest: " + digest + "\n"), nil
}

func ParseTask(filename string, data []byte) (Task, error) {
	frontmatter, body, err := splitFrontmatter(data)
	if err != nil {
		return Task{}, err
	}
	var task Task
	if err := strictYAML(frontmatter, &task, map[string]yamlKind{
		"schema": kindString,
		"id":     kindString,
		"title":  kindString,
		"state":  kindString,
		"covers": kindStringList,
	}); err != nil {
		return Task{}, fmt.Errorf("parse task frontmatter: %w", err)
	}
	if task.Schema != SchemaV1 {
		return Task{}, fmt.Errorf("parse task: schema must be %q", SchemaV1)
	}
	if !validNumericID(task.ID, taskIDPattern, "T-") {
		return Task{}, fmt.Errorf("parse task: invalid task ID %q", task.ID)
	}
	wantFilename := strings.ToLower(task.ID) + ".md"
	if filepath.Base(filename) != wantFilename {
		return Task{}, fmt.Errorf("parse task: filename %q does not match ID %q", filepath.Base(filename), task.ID)
	}
	if task.Title == "" {
		return Task{}, fmt.Errorf("parse task: title must not be empty")
	}
	if task.State != TaskOpen && task.State != TaskInProgress && task.State != TaskDone {
		return Task{}, fmt.Errorf("parse task: invalid state %q", task.State)
	}
	for _, id := range task.Covers {
		if !validNumericID(id, acceptanceIDPattern, "AC-") {
			return Task{}, fmt.Errorf("parse task: invalid coverage ID %q", id)
		}
	}
	if !bytes.HasPrefix(body, []byte("# "+task.ID+": "+task.Title+"\n")) {
		return Task{}, fmt.Errorf("parse task: title heading must match ID and title")
	}
	doc, err := parseMarkdown(body, taskHeadings)
	if err != nil {
		return Task{}, fmt.Errorf("parse task: %w", err)
	}
	deliverables, err := parseChecklist(doc.sectionBody("Deliverables"))
	if err != nil {
		return Task{}, fmt.Errorf("parse task deliverables: %w", err)
	}
	criteria, err := parseChecklist(doc.sectionBody("Acceptance criteria"))
	if err != nil {
		return Task{}, fmt.Errorf("parse task acceptance criteria: %w", err)
	}
	task.Sections = doc.Sections
	task.Deliverables = deliverables
	task.AcceptanceCriteria = criteria
	task.Raw = append([]byte(nil), data...)
	return task, nil
}

func splitFrontmatter(data []byte) ([]byte, []byte, error) {
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return nil, nil, fmt.Errorf("parse task: missing YAML frontmatter")
	}
	rest := data[len("---\n"):]
	end := bytes.Index(rest, []byte("\n---\n"))
	if end < 0 {
		return nil, nil, fmt.Errorf("parse task: unterminated YAML frontmatter")
	}
	frontmatter := rest[:end]
	body := rest[end+len("\n---\n"):]
	if len(body) > 0 && body[0] == '\n' {
		body = body[1:]
	}
	return frontmatter, body, nil
}

type yamlKind int

const (
	kindString yamlKind = iota
	kindNullableString
	kindStringList
)

func strictYAML(data []byte, destination any, fields map[string]yamlKind) error {
	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return err
	}
	if len(file.Docs) != 1 {
		return fmt.Errorf("expected exactly one YAML document")
	}
	for _, nodeType := range []ast.NodeType{ast.AnchorType, ast.AliasType, ast.TagType} {
		if len(ast.FilterFile(nodeType, file)) != 0 {
			return fmt.Errorf("anchors, aliases, and custom tags are not allowed")
		}
	}
	var raw map[string]any
	if err := yaml.UnmarshalWithOptions(data, &raw, yaml.Strict()); err != nil {
		return err
	}
	if len(raw) != len(fields) {
		return fmt.Errorf("expected exactly %d fields", len(fields))
	}
	for name, kind := range fields {
		value, ok := raw[name]
		if !ok {
			return fmt.Errorf("missing field %q", name)
		}
		if err := validateYAMLKind(name, value, kind); err != nil {
			return err
		}
	}
	if err := yaml.UnmarshalWithOptions(data, destination, yaml.Strict()); err != nil {
		return err
	}
	return nil
}

func validateYAMLKind(name string, value any, kind yamlKind) error {
	switch kind {
	case kindString:
		if _, ok := value.(string); !ok {
			return fmt.Errorf("field %q must be a string", name)
		}
	case kindNullableString:
		if value != nil {
			if _, ok := value.(string); !ok {
				return fmt.Errorf("field %q must be a string or null", name)
			}
		}
	case kindStringList:
		values, ok := value.([]any)
		if !ok {
			// goccy decodes an empty flow sequence as []interface{} and a
			// populated sequence identically; any other value is invalid.
			return fmt.Errorf("field %q must be a list of strings", name)
		}
		for _, item := range values {
			if _, ok := item.(string); !ok {
				return fmt.Errorf("field %q must be a list of strings", name)
			}
		}
	}
	return nil
}

func validNumericID(value string, pattern *regexp.Regexp, prefix string) bool {
	if !pattern.MatchString(value) {
		return false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(value, prefix))
	return err == nil && n > 0
}
