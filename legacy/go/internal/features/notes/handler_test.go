package notes

import (
	"reflect"
	"testing"

	"github.com/Noswad123/mind-weaver/internal/core/note"
)

func TestGlossaryCategoryFromPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
		ok   bool
	}{
		{
			name: "nested glossary term path",
			path: "autodactyl/glossary/biology/rna.md",
			want: "biology",
			ok:   true,
		},
		{
			name: "top-level file has no category",
			path: "hub.md",
			want: "",
			ok:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := glossaryCategoryFromPath(tt.path)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("glossaryCategoryFromPath(%q) = (%q, %v); want (%q, %v)", tt.path, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestFilterGlossaryNotesByCategory(t *testing.T) {
	notes := []note.Note{
		{Path: "autodactyl/glossary/biology/rna.md", Title: "RNA"},
		{Path: "autodactyl/glossary/biology/dna.md", Title: "DNA"},
		{Path: "autodactyl/glossary/chemistry/molality.md", Title: "Molality"},
	}

	filtered := filterGlossaryNotesByCategory(notes, "biology")

	gotTitles := []string{}
	for _, n := range filtered {
		gotTitles = append(gotTitles, n.Title)
	}

	wantTitles := []string{"RNA", "DNA"}
	if !reflect.DeepEqual(gotTitles, wantTitles) {
		t.Fatalf("filtered titles = %#v; want %#v", gotTitles, wantTitles)
	}
}
