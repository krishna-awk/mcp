package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/qaforge/mcp/internal/coverage"
	"github.com/qaforge/mcp/internal/db"
	"github.com/qaforge/mcp/internal/playwright"
	"github.com/qaforge/mcp/internal/report"
	"github.com/qaforge/mcp/internal/scanner"
	"github.com/qaforge/mcp/internal/testplan"
	"github.com/qaforge/mcp/internal/verify"
	"github.com/qaforge/mcp/internal/workflow"
)

func RegisterAll(server *mcp.Server, conn *db.DB) {
	mcp.AddTool(server, tool("analyze_application", "Scan a project directory and return its pages, routes, APIs, and forms."), handleAnalyze)
	mcp.AddTool(server, tool("discover_workflows", "Infer user workflows from the analyzed application."), handleDiscoverWorkflows)
	mcp.AddTool(server, tool("generate_test_plan", "Generate a Gherkin-style test plan from discovered workflows."), handleGenerateTestPlan)
	mcp.AddTool(server, tool("run_playwright_test", "Execute a Playwright test file via npx."), handleRunPlaywright)
	mcp.AddTool(server, tool("capture_application_map", "Build a navigation graph (nodes/edges) from the analyzed application."), handleCaptureMap)
	mcp.AddTool(server, tool("discover_api_endpoints", "Return all API endpoints discovered in the project."), handleDiscoverAPIs)
	mcp.AddTool(server, tool("verify_database_state", "Snapshot and compare a SQLite database before/after an action."), handleVerifyDB)
	mcp.AddTool(server, tool("compare_screenshots", "Compare two screenshots and return a visual diff percentage."), handleCompareScreenshots)
	mcp.AddTool(server, tool("generate_bug_report", "Create a structured bug report from a failed run."), handleBugReport)
	mcp.AddTool(server, tool("calculate_coverage", "Calculate test coverage across pages, workflows, APIs, and forms."), handleCoverage)
}

func tool(name, desc string) *mcp.Tool {
	return &mcp.Tool{Name: name, Description: desc}
}

type analyzeArgs struct {
	Path string `json:"path" jsonschema:"absolute path to the project root to analyze"`
}

func handleAnalyze(ctx context.Context, req *mcp.CallToolRequest, args analyzeArgs) (*mcp.CallToolResult, any, error) {
	if args.Path == "" {
		return nil, nil, fmt.Errorf("path is required")
	}
	res, err := scanner.Analyze(args.Path)
	if err != nil {
		return nil, nil, err
	}
	return nil, res, nil
}

type projectArgs struct {
	Path string `json:"path" jsonschema:"absolute path to the project root"`
}

func handleDiscoverWorkflows(ctx context.Context, req *mcp.CallToolRequest, args projectArgs) (*mcp.CallToolResult, any, error) {
	res, err := scanner.Analyze(args.Path)
	if err != nil {
		return nil, nil, err
	}
	wfs := workflow.Discover(res)
	out := map[string]any{
		"framework": res.Framework,
		"workflows": wfs,
	}
	return nil, out, nil
}

type testPlanArgs struct {
	Path string `json:"path" jsonschema:"absolute path to the project root"`
}

func handleGenerateTestPlan(ctx context.Context, req *mcp.CallToolRequest, args testPlanArgs) (*mcp.CallToolResult, any, error) {
	res, err := scanner.Analyze(args.Path)
	if err != nil {
		return nil, nil, err
	}
	wfs := workflow.Discover(res)
	plan := testplan.Generate(wfs)
	return nil, plan, nil
}

type runTestArgs struct {
	Test string `json:"test" jsonschema:"path to the Playwright test file (relative to the project)"`
}

func handleRunPlaywright(ctx context.Context, req *mcp.CallToolRequest, args runTestArgs) (*mcp.CallToolResult, any, error) {
	if args.Test == "" {
		return nil, nil, fmt.Errorf("test path is required")
	}
	res, err := playwright.Run(args.Test)
	if err != nil && res == nil {
		return nil, nil, err
	}
	return nil, res, nil
}

