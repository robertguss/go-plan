package plan

import (
	"embed"
	"strings"
)

const Placeholder = "TODO"

//go:embed templates/*.md
var templateFiles embed.FS

func SpecificationTemplate(title string) []byte {
	data, _ := templateFiles.ReadFile("templates/specification.md")
	return []byte(strings.ReplaceAll(string(data), "{{TITLE}}", title))
}

func ImplementationTemplate(title string) []byte {
	data, _ := templateFiles.ReadFile("templates/implementation-plan.md")
	return []byte(strings.ReplaceAll(string(data), "{{TITLE}}", title))
}

func NewTask(id, title string, covers []string) Task {
	data, _ := templateFiles.ReadFile("templates/task.md")
	doc, _ := ParseDocument(data, TaskHeadings)
	return Task{Meta: TaskMeta{Schema: Schema, ID: id, Title: title, State: "open", Covers: covers}, Document: doc}
}
