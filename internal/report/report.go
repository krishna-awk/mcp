package report

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/qaforge/mcp/internal/coverage"
	"github.com/qaforge/mcp/internal/scanner"
)

type BugReport struct {
	Title      string `json:"title"`
	Severity   string `json:"severity"`
	Steps      string `json:"steps"`
	Expected   string `json:"expected"`
	Actual     string `json:"actual"`
	Screenshot string `json:"screenshot,omitempty"`
	Trace      string `json:"trace,omitempty"`
	CreatedAt  string `json:"createdAt"`
}

func New(title, severity, steps, expected, actual, screenshot, trace string) *BugReport {
	return &BugReport{
		Title:      title,
		Severity:   severity,
		Steps:      steps,
		Expected:   expected,
		Actual:     actual,
		Screenshot: screenshot,
		Trace:      trace,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}
}

func (b *BugReport) Markdown() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n", b.Title)
	fmt.Fprintf(&sb, "**Severity:** %s  \n", b.Severity)
	fmt.Fprintf(&sb, "**Created:** %s\n\n", b.CreatedAt)
	fmt.Fprintf(&sb, "## Steps to Reproduce\n\n%s\n\n", b.Steps)
	fmt.Fprintf(&sb, "## Expected\n\n%s\n\n", b.Expected)
	fmt.Fprintf(&sb, "## Actual\n\n%s\n\n", b.Actual)
	if b.Screenshot != "" {
		fmt.Fprintf(&sb, "## Screenshot\n\n`%s`\n\n", b.Screenshot)
	}
	if b.Trace != "" {
		fmt.Fprintf(&sb, "## Trace\n\n`%s`\n", b.Trace)
	}
	return sb.String()
}

type Summary struct {
	Project    string                  `json:"project"`
	Framework  string                  `json:"framework"`
	StartedAt  string                  `json:"startedAt,omitempty"`
	FinishedAt string                  `json:"finishedAt,omitempty"`
	Pages      int                     `json:"pages"`
	APIs       int                     `json:"apis"`
	Forms      int                     `json:"forms"`
	Workflows  int                     `json:"workflows"`
	Specs      []string                `json:"specs"`
	BugReports []string                `json:"bugReports,omitempty"`
	Coverage   coverage.Coverage       `json:"coverage"`
	Auth       scanner.AuthInfo        `json:"auth"`
	Gaps       []string                `json:"gaps"`
	Language   string                  `json:"language,omitempty"`
	Target     string                  `json:"target,omitempty"`
}

