package markdown

import (
	"fmt"
	"os"
	"strings"
)

func EnsureMetaID(content string, id string) string {
	return MetaBlockRe.ReplaceAllStringFunc(content, func(block string) string {
		if MetaIDRe.MatchString(block) {
			mm := MetaIDRe.FindStringSubmatch(block)

			hasValue := false
			if len(mm) >= 4 {
				for i := 1; i <= 3; i++ {
					if strings.TrimSpace(mm[i]) != "" {
						hasValue = true
						break
					}
				}
			}

			if hasValue {
				return block
			}

			lines := strings.Split(block, "\n")
			for i, ln := range lines {
				if strings.HasPrefix(strings.TrimSpace(ln), "id:") {
					lines[i] = fmt.Sprintf("id: %q", id)
					break
				}
			}
			return strings.Join(lines, "\n")
		}

		lines := strings.Split(block, "\n")
		out := []string{}
		inserted := false

		for i, ln := range lines {
			out = append(out, ln)
			if !inserted && i == 0 && strings.TrimSpace(ln) == "---" {
				out = append(out, fmt.Sprintf("id: %q", id))
				inserted = true
			}
		}

		return strings.Join(out, "\n")
	})
}

func HasMetaBlock(content string) bool {
	return MetaBlockRe.MatchString(content)
}

func EnsureMetaBlock(content string) string {
	if HasMetaBlock(content) {
		return content
	}
	// minimal meta block; you can expand later
	return "---\n---\n\n" + content
}

func EnsureMetaBlockAndID(content string, desiredID string) (string, bool) {
	orig := content
	content = EnsureMetaBlock(content)
	content = EnsureMetaID(content, desiredID)
	return content, content != orig
}

func ReadMetaIDFromContent(content string) (string, bool) {
	m := MetaBlockRe.FindStringSubmatch(content)
	if len(m) != 2 {
		return "", false
	}
	meta := m[1]

	mm := MetaIDRe.FindStringSubmatch(meta)
	if len(mm) == 0 {
		return "", true // meta exists, id missing
	}

	// one of the capture groups will be non-empty
	for i := 1; i <= 3; i++ {
		if strings.TrimSpace(mm[i]) != "" {
			return strings.TrimSpace(mm[i]), true
		}
	}
	return "", true
}

func ReadMetaID(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return ReadMetaIDFromContent(string(b))
}

func EnsureMetaIDFile(path string, desiredID string) (bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Errorf("read %s: %w", path, err)
	}

	updated, changed := EnsureMetaBlockAndID(string(b), desiredID)
	if !changed {
		return false, nil
	}

	if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}

	return true, nil
}

func ValidateMetaID(content string, desiredID string) (bool, string) {
	orig := strings.TrimSpace(content) + "\n"
	updated, _ := EnsureMetaBlockAndID(content, desiredID)
	updated = strings.TrimSpace(updated) + "\n"

	if updated == orig {
		return false, ""
	}
	return true, "note is missing frontmatter id (run `mw notes format --all` to fix)"
}
