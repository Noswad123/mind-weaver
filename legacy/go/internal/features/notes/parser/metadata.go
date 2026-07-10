package parser

import (
	"regexp"
	"strings"
)

type Metadata struct {
	Tags    []string
	Domains []string
	Raw     map[string]string
}

var (
	// Matches a block starting with --- and ending with --- at the top of the file
	frontMatterRe = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---`)
	// Matches YAML key: value pairs, handling optional quotes
	yamlKvRe = regexp.MustCompile(`(?m)^([a-zA-Z0-9_-]+)\s*:\s*(?:"([^"]*)"|'([^']*)'|([^\n#]+))`)
)

func ExtractMetadata(content string) Metadata {
	m := Metadata{Raw: map[string]string{}}

	// Find the front matter block
	match := frontMatterRe.FindStringSubmatch(content)
	if len(match) < 2 {
		return m
	}

	block := match[1]
	lines := strings.Split(block, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		kvMatch := yamlKvRe.FindStringSubmatch(line)
		if len(kvMatch) == 0 {
			continue
		}

		key := strings.TrimSpace(kvMatch[1])
		val := ""

		// Check capturing groups for quoted or unquoted values
		for i := 2; i <= 4; i++ {
			if kvMatch[i] != "" {
				val = strings.TrimSpace(kvMatch[i])
				break
			}
		}

		switch key {
		case "tags":
			m.Tags = parseListLikeValue(val)
		case "domains":
			m.Domains = parseListLikeValue(val)
		default:
			m.Raw[key] = val
		}
	}

	return m
}

func parseListLikeValue(val string) []string {
	val = strings.TrimSpace(val)
	if val == "" {
		return []string{}
	}

	if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
		inner := strings.TrimSpace(val[1 : len(val)-1])
		if inner == "" {
			return []string{}
		}
		parts := splitCSVLoose(inner)
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			item := strings.TrimSpace(strings.Trim(part, "\"'"))
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	}

	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(strings.Trim(part, "\"'"))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func splitCSVLoose(s string) []string {
	parts := []string{}
	var b strings.Builder
	inSingle := false
	inDouble := false

	flush := func() {
		piece := strings.TrimSpace(b.String())
		b.Reset()
		if piece != "" {
			parts = append(parts, piece)
		}
	}

	for _, r := range s {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
			b.WriteRune(r)
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
			b.WriteRune(r)
		case ',':
			if inSingle || inDouble {
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
