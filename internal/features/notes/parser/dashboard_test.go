package parser

import "testing"

func TestParseDashboard_UsesLevelTwoAreaHeadingsUnderDashboard(t *testing.T) {
	t.Parallel()

	content := `---
id: dashboard
---

# Dashboard

## Code
- [ ] Ship app fix [[source-a]]

## Action
- [x] Refill water [[source-b]]
`

	got := MarkdownParser{}.ParseDashboard(content, []string{"Code", "Action"})

	if len(got["Code"]) != 1 {
		t.Fatalf("expected 1 Code todo, got %d", len(got["Code"]))
	}
	if got["Code"][0].Text != "- [ ] Ship app fix [[source-a]]" || got["Code"][0].IsDone {
		t.Fatalf("unexpected Code todo: %#v", got["Code"][0])
	}

	if len(got["Action"]) != 1 {
		t.Fatalf("expected 1 Action todo, got %d", len(got["Action"]))
	}
	if got["Action"][0].Text != "- [x] Refill water [[source-b]]" || !got["Action"][0].IsDone {
		t.Fatalf("unexpected Action todo: %#v", got["Action"][0])
	}
}

func TestParseDashboard_IgnoresLegacyLevelOneAreaHeadings(t *testing.T) {
	t.Parallel()

	content := `# Code
- [ ] Ship app fix [[source-a]]
`

	got := MarkdownParser{}.ParseDashboard(content, []string{"Code"})
	if len(got["Code"]) != 0 {
		t.Fatalf("expected legacy Code todo to be ignored, got %d", len(got["Code"]))
	}
}
