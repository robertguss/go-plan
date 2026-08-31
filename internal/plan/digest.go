package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

type approvalContent struct {
	Specification  string                `json:"specification"`
	Implementation string                `json:"implementation_plan"`
	Tasks          []approvalTaskContent `json:"tasks"`
}

type approvalTaskContent struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Covers             []string `json:"covers"`
	Goal               string   `json:"goal"`
	Context            string   `json:"context"`
	Deliverables       []string `json:"deliverables"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	Verification       string   `json:"verification"`
	OutOfScope         string   `json:"out_of_scope"`
}

func ApprovalDigest(plan Plan) string {
	content := approvalContent{
		Specification:  string(plan.Specification.Raw),
		Implementation: string(plan.Implementation.Raw),
		Tasks:          make([]approvalTaskContent, 0, len(plan.Tasks)),
	}
	for _, task := range plan.Tasks {
		item := approvalTaskContent{
			ID:           task.ID,
			Title:        task.Title,
			Covers:       append([]string(nil), task.Covers...),
			Goal:         taskSectionBody(task, "Goal"),
			Context:      taskSectionBody(task, "Context"),
			Verification: taskSectionBody(task, "Verification"),
			OutOfScope:   taskSectionBody(task, "Out of scope"),
		}
		if item.Covers == nil {
			item.Covers = []string{}
		}
		for _, deliverable := range task.Deliverables {
			item.Deliverables = append(item.Deliverables, deliverable.Text)
		}
		for _, criterion := range task.AcceptanceCriteria {
			item.AcceptanceCriteria = append(item.AcceptanceCriteria, criterion.Text)
		}
		if item.Deliverables == nil {
			item.Deliverables = []string{}
		}
		if item.AcceptanceCriteria == nil {
			item.AcceptanceCriteria = []string{}
		}
		content.Tasks = append(content.Tasks, item)
	}
	canonical, _ := json.Marshal(content)
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:])
}

func taskSectionBody(task Task, heading string) string {
	for _, section := range task.Sections {
		if section.Heading == heading {
			return section.Body
		}
	}
	return ""
}
