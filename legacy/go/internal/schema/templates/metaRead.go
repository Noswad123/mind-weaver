package templates

import (
	"regexp"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/markdown"
)

var (
	// key = 'value' | "value" | bareValue | [..] or key: value
	metaKVRe = regexp.MustCompile(`(?m)^\s*([a-zA-Z_][a-zA-Z0-9_-]*)\s*(?:=|:)\s*(.+?)\s*$`)
)

func metaBlock(content string) (string, bool) {
	m := markdown.MetaBlockRe.FindStringSubmatch(content)
	if len(m) < 2 {
		return "", false
	}
	return m[1], true
}

func readMetaString(content, key string) (string, bool) {
	block, ok := metaBlock(content)
	if !ok {
		return "", false
	}
	for ln := range strings.SplitSeq(block, "\n") {
		line := strings.TrimSpace(ln)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		m := metaKVRe.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		k := strings.TrimSpace(m[1])
		if k != key {
			continue
		}
		val := strings.TrimSpace(m[2])
		val = stripInlineComment(val)
		val = strings.TrimSpace(val)
		val = trimQuotes(val)
		if val == "" {
			return "", false
		}
		return val, true
	}
	return "", false
}

func readMetaStringList(content, key string) ([]string, bool) {
	raw, ok := readMetaRaw(content, key)
	if !ok {
		return nil, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, false
	}

	// Expect: [a, b] | ['a','b'] | ["a","b"] | []
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		inner := strings.TrimSpace(raw[1 : len(raw)-1])
		if inner == "" {
			return []string{}, true
		}
		parts := splitCSVLoose(inner)
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(trimQuotes(strings.TrimSpace(p)))
			if p != "" {
				out = append(out, p)
			}
		}
		return out, true
	}

	// Fallback: treat as single string
	s := trimQuotes(raw)
	if s == "" {
		return nil, false
	}
	return []string{s}, true
}

func readMetaRaw(content, key string) (string, bool) {
	block, ok := metaBlock(content)
	if !ok {
		return "", false
	}
	for ln := range strings.SplitSeq(block, "\n") {
		line := strings.TrimSpace(ln)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		m := metaKVRe.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		k := strings.TrimSpace(m[1])
		if k != key {
			continue
		}
		val := strings.TrimSpace(stripInlineComment(m[2]))
		return val, true
	}
	return "", false
}

func stripInlineComment(s string) string {
	// crude but effective: strip trailing " #..." or " //..."
	// (won’t try to be quote-aware; your meta lines are simple)
	if i := strings.Index(s, " #"); i >= 0 {
		return s[:i]
	}
	if i := strings.Index(s, " //"); i >= 0 {
		return s[:i]
	}
	return s
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// splits a,b,'c,d',"e,f" loosely (no nested arrays)
func splitCSVLoose(s string) []string {
	parts := []string{}
	var b strings.Builder
	inS := false
	inD := false

	flush := func() {
		p := strings.TrimSpace(b.String())
		b.Reset()
		if p != "" {
			parts = append(parts, p)
		}
	}

	for _, r := range s {
		switch r {
		case '\'':
			if !inD {
				inS = !inS
			}
			b.WriteRune(r)
		case '"':
			if !inS {
				inD = !inD
			}
			b.WriteRune(r)
		case ',':
			if inS || inD {
				b.WriteRune(r)
			} else {
				flush()
			}
		default:
			b.WriteRune(r)
		}
	}
	flush()
	return parts
}

var childFieldRe = regexp.MustCompile(`^\s*-\s*([a-zA-Z_][a-zA-Z0-9_-]*)\s*:\s*(.+?)\s*$`)

func parseChildFields(childBlock string) map[string]string {
	fields := map[string]string{}

	for ln := range strings.SplitSeq(childBlock, "\n") {
		line := strings.TrimRight(ln, "\r")
		m := childFieldRe.FindStringSubmatch(line)
		if len(m) != 3 {
			continue
		}
		k := strings.TrimSpace(m[1])
		v := strings.TrimSpace(m[2])
		if k != "" && v != "" {
			fields[k] = v
		}
	}
	return fields
}
