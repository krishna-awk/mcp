package specgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/qaforge/mcp/internal/scanner"
	"github.com/qaforge/mcp/internal/workflow"
)

type Target string

const (
	TargetTS Target = "ts"
	TargetPY Target = "py"
)

func Generate(analyze *scanner.AnalyzeResult, workflows []workflow.Workflow, outDir string, target Target) ([]string, error) {
	if target == "" {
		target = TargetTS
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}

	created := []string{}
	ext := ".spec.ts"
	render := renderSpecTS
	switch target {
	case TargetPY:
		ext = "_spec.py"
		render = renderSpecPY
		if err := writeAuthHelperPY(outDir); err != nil {
			return nil, err
		}
		created = append(created, filepath.Join(outDir, "auth_helper.py"))
	case TargetTS:
		if err := writeAuthHelperTS(outDir); err != nil {
			return nil, err
		}
		created = append(created, filepath.Join(outDir, "auth_helper.ts"))
	}

	for i, wf := range workflows {
		spec := render(analyze, wf, i+1)
		name := slugify(wf.Name) + ext
		full := filepath.Join(outDir, name)
		if err := os.WriteFile(full, []byte(spec), 0o644); err != nil {
			return created, fmt.Errorf("write %s: %w", full, err)
		}
		created = append(created, full)
	}
	if len(workflows) == 0 && len(analyze.Pages) > 0 {
		var spec string
		if target == TargetPY {
			spec = renderSmokeSpecPY(analyze)
			full := filepath.Join(outDir, "smoke_spec.py")
			if err := os.WriteFile(full, []byte(spec), 0o644); err != nil {
				return created, fmt.Errorf("write smoke: %w", err)
			}
			created = append(created, full)
		} else {
			spec = renderSmokeSpecTS(analyze)
			full := filepath.Join(outDir, "smoke.spec.ts")
			if err := os.WriteFile(full, []byte(spec), 0o644); err != nil {
				return created, fmt.Errorf("write smoke: %w", err)
			}
			created = append(created, full)
		}
	}
	return created, nil
}

func isAuthWorkflow(wf workflow.Workflow) bool {
	low := strings.ToLower(wf.Name)
	for _, s := range wf.Steps {
		t := strings.ToLower(s.Target)
		if strings.Contains(t, "password") || strings.Contains(t, "email") || strings.Contains(t, "login") {
			return true
		}
	}
	return strings.Contains(low, "login") || strings.Contains(low, "sign in") || strings.Contains(low, "auth")
}

// ---------- TypeScript ----------

func renderSpecTS(a *scanner.AnalyzeResult, wf workflow.Workflow, idx int) string {
	var sb strings.Builder
	auth := isAuthWorkflow(wf)
	fmt.Fprintf(&sb, "import { test, expect } from '@playwright/test';\n")
	if auth {
		fmt.Fprintf(&sb, "import { qaLogin } from './auth_helper';\n")
	}
	fmt.Fprintf(&sb, "\n")
	fmt.Fprintf(&sb, "test.describe('%s', () => {\n", escape(wf.Name))
	fmt.Fprintf(&sb, "  test('happy path: %s', async ({ page }) => {\n", escape(wf.Name))
	fmt.Fprintf(&sb, "    // Framework: %s\n", a.Framework)
	fmt.Fprintf(&sb, "    // Steps: %d\n", len(wf.Steps))
	fmt.Fprintf(&sb, "    const baseURL = process.env.QA_BASE_URL || 'http://localhost:3000';\n")
	if auth {
		fmt.Fprintf(&sb, "    const ctx = await qaLogin(page);\n")
	} else {
		fmt.Fprintf(&sb, "    await page.goto(baseURL);\n")
	}
	for i, step := range wf.Steps {
		fmt.Fprintf(&sb, "    // Step %d: %s -> %s\n", i+1, step.Action, step.Target)
	}
	fmt.Fprintf(&sb, "    // TODO: replace placeholder assertions with real checks.\n")
	fmt.Fprintf(&sb, "    await expect(page.locator('body')).toBeVisible();\n")
	if auth {
		fmt.Fprintf(&sb, "    await ctx.close();\n")
	}
	fmt.Fprintf(&sb, "  });\n\n")
	fmt.Fprintf(&sb, "  test('negative path: %s', async ({ page }) => {\n", escape(wf.Name))
	fmt.Fprintf(&sb, "    const baseURL = process.env.QA_BASE_URL || 'http://localhost:3000';\n")
	fmt.Fprintf(&sb, "    await page.goto(baseURL);\n")
	fmt.Fprintf(&sb, "    // TODO: assert error / validation path\n")
	fmt.Fprintf(&sb, "    await expect(page.locator('body')).toBeVisible();\n")
	fmt.Fprintf(&sb, "  });\n")
	fmt.Fprintf(&sb, "});\n")
	return sb.String()
}

