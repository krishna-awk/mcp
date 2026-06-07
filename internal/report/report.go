package report

import (
	"fmt"
	"strings"
	"time"
)

type BugReport struct {
	Title     string `json:"title"`
	Severity  string `json:"severity"`
	Steps     string `json:"steps"`
	Expected  string `json:"expected"`
	Actual    string `json:"actual"`
	Screenshot string `json:"screenshot,omitempty"`
	Trace     string `json:"trace,omitempty"`
	CreatedAt string `json:"createdAt"`
}

func New(title, severity, steps, expected, actual, screenshot, trace string) *BugReport {
	return &BugReport{
		Title:     title,
		Severity:  severity,
		Steps:     steps,
		Expected:  expected,
		Actual:    actual,
		Screenshot: screenshot,
		Trace:     trace,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
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
