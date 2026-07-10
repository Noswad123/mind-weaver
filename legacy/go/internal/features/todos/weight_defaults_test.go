package todos

import (
	"strings"
	"testing"
)

func TestMetadataTokensFromLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line        string
		mustContain []string
	}{
		{line: "- priority: p1", mustContain: []string{"p1"}},
		{line: "- energy: x-large", mustContain: []string{"e:xl"}},
		{line: "- due: 2026-03-25", mustContain: []string{"due:2026-03-25"}},
		{line: "- start: 2026-03-20", mustContain: []string{"start:2026-03-20"}},
		{line: "- p2 e:s", mustContain: []string{"p2", "e:s"}},
		{line: "- priority: p5", mustContain: []string{"p5"}},
		{line: "- p:5", mustContain: []string{"p5"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.line, func(t *testing.T) {
			t.Parallel()
			got := metadataTokensFromLine(tt.line)
			for _, want := range tt.mustContain {
				if !strings.Contains(got, want) {
					t.Fatalf("metadataTokensFromLine(%q) = %q, expected to contain %q", tt.line, got, want)
				}
			}
		})
	}
}

func TestExtractTaskMetadataByKey(t *testing.T) {
	t.Parallel()

	content := `---
id: "work/index"
domains: [task-index]
task_active: true
task_area: Code
---
# Todo
## Next
- [ ] Ship parser area:Code
  - priority: p1
  - energy: xl
  - due: 2026-03-25
- [ ] Cleanup
  - p4 e:s start:2026-04-01
`

	got := extractTaskMetadataByKey(content, "work/index", "Code")
	shipKey := selectionKey("work/index", "Ship parser")
	cleanupKey := selectionKey("work/index", "Cleanup")

	shipVals, ok := got[shipKey]
	if !ok || len(shipVals) != 1 {
		t.Fatalf("expected one metadata entry for ship task, got: %#v", shipVals)
	}
	if !strings.Contains(shipVals[0], "p1") || !strings.Contains(shipVals[0], "e:xl") || !strings.Contains(shipVals[0], "due:2026-03-25") {
		t.Fatalf("unexpected ship metadata: %q", shipVals[0])
	}

	cleanupVals, ok := got[cleanupKey]
	if !ok || len(cleanupVals) != 1 {
		t.Fatalf("expected one metadata entry for cleanup task, got: %#v", cleanupVals)
	}
	if !strings.Contains(cleanupVals[0], "p4") || !strings.Contains(cleanupVals[0], "e:s") || !strings.Contains(cleanupVals[0], "start:2026-04-01") {
		t.Fatalf("unexpected cleanup metadata: %q", cleanupVals[0])
	}
}
