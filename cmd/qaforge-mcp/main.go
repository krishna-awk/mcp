package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/qaforge/mcp/internal/coverage"
	"github.com/qaforge/mcp/internal/db"
	"github.com/qaforge/mcp/internal/pipeline"
	"github.com/qaforge/mcp/internal/playwright"
	"github.com/qaforge/mcp/internal/scanner"
	"github.com/qaforge/mcp/internal/specgen"
	"github.com/qaforge/mcp/internal/testplan"
	"github.com/qaforge/mcp/internal/tools"
	"github.com/qaforge/mcp/internal/workflow"
)

func main() {
	if len(os.Args) > 1 {
		runCLI(os.Args[1:])
		return
	}
	runMCPServer()
}

func runMCPServer() {
	storePath, err := defaultStorePath()
	if err != nil {
		log.Fatalf("resolve store path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
		log.Fatalf("create store dir: %v", err)
	}
	conn, err := db.Open(storePath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer conn.Close()

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "qaforge-mcp",
		Version: "0.1.0",
	}, &mcp.ServerOptions{
		Instructions: "QAForge MCP: application analysis, workflow discovery, test plan generation, Playwright execution, visual diff, database verification, bug reports, and coverage analysis.",
	})

	tools.RegisterAll(server, conn)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func runCLI(args []string) {
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "analyze":
		mustRun(analyzeCmd(rest))
	case "workflow":
		mustRun(workflowCmd(rest))
	case "plan":
		mustRun(planCmd(rest))
	case "spec":
		mustRun(specCmd(rest))
	case "test":
		mustRun(testCmd(rest))
	case "coverage":
		mustRun(coverageCmd(rest))
	case "run":
		mustRun(runCmd(rest))
	case "watch":
		mustRun(watchCmd(rest))
	case "init":
		mustRun(initCmd(rest))
	case "version", "-v", "--version":
		fmt.Println("qaforge-mcp 0.1.0")
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`qaforge-mcp — local QA automation (MCP server + CLI)

Usage:
  qaforge-mcp                       Run as MCP server (stdio)
  qaforge-mcp <command> [flags]     Run as CLI

Commands:
  analyze      Scan a project, print pages/APIs/forms
  workflow     Discover user workflows
  plan         Generate a Gherkin test plan
  spec         Generate Playwright .spec.ts files
  test         Run Playwright tests
  coverage     Print coverage report
  run          Full autonomous pipeline (analyze -> spec -> test -> report -> coverage)
  watch        Re-run pipeline on file changes
  init         Install git hooks and starter files
  version      Print version
  help         Show this help

Global flags (per command):
  -project <path>    Project root (default: .)
  -out <dir>         Output directory for reports (default: <project>/qa-reports)
  -tests <dir>       Output directory for spec files (default: <project>/tests/qaforge)
  -base <url>        Base URL for browser tests (default: $BASE_URL or http://localhost:3000)
  -json              Machine-readable output
  -no-test           Skip running tests (plan/spec only)

Examples:
  qaforge-mcp run -project=./myapp
  qaforge-mcp analyze -project=./myapp -json
  qaforge-mcp watch -project=./myapp
  qaforge-mcp init -project=./myapp`)
}

type commonFlags struct {
	project string
	out     string
	tests   string
	base    string
	jsonOut bool
	noTest  bool
	target  string
}

func parseCommon(name string, args []string) (*commonFlags, *flag.FlagSet) {
	fs := flag.NewFlagSet(name, flag.ExitOnError)
	cf := &commonFlags{}
	fs.StringVar(&cf.project, "project", ".", "project root")
	fs.StringVar(&cf.out, "out", "", "output directory for reports")
	fs.StringVar(&cf.tests, "tests", "", "output directory for spec files")
	fs.StringVar(&cf.base, "base", "", "base URL for browser tests")
	fs.BoolVar(&cf.jsonOut, "json", false, "machine-readable output")
	fs.BoolVar(&cf.noTest, "no-test", false, "skip running tests")
	fs.StringVar(&cf.target, "target", "ts", "spec target language: ts (Playwright TypeScript) or py (Playwright Python)")
	_ = fs.Parse(args)
	if cf.project == "" {
		cf.project = "."
	}
	return cf, fs
}

func analyzeCmd(args []string) error {
	cf, _ := parseCommon("analyze", args)
	a, err := scanner.Analyze(cf.project)
	if err != nil {
		return err
	}
	if cf.jsonOut {
		return printJSON(a)
	}
	fmt.Printf("Framework: %s\n", a.Framework)
	fmt.Printf("Pages:     %d\n", len(a.Pages))
	fmt.Printf("APIs:      %d\n", len(a.APIs))
	fmt.Printf("Forms:     %d\n", len(a.Forms))
	return nil
}

