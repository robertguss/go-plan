package plan

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var acceptanceLinePattern = regexp.MustCompile(`^- (AC-[0-9]{3,}): .+$`)

var specificationHeadings = []string{
	"Objective",
	"Context",
	"Users and workflows",
	"Goals",
	"Non-goals",
	"Assumptions",
	"Requirements",
	"Constraints",
	"Acceptance criteria",
	"Open questions",
}

var implementationHeadings = []string{
	"Approach",
	"Architecture",
	"Technology and dependencies",
	"Interfaces and data flow",
	"Change surface",
	"Verification strategy",
	"Decisions and tradeoffs",
	"Risks and recovery",
	"Out of scope",
}

var taskHeadings = []string{
	"Goal",
	"Context",
	"Deliverables",
	"Acceptance criteria",
	"Verification",
	"Evidence",
	"Out of scope",
}

type Section struct {
	Heading string
	Body    string
}

type MarkdownDocument struct {
	Sections      []Section
	AcceptanceIDs []string
	Raw           []byte
}

func ParseSpecification(data []byte) (MarkdownDocument, error) {
	doc, err := parseMarkdown(data, specificationHeadings)
	if err != nil {
		return MarkdownDocument{}, fmt.Errorf("parse specification: %w", err)
	}
	seen := map[string]struct{}{}
	for _, line := range strings.Split(doc.sectionBody("Acceptance criteria"), "\n") {
		if !strings.HasPrefix(line, "- AC-") {
			continue
		}
		match := acceptanceLinePattern.FindStringSubmatch(line)
		if match == nil || !validNumericID(strings.SplitN(strings.TrimPrefix(line, "- "), ":", 2)[0], acceptanceIDPattern, "AC-") {
			return MarkdownDocument{}, fmt.Errorf("parse specification: malformed acceptance criterion %q", line)
		}
		id := match[1]
		if _, exists := seen[id]; exists {
			return MarkdownDocument{}, fmt.Errorf("parse specification: duplicate acceptance criterion %s", id)
		}
		seen[id] = struct{}{}
		doc.AcceptanceIDs = append(doc.AcceptanceIDs, id)
	}
	return doc, nil
}

func ParseImplementationPlan(data []byte) (MarkdownDocument, error) {
	doc, err := parseMarkdown(data, implementationHeadings)
	if err != nil {
		return MarkdownDocument{}, fmt.Errorf("parse implementation plan: %w", err)
	}
	return doc, nil
}

func parseMarkdown(data []byte, required []string) (MarkdownDocument, error) {
	if !utf8.Valid(data) {
		return MarkdownDocument{}, fmt.Errorf("document is not valid UTF-8")
	}
	lines := strings.Split(string(data), "\n")
	type boundary struct {
		heading string
		line    int
	}
	var boundaries []boundary
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			boundaries = append(boundaries, boundary{heading: strings.TrimPrefix(line, "## "), line: i})
		}
	}
	if len(boundaries) != len(required) {
		return MarkdownDocument{}, fmt.Errorf("expected %d required sections, found %d", len(required), len(boundaries))
	}
	for i, heading := range required {
		if boundaries[i].heading != heading {
			return MarkdownDocument{}, fmt.Errorf("section %d must be %q, found %q", i+1, heading, boundaries[i].heading)
		}
	}
	doc := MarkdownDocument{Raw: append([]byte(nil), data...)}
	for i, current := range boundaries {
		end := len(lines)
		if i+1 < len(boundaries) {
			end = boundaries[i+1].line
		}
		body := strings.Join(lines[current.line+1:end], "\n")
		body = strings.Trim(body, "\n")
		doc.Sections = append(doc.Sections, Section{Heading: current.heading, Body: body})
	}
	return doc, nil
}

func (d MarkdownDocument) sectionBody(heading string) string {
	for _, section := range d.Sections {
		if section.Heading == heading {
			return section.Body
		}
	}
	return ""
}

func parseChecklist(body string) ([]ChecklistItem, error) {
	var items []ChecklistItem
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(line) < 7 || !strings.HasPrefix(line, "- [") || line[4] != ']' || line[5] != ' ' {
			return nil, fmt.Errorf("expected checklist item, found %q", line)
		}
		marker := line[3]
		if marker != ' ' && marker != 'x' && marker != 'X' {
			return nil, fmt.Errorf("invalid checklist marker in %q", line)
		}
		text := strings.TrimSpace(line[6:])
		if text == "" {
			return nil, fmt.Errorf("checklist text must not be empty")
		}
		items = append(items, ChecklistItem{Checked: marker == 'x' || marker == 'X', Text: text})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
