package specgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/qaforge/mcp/internal/scanner"
	"github.com/qaforge/mcp/internal/workflow"
)

func Generate(analyze *scanner.AnalyzeResult, workflows []workflow.Workflow, outDir string) ([]string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	created := []string{}
	for i, wf := range workflows {
		spec := renderSpec(analyze, wf, i+1)
		name := slugify(wf.Name) + ".spec.ts"
		full := filepath.Join(outDir, name)
		if err := os.WriteFile(full, []byte(spec), 0o644); err != nil {
			return created, fmt.Errorf("write %s: %w", full, err)
		}
		created = append(created, full)
	}
	if len(workflows) == 0 && len(analyze.Pages) > 0 {
		spec := renderSmokeSpec(analyze)
		full := filepath.Join(outDir, "smoke.spec.ts")
		if err := os.WriteFile(full, []byte(spec), 0o644); err != nil {
			return created, fmt.Errorf("write smoke: %w", err)
		}
		created = append(created, full)
	}
	return created, nil
}

func renderSpec(a *scanner.AnalyzeResult, wf workflow.Workflow, idx int) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "import { test, expect } from '@playwright/test';\n\n")
	fmt.Fprintf(&sb, "test.describe('%s', () => {\n", escape(wf.Name))
	fmt.Fprintf(&sb, "  test('%s', async ({ page, request }) => {\n", escape("happy path: "+wf.Name))
	fmt.Fprintf(&sb, "    // Framework: %s\n", a.Framework)
	fmt.Fprintf(&sb, "    // Steps: %d\n", len(wf.Steps))
	fmt.Fprintf(&sb, "    // TODO: set BASE_URL via playwright.config.ts\n")
	fmt.Fprintf(&sb, "    const baseURL = process.env.BASE_URL || 'http://localhost:3000';\n\n")
	for i, step := range wf.Steps {
		fmt.Fprintf(&sb, "    // Step %d: %s -> %s\n", i+1, step.Action, step.Target)
	}
	fmt.Fprintf(&sb, "\n    // Generated placeholder: replace with real assertions.\n")
	fmt.Fprintf(&sb, "    await page.goto(baseURL);\n")
	fmt.Fprintf(&sb, "    await expect(page).toHaveTitle(/.*/);\n")
	fmt.Fprintf(&sb, "  });\n\n")
	fmt.Fprintf(&sb, "  test('negative path: %s', async ({ page }) => {\n", escape(wf.Name))
	fmt.Fprintf(&sb, "    const baseURL = process.env.BASE_URL || 'http://localhost:3000';\n")
	fmt.Fprintf(&sb, "    await page.goto(baseURL);\n")
	fmt.Fprintf(&sb, "    // TODO: assert error / validation path\n")
	fmt.Fprintf(&sb, "    await expect(page.locator('body')).toBeVisible();\n")
	fmt.Fprintf(&sb, "  });\n")
	fmt.Fprintf(&sb, "});\n")
	return sb.String()
}

func renderSmokeSpec(a *scanner.AnalyzeResult) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "import { test, expect } from '@playwright/test';\n\n")
	fmt.Fprintf(&sb, "test.describe('smoke', () => {\n")
	fmt.Fprintf(&sb, "  test('all discovered pages return 2xx', async ({ page }) => {\n")
	fmt.Fprintf(&sb, "    const baseURL = process.env.BASE_URL || 'http://localhost:3000';\n")
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
