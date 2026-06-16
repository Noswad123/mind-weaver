package parser

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestMarkdownParser_ExtractsWikiLinks(t *testing.T) {
	t.Parallel()

	parsed, err := MarkdownParser{}.Parse(`
# Hub

- [[benefits]]
- [[ez-pass|EZ Pass]]
- [[medical#Insurance]]
`, "introspection/remember/hub.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(parsed.Links) != 3 {
		t.Fatalf("expected 3 links, got %d: %#v", len(parsed.Links), parsed.Links)
	}

	if parsed.Links[0].Target != "benefits" || parsed.Links[0].Label != "benefits" || parsed.Links[0].Type != "internal" {
		t.Fatalf("unexpected first wikilink: %#v", parsed.Links[0])
	}
	if parsed.Links[1].Target != "ez-pass" || parsed.Links[1].Label != "EZ Pass" {
		t.Fatalf("unexpected aliased wikilink: %#v", parsed.Links[1])
	}
	if parsed.Links[2].Target != "medical" {
		t.Fatalf("expected heading fragment to be stripped: %#v", parsed.Links[2])
	}
}

func TestMarkdownParser_ExtractsStandardMarkdownLinks(t *testing.T) {
	t.Parallel()

	parsed, err := MarkdownParser{}.Parse(`
[Local](child.md)
[Relative](../other/hub.md#section)
[Absolute](/Users/me/notes/absolute.md)
[External](https://example.com)
![Image](diagram.png)
`, "areas/current/hub.md")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(parsed.Links) != 4 {
		t.Fatalf("expected 4 links, got %d: %#v", len(parsed.Links), parsed.Links)
	}

	if parsed.Links[0].Type != "internal" || parsed.Links[0].ResolvedPath != "areas/current/child.md" {
		t.Fatalf("unexpected local markdown link: %#v", parsed.Links[0])
	}
	if parsed.Links[1].Type != "internal" || parsed.Links[1].ResolvedPath != "areas/other/hub.md" {
		t.Fatalf("unexpected relative markdown link: %#v", parsed.Links[1])
	}
	if parsed.Links[2].Type != "internal" || parsed.Links[2].ResolvedPath != "/Users/me/notes/absolute.md" {
		t.Fatalf("unexpected absolute markdown link: %#v", parsed.Links[2])
	}
	if parsed.Links[3].Type != "external" || parsed.Links[3].Target != "https://example.com" {
		t.Fatalf("unexpected external markdown link: %#v", parsed.Links[3])
	}
}

func TestMarkdownParser_RootAwareAbsoluteLinks(t *testing.T) {
	t.Parallel()
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Fatalf("home dir: %v", err)
	}
	notesRoot := filepath.Join(home, "notes")
	absUnderRoot := filepath.ToSlash(filepath.Join(notesRoot, "autodactyl/topic.md"))
	absOutsideRoot := filepath.ToSlash(filepath.Join(home, "elsewhere/topic.md"))

	content := fmt.Sprintf(`
[UnderRoot](%s#part)
[TildeUnderRoot](~/notes/autodactyl/tilde.md?x=1)
[OutsideRoot](%s)
[Relative](../other.md)
`, absUnderRoot, absOutsideRoot)
	parsed, err := MarkdownParser{}.ParseWithContext(content, ParseContext{
		SourceRelPath: "areas/current/hub.md",
		NotesRootAbs:  notesRoot,
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(parsed.Links) != 4 {
		t.Fatalf("expected 4 links, got %d: %#v", len(parsed.Links), parsed.Links)
	}
	if parsed.Links[0].ResolvedPath != "autodactyl/topic.md" {
		t.Fatalf("expected absolute root link to become repo-relative: %#v", parsed.Links[0])
	}
	if parsed.Links[1].ResolvedPath != "autodactyl/tilde.md" {
		t.Fatalf("expected tilde root link to become repo-relative: %#v", parsed.Links[1])
	}
	if parsed.Links[2].ResolvedPath != "" {
		t.Fatalf("expected outside-root absolute link to be unresolved: %#v", parsed.Links[2])
	}
	if parsed.Links[3].ResolvedPath != "areas/other.md" {
		t.Fatalf("expected relative link to remain source-relative: %#v", parsed.Links[3])
	}
}
