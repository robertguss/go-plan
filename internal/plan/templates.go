package plan

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed templates/*
var templateFS embed.FS

func RenderInitial(title string) (map[string][]byte, error) {
	if strings.TrimSpace(title) == "" {
		return nil, fmt.Errorf("render initial plan: title must not be empty")
	}
	metadata, err := RenderMetadata(Metadata{Schema: SchemaV1, Title: title})
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{"plan.yaml": metadata}
	assets := map[string]string{
		"specification.md":       "templates/specification.md.tmpl",
		"implementation-plan.md": "templates/implementation-plan.md.tmpl",
		"tasks/t-001.md":         "templates/t-001.md.tmpl",
	}
	for destination, source := range assets {
		content, err := templateFS.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("read embedded template %s: %w", source, err)
		}
		rendered := strings.ReplaceAll(string(content), "{{TITLE}}", title)
		rendered = strings.TrimRight(rendered, "\n") + "\n"
		files[destination] = []byte(rendered)
	}
	return files, nil
}
