package db

import (
	"context"
	"testing"
)

func TestListGraphEdges_ResolvesByPathAndNoteUID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	noteDB := newTestNoteDB(t)

	_, err := noteDB.conn.Exec(`
		INSERT INTO notes (id, path, title) VALUES
			(1, 'hub.md', 'Hub'),
			(2, 'benefits.md', 'Benefits'),
			(3, 'areas/current/child.md', 'Child');
		INSERT INTO note_ids (note_id, note_uid, path, is_hub) VALUES
			(1, 'hub', 'hub.md', 1),
			(2, 'benefits', 'benefits.md', 0),
			(3, 'child', 'areas/current/child.md', 0);
		INSERT INTO links (note_id, label, target, type, resolved_path) VALUES
			(1, 'Benefits', 'benefits', 'internal', 'benefits'),
			(1, 'Child', 'child', 'internal', 'areas/current/child.md');
	`)
	if err != nil {
		t.Fatalf("seed graph db: %v", err)
	}

	edges, err := noteDB.ListGraphEdges(ctx)
	if err != nil {
		t.Fatalf("list graph edges: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d: %#v", len(edges), edges)
	}

	seen := map[int]bool{}
	for _, edge := range edges {
		seen[edge.TargetID] = true
	}
	if !seen[2] || !seen[3] {
		t.Fatalf("expected targets 2 and 3, got %#v", edges)
	}
}
