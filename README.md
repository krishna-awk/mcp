# QAForge MCP

A local MCP server that turns any AI coding tool (Cursor, Claude Code, Antigravity, Gemini CLI, OpenCode, Cline, Roo) into an autonomous QA engineer.

You bring the LLM. QAForge brings the testing infrastructure.

## Why

Most MCP servers expose raw primitives (filesystem, git, Playwright). QAForge exposes **high-level QA operations** so the AI can reason about test strategy, not infrastructure plumbing.

```
AI Tool  →  QAForge MCP  →  App Analysis
                       →  Playwright
                       →  SQLite
                       →  Reports
                       →  Coverage
```

No API costs. No model management. The user already pays for Claude Max / Cursor Pro / Gemini.

## Tools

| Tool | Purpose |
|---|---|
| `analyze_application` | Scan a project, return pages, routes, APIs, forms |
| `discover_workflows` | Infer user workflows from the analyzed app |
| `generate_test_plan` | Produce a Gherkin-style test plan |
| `run_playwright_test` | Execute a Playwright spec via npx |
| `capture_application_map` | Build a navigation graph (nodes/edges) |
| `discover_api_endpoints` | List all APIs (OpenAPI, source, network) |
| `verify_database_state` | Snapshot + diff a SQLite database |
| `compare_screenshots` | Visual diff between two images |
| `generate_bug_report` | Create a structured bug report (Markdown) |
| `calculate_coverage` | Coverage % across pages, workflows, APIs, forms |

## Build

```bash
go build -o qaforge-mcp.exe ./cmd/qaforge-mcp
```

Cross-compile from any host:

```bash
GOOS=windows GOARCH=amd64 go build -o qaforge-mcp.exe      ./cmd/qaforge-mcp
GOOS=darwin  GOARCH=arm64 go build -o qaforge-mcp-macos   ./cmd/qaforge-mcp
GOOS=linux   GOARCH=amd64 go build -o qaforge-mcp-linux   ./cmd/qaforge-mcp
```

Static binary, no runtime, no cgo (pure-Go SQLite via `modernc.org/sqlite`).

## Attach to Cursor

`~/.cursor/mcp.json` (or `<project>/.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "qaforge": {
      "command": "C:/tools/qaforge-mcp.exe"
    }
  }
}
```

## Attach to Claude Code

```bash
claude mcp add qaforge -- "C:/tools/qaforge-mcp.exe"
```

## Attach to any MCP client

Any client that can spawn a process and speak JSON-RPC over stdio works. The server uses the official `github.com/modelcontextprotocol/go-sdk` and the standard newline-delimited JSON transport.

## Typical session in Claude Code

```
> Analyze the application at C:/projects/my-app.
> Discover all workflows.
> Generate a test plan.
> Generate Playwright tests for each workflow.
> Execute all tests.
> Create bug reports for any failures.
```

Claude will chain the tools and produce a working test suite.

## Storage

By default, QAForge stores its SQLite database at `%AppData%\qaforge-mcp\store.db` (macOS: `~/Library/Application Support/qaforge-mcp/store.db`, Linux: `~/.config/qaforge-mcp/store.db`).

Override with `QAFORGE_STORE=/path/to/store.db`.

## Architecture

```
cmd/qaforge-mcp/main.go      entry point, wires everything
internal/db/                 SQLite (modernc.org/sqlite, pure Go)
internal/scanner/            analyze_application, discover_api_endpoints
internal/workflow/           discover_workflows, capture_application_map
internal/testplan/           generate_test_plan
internal/playwright/         run_playwright_test, compare_screenshots
internal/report/             generate_bug_report
internal/coverage/           calculate_coverage
internal/verify/             verify_database_state
internal/tools/              MCP tool registration
artifacts/                   generated reports, screenshots, traces
```

## SQLite Schema

Seven tables, nothing else:

- `projects` — scanned projects
- `pages` — discovered pages
- `workflows` — inferred workflows
- `tests` — generated Playwright tests
- `runs` — test execution results
- `artifacts` — screenshots, traces, videos, reports
- `findings` — bugs and observations

See `internal/db/schema.sql`.

## Roadmap

- [ ] Persist scanned projects into SQLite (currently scanner is in-memory)
- [ ] Auto-generate Playwright test code from test plan
- [ ] Pixel-diff library for `compare_screenshots`
- [ ] Network traffic capture for `discover_api_endpoints`
- [ ] Real `verify_database_state` (snapshot tables before/after)
- [ ] HTML coverage report export
- [ ] Cross-platform release pipeline (Windows, macOS, Linux)