func RenderHTML(s Summary, outDir string) (string, error) {
	rows := []struct {
		Label string
		Got   int
		Cov   float64
	}{
		{"Pages", s.Pages, s.Coverage.PageCoverage},
		{"Workflows", s.Workflows, s.Coverage.WorkflowCoverage},
		{"APIs", s.APIs, s.Coverage.APICoverage},
		{"Forms", s.Forms, s.Coverage.FormCoverage},
	}
	var sb strings.Builder
	sb.WriteString(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>QAForge Report</title>
<meta name="viewport" content="width=device-width,initial-scale=1">
<style>
:root{--bg:#0f1115;--fg:#e5e7eb;--muted:#9ca3af;--card:#1a1d24;--line:#2a2f3a;--ok:#10b981;--warn:#f59e0b;--bad:#ef4444;--acc:#3b82f6;}
@media(prefers-color-scheme:light){:root{--bg:#fff;--fg:#111827;--muted:#6b7280;--card:#f9fafb;--line:#e5e7eb;--ok:#059669;--warn:#d97706;--bad:#dc2626;--acc:#2563eb;}}
*{box-sizing:border-box}
body{font:14px/1.5 -apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif;background:var(--bg);color:var(--fg);margin:0;padding:24px;max-width:1100px;margin:auto}
h1{margin:0 0 8px}
h2{margin:32px 0 12px;border-bottom:1px solid var(--line);padding-bottom:6px}
.sub{color:var(--muted);font-size:13px;margin-bottom:24px}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:12px;margin:16px 0 24px}
.card{background:var(--card);border:1px solid var(--line);border-radius:8px;padding:16px}
.card .n{font-size:28px;font-weight:600;margin:0}
.card .l{color:var(--muted);font-size:12px;text-transform:uppercase;letter-spacing:.5px}
.bar{height:8px;background:var(--line);border-radius:4px;overflow:hidden;margin-top:8px}
.bar > div{height:100%;background:var(--ok)}
.bar > div.warn{background:var(--warn)}
.bar > div.bad{background:var(--bad)}
table{width:100%;border-collapse:collapse;margin-top:8px}
th,td{text-align:left;padding:8px 10px;border-bottom:1px solid var(--line);font-size:13px}
th{color:var(--muted);font-weight:500;text-transform:uppercase;font-size:11px;letter-spacing:.5px}
code{background:var(--card);padding:2px 6px;border-radius:4px;font-size:12px}
.badge{display:inline-block;padding:2px 8px;border-radius:999px;font-size:11px;font-weight:600;background:var(--line)}
.badge.ok{background:var(--ok);color:#fff}
.badge.warn{background:var(--warn);color:#fff}
.badge.bad{background:var(--bad);color:#fff}
ul.gaps{list-style:none;padding:0}
ul.gaps li{padding:8px 12px;background:var(--card);border:1px solid var(--line);border-radius:6px;margin:6px 0;font-size:13px}
footer{margin-top:48px;padding-top:16px;border-top:1px solid var(--line);color:var(--muted);font-size:12px}
</style></head><body>
`)
	fmt.Fprintf(&sb, `<h1>QAForge Report</h1>`)
	fmt.Fprintf(&sb, `<div class="sub">%s &middot; <code>%s</code> &middot; %s</div>`, html.EscapeString(s.Project), html.EscapeString(s.Framework), html.EscapeString(s.StartedAt))

	fmt.Fprintf(&sb, `<h2>Coverage</h2><div class="cards">`)
	for _, r := range rows {
		cls := "ok"
		if r.Cov < 50 {
			cls = "bad"
		} else if r.Cov < 80 {
			cls = "warn"
		}
		fmt.Fprintf(&sb, `<div class="card"><div class="l">%s</div><div class="n">%.0f%%</div><div class="bar"><div class="%s" style="width:%.1f%%"></div></div><div class="sub">%d / %d</div></div>`, r.Label, r.Cov, cls, r.Cov, int(r.Cov/100*float64(r.Got)), r.Got)
	}
	fmt.Fprintf(&sb, `</div>`)

	fmt.Fprintf(&sb, `<h2>Auth</h2><table><tr><th>Signal</th><th>Detected</th></tr>`)
	fmt.Fprintf(&sb, `<tr><td>Login forms</td><td>%s</td></tr>`, yesno(len(s.Auth.LoginForms) > 0, fmt.Sprintf("%d found", len(s.Auth.LoginForms))))
	fmt.Fprintf(&sb, `<tr><td>OAuth (Google/GitHub/etc.)</td><td>%s</td></tr>`, yesno(s.Auth.HasOAuth, strings.Join(s.Auth.OAuthHints, ", ")))
	fmt.Fprintf(&sb, `<tr><td>JWT</td><td>%s</td></tr>`, yesno(s.Auth.HasJWT, ""))
	fmt.Fprintf(&sb, `<tr><td>API key</td><td>%s</td></tr>`, yesno(s.Auth.HasAPIKey, ""))
	fmt.Fprintf(&sb, `<tr><td>Session cookies</td><td>%s</td></tr>`, yesno(s.Auth.HasSession, ""))
	fmt.Fprintf(&sb, `</table>`)
	fmt.Fprintf(&sb, `<div class="sub">Required env vars: <code>%s</code></div>`, html.EscapeString(strings.Join(s.Auth.EnvVars, "</code> <code>")))

	if len(s.Specs) > 0 {
		fmt.Fprintf(&sb, `<h2>Generated Specs (%d)</h2><table><tr><th>File</th></tr>`, len(s.Specs))
		for _, sp := range s.Specs {
			fmt.Fprintf(&sb, `<tr><td><code>%s</code></td></tr>`, html.EscapeString(sp))
		}
		fmt.Fprintf(&sb, `</table>`)
	}

	if len(s.BugReports) > 0 {
		fmt.Fprintf(&sb, `<h2>Bug Reports (%d)</h2><table><tr><th>File</th></tr>`, len(s.BugReports))
		for _, b := range s.BugReports {
			fmt.Fprintf(&sb, `<tr><td><code>%s</code></td></tr>`, html.EscapeString(b))
		}
		fmt.Fprintf(&sb, `</table>`)
	}

	if len(s.Gaps) > 0 {
		fmt.Fprintf(&sb, `<h2>Coverage Gaps (not auto-discoverable)</h2><ul class="gaps">`)
		for _, g := range s.Gaps {
			fmt.Fprintf(&sb, `<li>%s</li>`, html.EscapeString(g))
		}
		fmt.Fprintf(&sb, `</ul>`)
	}

	fmt.Fprintf(&sb, `<footer>Generated by QAForge MCP &middot; <code>qaforge-mcp</code></footer></body></html>`)

	out := filepath.Join(outDir, "report.html")
	if err := os.WriteFile(out, []byte(sb.String()), 0o644); err != nil {
		return "", err
	}
	return out, nil
}

func RenderMarkdown(s Summary, outDir string) (string, error) {
	summaryJSON := filepath.Join(outDir, "summary.json")
	if b, err := json.MarshalIndent(s, "", "  "); err == nil {
		_ = os.WriteFile(summaryJSON, b, 0o644)
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "# QAForge Coverage Report\n\n")
	fmt.Fprintf(&sb, "**Project:** `%s`  \n", s.Project)
	fmt.Fprintf(&sb, "**Framework:** `%s`  \n", s.Framework)
	if s.StartedAt != "" {
		fmt.Fprintf(&sb, "**Started:** %s  \n", s.StartedAt)
	}
	if s.Target != "" {
		fmt.Fprintf(&sb, "**Target:** `%s`  \n", s.Target)
	}
	fmt.Fprintf(&sb, "\n## Coverage\n\n")
	fmt.Fprintf(&sb, "| Surface | Total | Coverage |\n|---|---:|---:|\n")
	fmt.Fprintf(&sb, "| Pages    | %d | %.1f%% |\n", s.Pages, s.Coverage.PageCoverage)
	fmt.Fprintf(&sb, "| Workflows| %d | %.1f%% |\n", s.Workflows, s.Coverage.WorkflowCoverage)
	fmt.Fprintf(&sb, "| APIs     | %d | %.1f%% |\n", s.APIs, s.Coverage.APICoverage)
	fmt.Fprintf(&sb, "| Forms    | %d | %.1f%% |\n", s.Forms, s.Coverage.FormCoverage)

	fmt.Fprintf(&sb, "\n## Auth detected\n\n")
	fmt.Fprintf(&sb, "- **Login forms:** %d\n", len(s.Auth.LoginForms))
	fmt.Fprintf(&sb, "- **OAuth:** %s", yesno(s.Auth.HasOAuth, ""))
	if len(s.Auth.OAuthHints) > 0 {
		fmt.Fprintf(&sb, " (%s)", strings.Join(s.Auth.OAuthHints, ", "))
	}
	fmt.Fprintf(&sb, "\n- **JWT:** %s\n", yesno(s.Auth.HasJWT, ""))
	fmt.Fprintf(&sb, "- **API key:** %s\n", yesno(s.Auth.HasAPIKey, ""))
	fmt.Fprintf(&sb, "- **Session:** %s\n", yesno(s.Auth.HasSession, ""))
	fmt.Fprintf(&sb, "\nRequired env vars: `%s`\n", strings.Join(s.Auth.EnvVars, "`, `"))

	if len(s.Specs) > 0 {
		fmt.Fprintf(&sb, "\n## Generated specs (%d)\n\n", len(s.Specs))
		for _, sp := range s.Specs {
			fmt.Fprintf(&sb, "- `%s`\n", sp)
		}
	}

	if len(s.BugReports) > 0 {
		fmt.Fprintf(&sb, "\n## Bug reports (%d)\n\n", len(s.BugReports))
		for _, b := range s.BugReports {
			fmt.Fprintf(&sb, "- `%s`\n", b)
		}
	}

	if len(s.Gaps) > 0 {
		fmt.Fprintf(&sb, "\n## Gaps (need manual coverage)\n\n")
		for _, g := range s.Gaps {
			fmt.Fprintf(&sb, "- %s\n", g)
		}
	}

	out := filepath.Join(outDir, "summary.md")
	if err := os.WriteFile(out, []byte(sb.String()), 0o644); err != nil {
		return "", err
	}
	return out, nil
}

func InferGaps(a *scanner.AnalyzeResult, s Summary) []string {
	gaps := []string{}
	if a.Auth.HasOAuth {
		gaps = append(gaps, "OAuth flows (login, callback, refresh) - use test accounts, mock the provider")
	}
	if a.Auth.HasJWT {
		gaps = append(gaps, "JWT expiry / refresh / revocation paths")
	}
	if a.Auth.HasSession {
		gaps = append(gaps, "Session timeout, concurrent sessions, logout-everywhere")
	}
	if len(a.Forms) > 0 {
		gaps = append(gaps, "Form validation edge cases (unicode, XSS, oversized input)")
	}
	if len(a.APIs) > 0 {
		gaps = append(gaps, "API error paths (4xx, 5xx, timeouts) and rate limiting")
	}
	sort.Strings(gaps)
	return gaps
}

func yesno(b bool, extra string) string {
	if b {
		if extra != "" {
			return "yes — " + extra
		}
		return "yes"
	}
	return "no"
}
