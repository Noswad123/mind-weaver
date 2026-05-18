package parser

// TODO: This file needs work
import (
	"strings"
)

type ASTNode struct {
	Type     string     `json:"type"`               // "root" | "heading" | "bullet" | "paragraph" | "code"
	Level    int        `json:"level,omitempty"`    // heading only
	Text     string     `json:"text,omitempty"`     // heading/bullet/paragraph
	Lang     string     `json:"lang,omitempty"`     // code only
	Code     string     `json:"code,omitempty"`     // code only
	Children []ASTNode  `json:"children,omitempty"` // nested content
}

func ParseAST(content string) ASTNode {
	root := ASTNode{Type: "root", Children: []ASTNode{}}

	lines := strings.Split(content, "\n")

	// stack of pointers to children slices at each heading depth
	// stack[0] corresponds to root.Children
	stack := []*[]ASTNode{&root.Children}

	inCode := false
	codeLang := ""
	codeLines := []string{}

	flushCode := func() {
		if !inCode {
			return
		}
		node := ASTNode{
			Type: "code",
			Lang: strings.TrimSpace(codeLang),
			Code: strings.TrimRight(strings.Join(codeLines, "\n"), "\n"),
		}
		*stack[len(stack)-1] = append(*stack[len(stack)-1], node)
		inCode = false
		codeLang = ""
		codeLines = nil
	}

	appendParagraph := func(text string) {
		t := strings.TrimSpace(text)
		if t == "" {
			return
		}
		node := ASTNode{Type: "paragraph", Text: t}
		*stack[len(stack)-1] = append(*stack[len(stack)-1], node)
	}

	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trim := strings.TrimSpace(line)

		// Handle code blocks: "@code <lang>" ... "@end"
		if inCode {
			if trim == "@end" {
				flushCode()
			} else {
				codeLines = append(codeLines, raw) // preserve indentation
			}
			continue
		}

		// Start code
		if strings.HasPrefix(trim, "@code") {
			parts := strings.Fields(trim)
			lang := ""
			if len(parts) >= 2 {
				lang = parts[1]
			}
			inCode = true
			codeLang = lang
			codeLines = []string{}
			continue
		}

		firstBreak := 0
		// Ignore meta blocks in AST for now (you can add meta nodes later)
		if strings.HasPrefix(trim, "---") {
			firstBreak++
			for trim != "@end" && firstBreak > 1 { 
				break
			}
		}

		// Headings "# ", "## ", etc.
		if strings.HasPrefix(trim, "#") {
			// count leading '#'
			level := 0
			for level < len(trim) && trim[level] == '#' {
				level++
			}
			// must be "* " style
			if level > 0 && len(trim) > level && trim[level] == ' ' {
				title := strings.TrimSpace(trim[level:])
				// adjust stack to heading level (root is level 0)
				// stack length should be level+1 (root + levels)
				for len(stack) > level {
					stack = stack[:len(stack)-1]
				}
				for len(stack) < level {
					// if headings skip levels, we still create a level by reusing last
					stack = append(stack, stack[len(stack)-1])
				}

				node := ASTNode{Type: "heading", Level: level, Text: title, Children: []ASTNode{}}
				cur := stack[len(stack)-1]
				*cur = append(*cur, node)

				// push children slice of newly appended node
				lastIdx := len(*cur) - 1
				stack = append(stack, &(*cur)[lastIdx].Children)
				continue
			}
		}

		// Bullets "-" (you like these; we treat them as bullets, NOT arrays)
		if strings.HasPrefix(trim, "- ") {
			node := ASTNode{Type: "bullet", Text: strings.TrimSpace(strings.TrimPrefix(trim, "- "))}
			*stack[len(stack)-1] = append(*stack[len(stack)-1], node)
			continue
		}

		// Blank line: paragraph boundary (we keep paragraphs as single-line for now)
		if trim == "" {
			continue
		}

		// Default: paragraph line
		appendParagraph(raw)
	}

	flushCode()
	return root
}
