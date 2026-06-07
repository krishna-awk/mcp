package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/qaforge/mcp/internal/coverage"
	"github.com/qaforge/mcp/internal/playwright"
	"github.com/qaforge/mcp/internal/report"
	"github.com/qaforge/mcp/internal/scanner"
	"github.com/qaforge/mcp/internal/specgen"
	"github.com/qaforge/mcp/internal/testplan"
	"github.com/qaforge/mcp/internal/workflow"
)

type Options struct {
	ProjectPath string
	OutDir      string
	TestsDir    string
	BaseURL     string
	RunTests    bool
	JSON        bool
	Target      specgen.Target
}

type Result struct {
	Project    string                  `json:"project"`
	Framework  string                  `json:"framework"`
	StartedAt  string                  `json:"startedAt"`
	FinishedAt string                  `json:"finishedAt"`
	Pages      int                     `json:"pages"`
	APIs       int                     `json:"apis"`
	Forms      int                     `json:"forms"`
	Workflows  int                     `json:"workflows"`
	Specs      []string                `json:"specs"`
	TestRuns   []*playwright.RunResult `json:"testRuns,omitempty"`
	BugReports []string                `json:"bugReports,omitempty"`
	Coverage   coverage.Coverage       `json:"coverage"`
	Auth       scanner.AuthInfo        `json:"auth"`
	Gaps       []string                `json:"gaps"`
	Target     string                  `json:"target,omitempty"`
}

func Run(opts Options) (*Result, error) {
	if opts.OutDir == "" {
		opts.OutDir = filepath.Join(opts.ProjectPath, "qa-reports")
	}
	if opts.TestsDir == "" {
		opts.TestsDir = filepath.Join(opts.ProjectPath, "tests", "qaforge")
	}
	if opts.Target == "" {
		opts.Target = specgen.TargetTS
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir out: %w", err)
	}
	if err := os.MkdirAll(opts.TestsDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir tests: %w", err)
	}

	started := time.Now().UTC()
	res := &Result{
		Project:   opts.ProjectPath,
		Target:    string(opts.Target),
		StartedAt: started.Format(time.RFC3339),
	}

	analyze, err := scanner.Analyze(opts.ProjectPath)
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}
	res.Framework = analyze.Framework
	res.Pages = len(analyze.Pages)
	res.APIs = len(analyze.APIs)
	res.Forms = len(analyze.Forms)
	res.Auth = analyze.Auth

	wfs := workflow.Discover(analyze)
	res.Workflows = len(wfs)

	plan := testplan.Generate(wfs)
	planPath := filepath.Join(opts.OutDir, "test-plan.feature")
	if err := os.WriteFile(planPath, []byte(plan.Text), 0o644); err != nil {
		return nil, fmt.Errorf("write plan: %w", err)
	}

	specs, err := specgen.Generate(analyze, wfs, opts.TestsDir, opts.Target)
	if err != nil {
		return nil, fmt.Errorf("specgen: %w", err)
	}
	res.Specs = specs

	if opts.RunTests {
		for _, spec := range specs {
			rel, _ := filepath.Rel(opts.ProjectPath, spec)
			run, err := playwright.Run(rel)
			if err != nil && run == nil {
				return res, fmt.Errorf("playwright %s: %w", rel, err)
			}
			res.TestRuns = append(res.TestRuns, run)
			if run != nil && run.Status != "passed" {
				br := report.New(
					fmt.Sprintf("Test failure: %s", filepath.Base(rel)),
					"major",
					"Run the spec and observe the failure",
					"Test passes",
					run.Output,
					"",
					"",
				)
				md := br.Markdown()
				bugPath := filepath.Join(opts.OutDir, "bug-"+time.Now().UTC().Format("20060102-150405")+"-"+slugify(filepath.Base(rel))+".md")
				if err := os.WriteFile(bugPath, []byte(md), 0o644); err == nil {
					res.BugReports = append(res.BugReports, bugPath)
				}
			}
		}
	}

	c := coverage.Calculate(
		res.Pages, len(res.Specs),
		res.Workflows, len(res.Specs),
		res.APIs, len(res.Specs),
		res.Forms, len(res.Specs),
	)
	res.Coverage = c

	res.Gaps = report.InferGaps(analyze, report.Summary{
		Project: res.Project, Framework: res.Framework, Pages: res.Pages,
		APIs: res.APIs, Forms: res.Forms, Workflows: res.Workflows,
		Coverage: res.Coverage, Auth: res.Auth,
	})

	res.FinishedAt = time.Now().UTC().Format(time.RFC3339)

	sum := report.Summary{
		Project: res.Project, Framework: res.Framework, StartedAt: res.StartedAt, FinishedAt: res.FinishedAt,
		Pages: res.Pages, APIs: res.APIs, Forms: res.Forms, Workflows: res.Workflows,
		Specs: res.Specs, BugReports: res.BugReports, Coverage: res.Coverage,
		Auth: res.Auth, Gaps: res.Gaps, Target: res.Target,
	}
	summary, _ := json.MarshalIndent(sum, "", "  ")
	_ = os.WriteFile(filepath.Join(opts.OutDir, "summary.json"), summary, 0o644)

	if _, err := report.RenderMarkdown(sum, opts.OutDir); err != nil {
		return res, fmt.Errorf("markdown: %w", err)
	}
	if _, err := report.RenderHTML(sum, opts.OutDir); err != nil {
		return res, fmt.Errorf("html: %w", err)
	}

	if !opts.JSON {
		fmt.Printf("Project:      %s\n", res.Project)
		fmt.Printf("Framework:    %s\n", res.Framework)
		fmt.Printf("Target:       %s\n", res.Target)
		fmt.Printf("Pages:        %d\n", res.Pages)
		fmt.Printf("APIs:         %d\n", res.APIs)
		fmt.Printf("Forms:        %d\n", res.Forms)
		fmt.Printf("Workflows:    %d\n", res.Workflows)
		fmt.Printf("Specs:        %d\n", len(res.Specs))
		fmt.Printf("Bug reports:  %d\n", len(res.BugReports))
		fmt.Printf("Login forms:  %d\n", len(res.Auth.LoginForms))
		fmt.Printf("OAuth:        %v\n", res.Auth.HasOAuth)
		fmt.Printf("Page cov:     %.1f%%\n", res.Coverage.PageCoverage)
		fmt.Printf("Workflow cov: %.1f%%\n", res.Coverage.WorkflowCoverage)
		fmt.Printf("Reports:      %s\n", opts.OutDir)
		fmt.Printf("HTML report:  %s\n", filepath.Join(opts.OutDir, "report.html"))
	} else {
		fmt.Println(string(summary))
	}
	return res, nil
}

func slugify(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, r)
		case r == ' ' || r == '-' || r == '_' || r == '.':
			out = append(out, '-')
		}
	}
	return string(out)
}
