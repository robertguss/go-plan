package plan

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/goccy/go-yaml"
)

var headingRE = regexp.MustCompile(`(?m)^## ([^\r\n]+)\r?$`)
var keyRE = regexp.MustCompile(`^([A-Za-z_][A-Za-z0-9_]*):`)
var yamlAnchorRE = regexp.MustCompile(`(^|[[:space:]])[&*][A-Za-z0-9_-]+`)
var yamlTagRE = regexp.MustCompile(`(^|:[[:space:]]+|-[[:space:]]+)!`)

func strictYAML(data []byte, out any, allowed map[string]bool) error {
	s := string(data)
	if strings.Contains(s, "\n---") || strings.Contains(s, "\n...") {
		return fmt.Errorf("multiple YAML documents are not allowed")
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(s, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		if yamlAnchorRE.MatchString(trim) || yamlTagRE.MatchString(trim) {
			return fmt.Errorf("aliases and custom tags are not allowed")
		}
		if strings.HasPrefix(trim, "-") {
			continue
		}
		if m := keyRE.FindStringSubmatch(trim); m != nil && len(line)-len(strings.TrimLeft(line, " \t")) == 0 {
			if !allowed[m[1]] {
				return fmt.Errorf("unknown field %q", m[1])
			}
			if seen[m[1]] {
				return fmt.Errorf("duplicate field %q", m[1])
			}
			seen[m[1]] = true
		}
	}
	if err := yaml.UnmarshalWithOptions(data, out, yaml.DisallowUnknownField()); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	for key := range allowed {
		if !seen[key] {
			return fmt.Errorf("missing required field %q", key)
		}
	}
	return nil
}

func ParseMetadata(data []byte) (Metadata, error) {
	var m Metadata
	err := strictYAML(data, &m, map[string]bool{"schema": true, "title": true, "approval_digest": true})
	if err == nil && m.Schema != Schema {
		err = fmt.Errorf("schema must be %q", Schema)
	}
	return m, err
}

func ParseDocument(data []byte, required []string) (Document, error) {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	matches := headingRE.FindAllStringSubmatchIndex(s, -1)
	if len(matches) != len(required) {
		return Document{}, fmt.Errorf("expected headings in canonical order: %s", strings.Join(required, ", "))
	}
	d := Document{Raw: s, Sections: map[string]string{}}
	for i, m := range matches {
		name := s[m[2]:m[3]]
		if name != required[i] {
			return Document{}, fmt.Errorf("expected heading %q at position %d", required[i], i+1)
		}
		end := len(s)
		if i+1 < len(matches) {
			end = matches[i+1][0]
		}
		d.Sections[name] = strings.TrimSpace(s[m[1]:end])
	}
	return d, nil
}

func ParseTask(data []byte, path string) (Task, error) {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return Task{}, fmt.Errorf("missing YAML frontmatter")
	}
	i := strings.Index(s[4:], "\n---\n")
	if i < 0 {
		return Task{}, fmt.Errorf("unterminated YAML frontmatter")
	}
	i += 4
	var m TaskMeta
	if err := strictYAML([]byte(s[4:i]), &m, map[string]bool{"schema": true, "id": true, "title": true, "state": true, "covers": true}); err != nil {
		return Task{}, err
	}
	if m.Schema != Schema {
		return Task{}, fmt.Errorf("schema must be %q", Schema)
	}
	d, err := ParseDocument([]byte(s[i+5:]), TaskHeadings)
	if err != nil {
		return Task{}, err
	}
	d.Raw = s
	return Task{Meta: m, Document: d, Path: path}, nil
}

func RenderMetadata(m Metadata) []byte {
	digest := "null"
	if m.ApprovalDigest != nil {
		digest = fmt.Sprintf("%q", *m.ApprovalDigest)
	}
	return []byte(fmt.Sprintf("schema: %q\ntitle: %q\napproval_digest: %s\n", m.Schema, m.Title, digest))
}

func RenderTask(t Task) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "---\nschema: %q\nid: %q\ntitle: %q\nstate: %q\ncovers:", Schema, t.Meta.ID, t.Meta.Title, t.Meta.State)
	if len(t.Meta.Covers) == 0 {
		b.WriteString(" []\n")
	} else {
		b.WriteByte('\n')
		for _, c := range t.Meta.Covers {
			fmt.Fprintf(&b, "  - %q\n", c)
		}
	}
	b.WriteString("---\n")
	for _, h := range TaskHeadings {
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", h, strings.TrimSpace(t.Sections[h]))
	}
	return b.Bytes()
}

func SortedFindings(in []Finding) []Finding {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].Path != in[j].Path {
			return in[i].Path < in[j].Path
		}
		if in[i].Field != in[j].Field {
			return in[i].Field < in[j].Field
		}
		return in[i].Message < in[j].Message
	})
	return in
}
