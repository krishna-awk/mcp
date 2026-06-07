package testplan

import (
	"fmt"
	"strings"

	"github.com/qaforge/mcp/internal/workflow"
)

type Scenario struct {
	Name   string   `json:"name"`
	Given  []string `json:"given"`
	When   []string `json:"when"`
	Then   []string `json:"then"`
}

type Feature struct {
	Name      string     `json:"name"`
	Scenarios []Scenario `json:"scenarios"`
}

type Plan struct {
	Features []Feature `json:"features"`
	Text     string    `json:"text"`
}

func Generate(workflows []workflow.Workflow) Plan {
	plan := Plan{Features: []Feature{}}
	var sb strings.Builder
	for _, wf := range workflows {
		f := Feature{Name: wf.Name, Scenarios: []Scenario{}}
		scenario := Scenario{
			Name:  "Happy path for " + wf.Name,
			Given: []string{"the application is running"},
			When:  []string{},
			Then:  []string{"the workflow completes successfully"},
		}
		for _, s := range wf.Steps {
			scenario.When = append(scenario.When, fmt.Sprintf("user performs %s on %s", s.Action, s.Target))
		}
		f.Scenarios = append(f.Scenarios, scenario)
		f.Scenarios = append(f.Scenarios, Scenario{
			Name:   "Negative path for " + wf.Name,
			Given:  []string{"the application is running"},
			When:  []string{"invalid input is provided"},
			Then:  []string{"the workflow surfaces a validation error"},
		})
		plan.Features = append(plan.Features, f)
		fmt.Fprintf(&sb, "Feature: %s\n\n", f.Name)
		for _, sc := range f.Scenarios {
			fmt.Fprintf(&sb, "  Scenario: %s\n", sc.Name)
			for _, g := range sc.Given {
				fmt.Fprintf(&sb, "    Given %s\n", g)
			}
			for _, w := range sc.When {
				fmt.Fprintf(&sb, "    When %s\n", w)
			}
			for _, t := range sc.Then {
				fmt.Fprintf(&sb, "    Then %s\n", t)
			}
			fmt.Fprintln(&sb)
		}
	}
	plan.Text = sb.String()
	return plan
}
