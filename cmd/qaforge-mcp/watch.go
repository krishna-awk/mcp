package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func runWatch(project string, cf *commonFlags) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("watcher: %w", err)
	}
	defer watcher.Close()

	abs, err := filepath.Abs(project)
	if err != nil {
		return err
	}
	if err := filepath.WalkDir(abs, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == ".git" || name == "dist" || name == "build" || name == "qa-reports" || name == "tests" {
				return filepath.SkipDir
			}
			return watcher.Add(p)
		}
		return nil
	}); err != nil {
		return err
	}

	fmt.Printf("Watching %s (Ctrl-C to stop)\n", abs)
	debounce := time.NewTimer(time.Hour)
	debounce.Stop()
	running := false
	pending := false
	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if isRelevant(ev.Name) {
				pending = true
				debounce.Reset(500 * time.Millisecond)
			}
		case <-debounce.C:
			if pending && !running {
				pending = false
				running = true
				fmt.Printf("\n[%s] change detected -> re-running pipeline\n", time.Now().Format("15:04:05"))
				_ = runOnce(project, cf)
				running = false
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "watch error: %v\n", err)
		}
	}
}

func runOnce(project string, cf *commonFlags) error {
	cmd := exec.Command(os.Args[0], "run",
		"-project="+project,
	)
	if cf.out != "" {
		cmd.Args = append(cmd.Args, "-out="+cf.out)
	}
	if cf.tests != "" {
		cmd.Args = append(cmd.Args, "-tests="+cf.tests)
	}
	if cf.base != "" {
		cmd.Args = append(cmd.Args, "-base="+cf.base)
	}
	if cf.noTest {
		cmd.Args = append(cmd.Args, "-no-test")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func isRelevant(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if strings.HasPrefix(base, ".") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".html", ".vue", ".svelte", ".feature":
		return true
	}
	return ext == ""
}

func scaffoldProject(project string) error {
	abs, err := filepath.Abs(project)
	if err != nil {
		return err
	}
	files := map[string]string{
		filepath.Join(abs, "qa-reports", ".gitkeep"):              "",
		filepath.Join(abs, "tests", "qaforge", ".gitkeep"):         "",
		filepath.Join(abs, "playwright.qaforge.config.ts"):         playwrightConfigTS,
		filepath.Join(abs, "pytest.qaforge.ini"):                   pytestINI,
		filepath.Join(abs, ".env.qaforge.example"):                 envExample,
		filepath.Join(abs, ".github", "workflows", "qaforge.yml"):  githubActionYML,
		filepath.Join(abs, "scripts", "qa-pre-commit.sh"):          preCommitSh,
		filepath.Join(abs, "CLAUDE.md"):                            claudeMD,
		filepath.Join(abs, ".cursorrules"):                         cursorRules,
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if _, err := os.Stat(path); err == nil {
			fmt.Printf("exists  %s\n", path)
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
		fmt.Printf("created %s\n", path)
	}
	hookPath := filepath.Join(abs, ".git", "hooks", "pre-commit")
	if _, err := os.Stat(filepath.Join(abs, ".git")); err == nil {
		hook := "#!/bin/sh\nqaforge-mcp run -project=. -no-test || exit 1\n"
		if err := os.WriteFile(hookPath, []byte(hook), 0o755); err == nil {
			fmt.Printf("created %s\n", hookPath)
		}
	}
	fmt.Printf("\nInitialized QAForge in %s\n", abs)
	fmt.Println("Next: qaforge-mcp run -project=.")
	return nil
}

const playwrightConfigTS = `import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests/qaforge',
  timeout: 30 * 1000,
  use: {
    baseURL: process.env.BASE_URL || 'http://localhost:3000',
    headless: true,
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },
  reporter: [['list'], ['json', { outputFile: 'qa-reports/playwright-results.json' }]],
});
`

const githubActionYML = `name: QAForge
on:
  push:
    branches: [main, develop]
  pull_request:
  schedule:
    - cron: '0 6 * * *'   # nightly at 06:00 UTC
jobs:
  qa:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - name: Install QAForge
        run: |
          go install github.com/qaforge/mcp/cmd/qaforge-mcp@latest
      - uses: actions/setup-node@v4
        with:
          node-version: '20'
      - name: Install Playwright
        run: npx playwright install --with-deps chromium
      - name: Run QA pipeline
        run: qaforge-mcp run -project=. -json
      - name: Upload reports
        if: always()
        uses: actions/upload-artifact@v4
        with:
          name: qa-reports
          path: qa-reports/
      - name: Comment PR
        if: github.event_name == 'pull_request'
        uses: actions/github-script@v7
        with:
          script: |
            const fs = require('fs');
            const mdPath = 'qa-reports/summary.md';
            if (!fs.existsSync(mdPath)) return;
            const body = fs.readFileSync(mdPath, 'utf8');
            github.rest.issues.createComment({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
              body,
            });
`

const preCommitSh = `#!/bin/sh
# QAForge pre-commit hook
# Runs the full QA pipeline (no-test mode for speed). Install:
#   cp scripts/qa-pre-commit.sh .git/hooks/pre-commit
#   chmod +x .git/hooks/pre-commit
set -e
cd "$(git rev-parse --show-toplevel)"
qaforge-mcp run -project=. -no-test
`

const envExample = `# QAForge environment variables.
# Copy to .env.qaforge (which is gitignored) and fill in real values.
# DO NOT commit real credentials.

# Target application
QA_BASE_URL=http://localhost:3000

# Form login (basic auth / username + password)
QA_USER=alice@test.com
QA_PASSWORD=changeme

# API auth (bearer token / API key)
QA_API_TOKEN=
`

const pytestINI = `[pytest]
testpaths = tests/qaforge
python_files = *_spec.py
python_classes = Test*
python_functions = test_*
addopts = -v --tb=short
`

const claudeMD = `# QA Instructions
When the user says "test this app", "run QA", or "check coverage", use QAForge MCP:
1. analyze_application
2. discover_workflows
3. generate_test_plan
4. Generate Playwright specs in ./tests/qaforge/
5. run_playwright_test
6. generate_bug_report on any failures (save to ./qa-reports/)
7. calculate_coverage

Default working project: current directory unless told otherwise.
Never run destructive commands without confirmation.
`

const cursorRules = `For any QA task, use the QAForge MCP tools.
Default workflow: analyze_application -> discover_workflows -> generate_test_plan
-> generate Playwright specs -> run_playwright_test -> generate_bug_report on failure
-> calculate_coverage.
Save artifacts to ./qa-reports/ and screenshots to ./qa-reports/screenshots/.
`
