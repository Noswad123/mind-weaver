package notes

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/core/note"
	"github.com/Noswad123/mind-weaver/internal/core/shared"
	"github.com/Noswad123/mind-weaver/internal/features/notes/parser"
	"github.com/Noswad123/mind-weaver/internal/schema/view"
	"github.com/urfave/cli/v2"
)

type QueryNoteResult struct {
	ID      shared.ID      `json:"id"`
	UID     string         `json:"uid,omitempty"`
	Path    string         `json:"path,omitempty"`
	Title   string         `json:"title"`
	Tags    []string       `json:"tags"`
	Domains []string       `json:"domains"`
	Links   []note.Link    `json:"links"`
	Content string         `json:"content"`
	AST     parser.ASTNode `json:"ast"`
}

type NoteService interface {
	SearchByTitle(ctx context.Context, q string) ([]note.Note, error)
	ListByTags(ctx context.Context, tags []string) ([]note.Note, error)
	ListByDomain(ctx context.Context, domain string) ([]note.Note, error)
	ListDomains(ctx context.Context) ([]string, error)
	List(ctx context.Context, limit, offset int) ([]note.Note, error)
	GetByID(ctx context.Context, id int) (*note.Note, error)
	GetByUID(ctx context.Context, uid string) (*note.Note, error)
}

func QueryDomains(c *cli.Context, svc NoteService) error {
	domains, err := svc.ListDomains(c.Context)
	if err != nil {
		return cli.Exit("❌ "+err.Error(), 1)
	}
	return writeJSON(domains)
}

func QueryNotes(c *cli.Context, svc NoteService) error {
	limit := c.Int("limit")
	offset := c.Int("offset")

	format := strings.ToLower(strings.TrimSpace(c.String("format")))
	if format == "" {
		format = "json"
	}
	viewName := strings.ToLower(strings.TrimSpace(c.String("view")))

	ctx := c.Context

	if q := strings.TrimSpace(c.String("search")); q != "" {
		notes, err := svc.SearchByTitle(ctx, q)
		if err != nil {
			return cli.Exit("❌ "+err.Error(), 1)
		}
		return writeJSON(notes)
	}

	if tagStr := strings.TrimSpace(c.String("tags")); tagStr != "" {
		tags := splitCSV(tagStr)
		notes, err := svc.ListByTags(ctx, tags)
		if err != nil {
			return cli.Exit("❌ "+err.Error(), 1)
		}
		return writeJSON(notes)
	}

	domain := strings.TrimSpace(c.String("domain"))
	category := strings.TrimSpace(c.String("category"))

	if category != "" {
		if domain == "" {
			domain = "glossary"
		}
		if !strings.EqualFold(domain, "glossary") {
			return cli.Exit("❌ --category is only supported for domain=glossary", 1)
		}
	}

	if domain != "" {
		notes, err := svc.ListByDomain(ctx, domain)
		if err != nil {
			return cli.Exit("❌ "+err.Error(), 1)
		}

		if category != "" {
			notes = filterGlossaryNotesByCategory(notes, category)
		}

		notes = paginateNotes(notes, limit, offset)

		return writeJSON(notes)
	}

	n, uid, ok, err := resolveSingleNote(ctx, c, svc)
	if err != nil {
		return cli.Exit("❌ "+err.Error(), 1)
	}

	if ok {
		if format == "text" {
			if viewName == "" {
				viewName = "pretty"
			}
			return renderNoteText(os.Stdout, *n, uid, viewName)
		}
		return writeJSON(makeQueryNoteResult(*n, uid))
	}

	rows, err := svc.List(ctx, limit, offset)
	if err != nil {
		return cli.Exit("❌ "+err.Error(), 1)
	}
	return writeJSON(rows)
}

func renderNoteText(w *os.File, n note.Note, uid string, viewName string) error {
	// Parse AST without meta block so your pretty output isn't polluted
	contentForAst := view.StripMeta(n.Content)
	ast := parser.ParseAST(contentForAst)

	switch viewName {
	case "commands":
		return writeString(w, renderDevToolCommandsText(n, uid, ast))
	case "pretty":
		fallthrough
	default:
		return writeString(w, renderPrettyText(n, uid, ast))
	}
}

func renderDevToolCommandsText(n note.Note, uid string, ast parser.ASTNode) string {
	var b strings.Builder

	title := n.Title
	if title == "" {
		title = "(untitled)"
	}

	b.WriteString(title)
	if uid != "" {
		b.WriteString("  [" + uid + "]")
	}
	b.WriteString("\n\n")

	commands := findHeading(ast, 1, "Commands")
	if commands == nil {
		b.WriteString("No Commands section.\n")
		return strings.TrimRight(b.String(), "\n")
	}

	// Each level-2 heading under Commands is a command entry
	for _, ch := range commands.Children {
		if ch.Type != "heading" || ch.Level != 2 {
			continue
		}

		cmdName := strings.TrimSpace(ch.Text)
		if cmdName == "" {
			continue
		}

		desc := findBulletValue(ch, "description")
		tpl := findBulletValue(ch, "template")
		notes := findBulletValue(ch, "notes")

		b.WriteString("• " + cmdName + "\n")
		if tpl != "" {
			b.WriteString("  " + tpl + "\n")
		}
		if desc != "" {
			b.WriteString("  - " + desc + "\n")
		}
		if notes != "" {
			b.WriteString("  - notes: " + notes + "\n")
		}

		// show up to 2 examples if they exist (lightweight heuristic)
		examples := extractExampleLines(ch, 2)
		for _, ex := range examples {
			b.WriteString("  - ex: " + ex + "\n")
		}

		b.WriteString("\n")
	}

	return strings.TrimRight(b.String(), "\n")
}

