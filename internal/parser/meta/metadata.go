package meta

import (
	"encoding/json"
	"log"
	"regexp"
	"strings"
)

type Metadata struct {
	Tags []string `json:"tags"`
	// Extend with more fields as needed
}

func ExtractMetadata(content string) Metadata {
	metaRegex := regexp.MustCompile(`@meta\s+([\s\S]*?)@end`)
	matches := metaRegex.FindStringSubmatch(content)
	if matches == nil || len(matches) < 2 {
		return Metadata{}
	}

	metaBlock := strings.TrimSpace(matches[1])
	metaBlock = sanitizeMetaBlock(metaBlock)
	jsonText := "{" + metaBlock + "}"

	var meta Metadata
	err := json.Unmarshal([]byte(jsonText), &meta)
	if err != nil {
		log.Printf("⚠️  Failed to parse metadata: %v\n", err)
		return Metadata{}
	}

	return meta
}

// sanitizeMetaBlock converts key: value lines into "key": value format for JSON parsing.
func sanitizeMetaBlock(input string) string {
	lines := strings.Split(input, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		out = append(out, "\""+key+"\": "+val)
	}
	return strings.Join(out, ",\n")
}