func renderSmokeSpecTS(a *scanner.AnalyzeResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "import { test, expect } from '@playwright/test';\n\n")
	fmt.Fprintf(&sb, "test.describe('smoke', () => {\n")
	fmt.Fprintf(&sb, "  test('all discovered pages return 2xx', async ({ page }) => {\n")
	fmt.Fprintf(&sb, "    const baseURL = process.env.QA_BASE_URL || 'http://localhost:3000';\n")
	fmt.Fprintf(&sb, "    const routes = [\n")
	for _, p := range a.Pages {
		fmt.Fprintf(&sb, "      '%s',\n", escape(p.Route))
	}
	fmt.Fprintf(&sb, "    ];\n")
	fmt.Fprintf(&sb, "    for (const route of routes) {\n")
	fmt.Fprintf(&sb, "      const res = await page.goto(baseURL + route);\n")
	fmt.Fprintf(&sb, "      expect(res?.status() ?? 0).toBeLessThan(400);\n")
	fmt.Fprintf(&sb, "    }\n")
	fmt.Fprintf(&sb, "  });\n")
	fmt.Fprintf(&sb, "});\n")
	return sb.String()
}

func writeAuthHelperTS(outDir string) error {
	helper := `// QAForge auth helper. Reads credentials from environment.
// Required env vars: QA_USER, QA_PASSWORD
// Optional: QA_BASE_URL (default: http://localhost:3000)
import { BrowserContext, Page } from '@playwright/test';

export async function qaLogin(page: Page): Promise<BrowserContext> {
  const user = process.env.QA_USER;
  const password = process.env.QA_PASSWORD;
  if (!user || !password) {
    throw new Error('QA_USER and QA_PASSWORD env vars are required for auth tests');
  }
  const baseURL = process.env.QA_BASE_URL || 'http://localhost:3000';
  await page.goto(baseURL + '/login');
  await page.fill('input[name=email], input[name=username]', user);
  await page.fill('input[name=password]', password);
  await page.click('button[type=submit]');
  // Wait for redirect away from /login
  await page.waitForURL((url) => !url.pathname.startsWith('/login'), { timeout: 10000 });
  return page.context();
}

export async function qaApiToken(): Promise<string> {
  const token = process.env.QA_API_TOKEN;
  if (!token) {
    throw new Error('QA_API_TOKEN env var is required for API auth tests');
  }
  return token;
}
`
	return os.WriteFile(filepath.Join(outDir, "auth_helper.ts"), []byte(helper), 0o644)
}

// ---------- Python ----------

