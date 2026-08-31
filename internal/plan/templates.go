package plan

import (
	"embed"
	"strings"
)

const Placeholder = "TODO"

//go:embed templates/*.md
var templateFiles embed.FS

func renderTemplate(filename, title string) []byte {
	data, _ := templateFiles.ReadFile("templates/" + filename)
	return []byte(strings.ReplaceAll(string(data), "{{TITLE}}", title))
}

func SpecificationTemplate(title string) []byte {
	return renderTemplate("specification.md", title)
}

func ImplementationTemplate(title string) []byte {
	return renderTemplate("implementation-plan.md", title)
}

func NewTask(id, title string, covers []string) Task {
	data, _ := templateFiles.ReadFile("templates/task.md")
	doc, _ := ParseDocument(data, TaskHeadings)
	return Task{Meta: TaskMeta{Schema: Schema, ID: id, Title: title, State: StateOpen, Covers: covers}, Document: doc}
}