// Find a heading by level + exact text
func findHeading(node parser.ASTNode, level int, text string) *parser.ASTNode {
	if node.Type == "heading" && node.Level == level && strings.TrimSpace(node.Text) == text {
		return &node
	}
	for _, ch := range node.Children {
		if found := findHeading(ch, level, text); found != nil {
			return found
		}
	}
	return nil
}

// looks for bullet like "template: xxx" and returns "xxx"
func findBulletValue(node parser.ASTNode, key string) string {
	prefix := key + ":"
	for _, ch := range node.Children {
		if ch.Type != "bullet" {
			continue
		}
		txt := strings.TrimSpace(ch.Text)
		if strings.HasPrefix(txt, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(txt, prefix))
		}
	}
	return ""
}

// extremely simple: find paragraph lines that look like `{ example: "..."`
func extractExampleLines(node parser.ASTNode, max int) []string {
	out := []string{}
	for _, ch := range node.Children {
		if ch.Type == "paragraph" {
			t := strings.TrimSpace(ch.Text)
			if strings.HasPrefix(t, "{ example:") {
				out = append(out, t)
				if len(out) >= max {
					return out
				}
			}
		}
		// also traverse grandchildren (because your examples live under bullet "examples: [")
		for _, g := range ch.Children {
			if g.Type == "paragraph" {
				t := strings.TrimSpace(g.Text)
				if strings.HasPrefix(t, "{ example:") {
					out = append(out, t)
					if len(out) >= max {
						return out
					}
				}
			}
		}
	}
	return out
}

func renderPrettyText(n note.Note, uid string, ast parser.ASTNode) string {
	var b strings.Builder

	title := n.Title
	if title == "" {
		title = "(untitled)"
	}

	b.WriteString(title)
	if uid != "" {
		b.WriteString("  [" + uid + "]")
	}
	b.WriteString("\n")
	if n.Path != "" {
		b.WriteString(n.Path + "\n")
	}
	b.WriteString("\n")

	renderASTPretty(&b, ast, 0)
	return strings.TrimRight(b.String(), "\n")
}

func renderASTPretty(b *strings.Builder, node parser.ASTNode, indent int) {
	switch node.Type {
	case "root":
		for _, ch := range node.Children {
			renderASTPretty(b, ch, indent)
		}
	case "heading":
		// blank line before headings (except at top)
		if b.Len() > 0 && !strings.HasSuffix(b.String(), "\n\n") {
			b.WriteString("\n")
		}
		prefix := strings.Repeat("#", node.Level) // display heading depth as Markdown
		b.WriteString(prefix + " " + strings.TrimSpace(node.Text) + "\n")
		for _, ch := range node.Children {
			renderASTPretty(b, ch, indent+2)
		}
	case "bullet":
		b.WriteString(strings.Repeat(" ", indent) + "- " + strings.TrimSpace(node.Text) + "\n")
	case "paragraph":
		txt := strings.TrimSpace(node.Text)
		if txt == "" {
			return
		}
		b.WriteString(strings.Repeat(" ", indent) + txt + "\n")
	default:
		// ignore unknown nodes
	}
}
func writeString(w *os.File, s string) error {
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	_, err := w.WriteString(s)
	return err
}

func resolveSingleNote(ctx context.Context, c *cli.Context, svc NoteService) (*note.Note, string, bool, error) {
	if uid := strings.TrimSpace(c.String("uid")); uid != "" {
		n, err := svc.GetByUID(ctx, uid)
		if err != nil {
			return nil, "", false, err
		}
		return n, uid, true, nil
	}

	if id := c.Int("id"); id != 0 {
		n, err := svc.GetByID(ctx, id)
		if err != nil {
			return nil, "", false, err
		}
		return n, "", true, nil
	}

	return nil, "", false, nil
}
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func filterGlossaryNotesByCategory(notes []note.Note, wantedCategory string) []note.Note {
	wantedCategory = strings.ToLower(strings.TrimSpace(wantedCategory))
	if wantedCategory == "" {
		return notes
	}

	filtered := make([]note.Note, 0, len(notes))
	for _, n := range notes {
		if category, ok := glossaryCategoryFromPath(n.Path); ok && strings.EqualFold(category, wantedCategory) {
			filtered = append(filtered, n)
		}
	}

	return filtered
}

func glossaryCategoryFromPath(notePath string) (string, bool) {
	normalized := path.Clean(strings.TrimSpace(strings.ReplaceAll(notePath, "\\", "/")))
	if normalized == "." || normalized == "/" || normalized == "" {
		return "", false
	}

	parent := path.Dir(normalized)
	if parent == "." || parent == "/" || parent == "" {
		return "", false
	}

	category := strings.TrimSpace(path.Base(parent))
	if category == "." || category == "/" || category == "" {
		return "", false
	}

	return category, true
}

func paginateNotes(notes []note.Note, limit, offset int) []note.Note {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(notes) {
		return []note.Note{}
	}

	if limit <= 0 {
		limit = len(notes)
	}

	end := offset + limit
	if end > len(notes) {
		end = len(notes)
	}

	return notes[offset:end]
}

func makeQueryNoteResult(n note.Note, uid string) QueryNoteResult {
	return QueryNoteResult{
		ID:      n.ID,
		UID:     uid,
		Path:    n.Path,
		Title:   n.Title,
		Tags:    ensureStringSlice(n.Tags),
		Domains: ensureStringSlice(n.Domains),
		Links:   ensureLinkSlice(n.Links),
		Content: n.Content,
		AST:     parser.ParseAST(n.Content),
	}
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func ensureStringSlice(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func ensureLinkSlice(s []note.Link) []note.Link {
	if s == nil {
		return []note.Link{}
	}
	return s
}
