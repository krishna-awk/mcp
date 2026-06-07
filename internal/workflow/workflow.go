package workflow

import (
	"github.com/qaforge/mcp/internal/scanner"
)

type Step struct {
	Action  string `json:"action"`
	Target  string `json:"target"`
	Outcome string `json:"outcome,omitempty"`
}

type Workflow struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Steps       []Step `json:"steps"`
}

func Discover(analyze *scanner.AnalyzeResult) []Workflow {
	workflows := []Workflow{}
	seen := map[string]bool{}
	for _, f := range analyze.Forms {
		key := formKey(f.Action, f.Method)
		if seen[key] {
			continue
		}
		seen[key] = true
		wf := Workflow{
			Name:        formName(f),
			Description: "Form submission workflow",
			Steps: []Step{
				{Action: "navigate", Target: f.Action},
				{Action: "fill", Target: "form fields"},
				{Action: "submit", Target: f.Method},
			},
		}
		workflows = append(workflows, wf)
	}
	return workflows
}

func formName(f scanner.Form) string {
	act := f.Action
	if act == "" {
		act = f.Method
	}
	if act == "" {
		return "Submit Form"
	}
	return "Submit " + act
}

func formKey(action, method string) string {
	return action + "|" + method
}