func workflowCmd(args []string) error {
	cf, _ := parseCommon("workflow", args)
	a, err := scanner.Analyze(cf.project)
	if err != nil {
		return err
	}
	wfs := workflow.Discover(a)
	if cf.jsonOut {
		return printJSON(map[string]any{"framework": a.Framework, "workflows": wfs})
	}
	fmt.Printf("Found %d workflows:\n", len(wfs))
	for _, w := range wfs {
		fmt.Printf("  - %s (%d steps)\n", w.Name, len(w.Steps))
	}
	return nil
}

func planCmd(args []string) error {
	cf, _ := parseCommon("plan", args)
	a, err := scanner.Analyze(cf.project)
	if err != nil {
		return err
	}
	wfs := workflow.Discover(a)
	plan := testplan.Generate(wfs)
	outDir := cf.out
	if outDir == "" {
		outDir = filepath.Join(cf.project, "qa-reports")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	planPath := filepath.Join(outDir, "test-plan.feature")
	if err := os.WriteFile(planPath, []byte(plan.Text), 0o644); err != nil {
		return err
	}
	fmt.Printf("Test plan: %s\n", planPath)
	return nil
}

func specCmd(args []string) error {
	cf, _ := parseCommon("spec", args)
	a, err := scanner.Analyze(cf.project)
	if err != nil {
		return err
	}
	wfs := workflow.Discover(a)
	testsDir := cf.tests
	if testsDir == "" {
		testsDir = filepath.Join(cf.project, "tests", "qaforge")
	}
	specs, err := specgen.Generate(a, wfs, testsDir, specgen.Target(cf.target))
	if err != nil {
		return err
	}
	fmt.Printf("Generated %d specs in %s\n", len(specs), testsDir)
	for _, s := range specs {
		fmt.Printf("  - %s\n", s)
	}
	return nil
}

func testCmd(args []string) error {
	cf, _ := parseCommon("test", args)
	testsDir := cf.tests
	if testsDir == "" {
		testsDir = filepath.Join(cf.project, "tests", "qaforge")
	}
	patterns := []string{
		filepath.Join(testsDir, "*.spec.ts"),
		filepath.Join(testsDir, "*_spec.py"),
	}
	matches := []string{}
	for _, p := range patterns {
		m, err := filepath.Glob(p)
		if err != nil {
			return err
		}
		matches = append(matches, m...)
	}
	if len(matches) == 0 {
		fmt.Printf("No specs found in %s\n", testsDir)
		return nil
	}
	failed := 0
	for _, spec := range matches {
		rel, _ := filepath.Rel(cf.project, spec)
		run, err := playwright.Run(rel)
		if err != nil && run == nil {
			return err
		}
		fmt.Printf("[%s] %s\n", run.Status, rel)
		if run.Status != "passed" {
			failed++
		}
	}
	if failed > 0 {
		fmt.Printf("%d/%d failed\n", failed, len(matches))
		os.Exit(1)
	}
	fmt.Printf("All %d passed\n", len(matches))
	return nil
}

func coverageCmd(args []string) error {
	cf, _ := parseCommon("coverage", args)
	a, err := scanner.Analyze(cf.project)
	if err != nil {
		return err
	}
	c := coverage.Calculate(len(a.Pages), 0, len(a.Forms), 0, len(a.APIs), 0, len(a.Forms), 0)
	if cf.jsonOut {
		return printJSON(c)
	}
	fmt.Printf("Page:     %.1f%%\n", c.PageCoverage)
	fmt.Printf("Workflow: %.1f%%\n", c.WorkflowCoverage)
	fmt.Printf("API:      %.1f%%\n", c.APICoverage)
	fmt.Printf("Form:     %.1f%%\n", c.FormCoverage)
	return nil
}

func runCmd(args []string) error {
	cf, _ := parseCommon("run", args)
	_, err := pipeline.Run(pipeline.Options{
		ProjectPath: cf.project,
		OutDir:      cf.out,
		TestsDir:    cf.tests,
		BaseURL:     cf.base,
		RunTests:    !cf.noTest,
		JSON:        cf.jsonOut,
		Target:      specgen.Target(cf.target),
	})
	return err
}

func watchCmd(args []string) error {
	cf, _ := parseCommon("watch", args)
	return runWatch(cf.project, cf)
}

func initCmd(args []string) error {
	cf, _ := parseCommon("init", args)
	return scaffoldProject(cf.project)
}

func printJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}

func mustRun(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func defaultStorePath() (string, error) {
	if v := os.Getenv("QAFORGE_STORE"); v != "" {
		return v, nil
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "qaforge-mcp", "store.db"), nil
}