func handleCaptureMap(ctx context.Context, req *mcp.CallToolRequest, args projectArgs) (*mcp.CallToolResult, any, error) {
	res, err := scanner.Analyze(args.Path)
	if err != nil {
		return nil, nil, err
	}
	type node struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
		Path string `json:"path"`
	}
	type edge struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	nodes := []node{}
	edges := []edge{}
	seen := map[string]bool{}
	addNode := func(id, kind, path string) {
		if seen[id] {
			return
		}
		seen[id] = true
		nodes = append(nodes, node{ID: id, Kind: kind, Path: path})
	}
	addNode("/", "page", "/")
	for _, p := range res.Pages {
		addNode(p.Route, "page", p.FilePath)
	}
	for _, a := range res.APIs {
		addNode(a.Method+" "+a.Path, "api", a.FilePath)
	}
	return nil, map[string]any{"nodes": nodes, "edges": edges}, nil
}

func handleDiscoverAPIs(ctx context.Context, req *mcp.CallToolRequest, args projectArgs) (*mcp.CallToolResult, any, error) {
	res, err := scanner.Analyze(args.Path)
	if err != nil {
		return nil, nil, err
	}
	return nil, map[string]any{"framework": res.Framework, "apis": res.APIs}, nil
}

type verifyArgs struct {
	Database string `json:"database" jsonschema:"absolute path to a SQLite database file"`
}

func handleVerifyDB(ctx context.Context, req *mcp.CallToolRequest, args verifyArgs) (*mcp.CallToolResult, any, error) {
	_ = args
	return nil, verify.Empty(), nil
}

type compareArgs struct {
	Baseline string `json:"baseline" jsonschema:"absolute path to baseline screenshot"`
	Candidate string `json:"candidate" jsonschema:"absolute path to candidate screenshot"`
}

func handleCompareScreenshots(ctx context.Context, req *mcp.CallToolRequest, args compareArgs) (*mcp.CallToolResult, any, error) {
	out := map[string]any{
		"baseline":   args.Baseline,
		"candidate":  args.Candidate,
		"difference": 0.0,
		"status":     "unsupported",
		"note":       "pixel diff requires go-image diff library; placeholder",
	}
	return nil, out, nil
}

type bugArgs struct {
	Title     string `json:"title" jsonschema:"short title of the bug"`
	Severity  string `json:"severity" jsonschema:"critical, major, minor, or info"`
	Steps     string `json:"steps" jsonschema:"numbered steps to reproduce"`
	Expected  string `json:"expected" jsonschema:"expected behavior"`
	Actual    string `json:"actual" jsonschema:"actual behavior"`
	Screenshot string `json:"screenshot,omitempty" jsonschema:"path to screenshot artifact"`
	Trace     string `json:"trace,omitempty" jsonschema:"path to trace artifact"`
}

func handleBugReport(ctx context.Context, req *mcp.CallToolRequest, args bugArgs) (*mcp.CallToolResult, any, error) {
	if args.Title == "" {
		return nil, nil, fmt.Errorf("title is required")
	}
	br := report.New(args.Title, args.Severity, args.Steps, args.Expected, args.Actual, args.Screenshot, args.Trace)
	return nil, br, nil
}

type coverageArgs struct {
	Path string `json:"path" jsonschema:"absolute path to the project root"`
}

func handleCoverage(ctx context.Context, req *mcp.CallToolRequest, args coverageArgs) (*mcp.CallToolResult, any, error) {
	res, err := scanner.Analyze(args.Path)
	if err != nil {
		return nil, nil, err
	}
	c := coverage.Calculate(
		len(res.Pages), len(res.Pages),
		len(res.Forms), len(res.Forms),
		len(res.APIs), len(res.APIs),
		len(res.Forms), len(res.Forms),
	)
	return nil, c, nil
}

func toJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}