func renderSpecPY(a *scanner.AnalyzeResult, wf workflow.Workflow, idx int) string {
	var sb strings.Builder
	auth := isAuthWorkflow(wf)
	fmt.Fprintf(&sb, "import os\n")
	fmt.Fprintf(&sb, "from playwright.sync_api import Page, expect\n")
	if auth {
		fmt.Fprintf(&sb, "from auth_helper import qa_login\n")
	}
	fmt.Fprintf(&sb, "\n")
	fmt.Fprintf(&sb, "BASE_URL = os.environ.get('QA_BASE_URL', 'http://localhost:3000')\n\n")
	fmt.Fprintf(&sb, "def test_happy_path_%s(page: Page):\n", slugify(wf.Name))
	fmt.Fprintf(&sb, "    \"\"\"%s\n\n", escape(wf.Name))
	fmt.Fprintf(&sb, "    Framework: %s\n", a.Framework)
	fmt.Fprintf(&sb, "    Steps: %d\n", len(wf.Steps))
	for i, step := range wf.Steps {
		fmt.Fprintf(&sb, "    %d. %s -> %s\n", i+1, step.Action, step.Target)
	}
	fmt.Fprintf(&sb, "    \"\"\"\n")
	if auth {
		fmt.Fprintf(&sb, "    qa_login(page)\n")
	} else {
		fmt.Fprintf(&sb, "    page.goto(BASE_URL)\n")
	}
	fmt.Fprintf(&sb, "    # TODO: replace with real assertions\n")
	fmt.Fprintf(&sb, "    expect(page.locator('body')).to_be_visible()\n\n")
	fmt.Fprintf(&sb, "def test_negative_path_%s(page: Page):\n", slugify(wf.Name))
	fmt.Fprintf(&sb, "    page.goto(BASE_URL)\n")
	fmt.Fprintf(&sb, "    # TODO: assert error / validation path\n")
	fmt.Fprintf(&sb, "    expect(page.locator('body')).to_be_visible()\n")
	return sb.String()
}

func renderSmokeSpecPY(a *scanner.AnalyzeResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "import os\n")
	fmt.Fprintf(&sb, "from playwright.sync_api import Page, expect\n\n")
	fmt.Fprintf(&sb, "BASE_URL = os.environ.get('QA_BASE_URL', 'http://localhost:3000')\n")
	fmt.Fprintf(&sb, "ROUTES = [\n")
	for _, p := range a.Pages {
		fmt.Fprintf(&sb, "    '%s',\n", escape(p.Route))
	}
	fmt.Fprintf(&sb, "]\n\n")
	fmt.Fprintf(&sb, "def test_all_pages_2xx(page: Page):\n")
	fmt.Fprintf(&sb, "    for route in ROUTES:\n")
	fmt.Fprintf(&sb, "        response = page.goto(BASE_URL + route)\n")
	fmt.Fprintf(&sb, "        assert response is not None and response.status < 400, f'{route} returned {response.status if response else None}'\n")
	return sb.String()
}

func writeAuthHelperPY(outDir string) error {
	helper := `# QAForge auth helper. Reads credentials from environment.
# Required env vars: QA_USER, QA_PASSWORD
# Optional: QA_BASE_URL (default: http://localhost:3000)
import os
from playwright.sync_api import Page

def qa_login(page: Page) -> None:
    """Sign in via /login form using env credentials. Raises if not set."""
    user = os.environ.get('QA_USER')
    password = os.environ.get('QA_PASSWORD')
    if not user or not password:
        raise RuntimeError('QA_USER and QA_PASSWORD env vars are required for auth tests')
    base_url = os.environ.get('QA_BASE_URL', 'http://localhost:3000')
    page.goto(base_url + '/login')
    page.fill('input[name=email], input[name=username]', user)
    page.fill('input[name=password]', password)
    page.click('button[type=submit]')
    page.wait_for_url(lambda url: not url.path.startswith('/login'), timeout=10000)

def qa_api_token() -> str:
    token = os.environ.get('QA_API_TOKEN')
    if not token:
        raise RuntimeError('QA_API_TOKEN env var is required for API auth tests')
    return token
`
	return os.WriteFile(filepath.Join(outDir, "auth_helper.py"), []byte(helper), 0o644)
}

// ---------- shared ----------

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			return r
		case r == ' ' || r == '-' || r == '_' || r == '/':
			return '-'
		}
		return -1
	}, s)
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		return "test"
	}
	return s
}

func escape(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}
