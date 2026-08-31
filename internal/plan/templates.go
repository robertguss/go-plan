package plan

import (
	"embed"
	"strings"
)

const Placeholder = "TODO"

//go:embed templates/*.md
var templateFiles embed.FS

func mustTemplate(name string) []byte {
	data, err := templateFiles.ReadFile("templates/" + name)
	if err != nil {
		panic(err)
	}
	return data
}

func renderTemplate(filename, title string) []byte {
	return []byte(strings.ReplaceAll(string(mustTemplate(filename)), "{{TITLE}}", title))
}

func SpecificationTemplate(title string) []byte {
	return renderTemplate("specification.md", title)
}

func ImplementationTemplate(title string) []byte {
	return renderTemplate("implementation-plan.md", title)
}

func NewTask(id, title string, covers []string) Task {
	doc, err := ParseDocument(mustTemplate("task.md"), TaskHeadings)
	if err != nil {
		panic(err)
	}
	return Task{Meta: TaskMeta{Schema: Schema, ID: id, Title: title, State: StateOpen, Covers: covers}, Document: doc}
}
