package meta

import (
	"encoding/json"
	"log"
	"regexp"
	"strings"
)

type Metadata struct {
	Tags []string `json:"tags"`
	// Add more fields as needed, or use map[string]any for full flexibility
}

func ExtractMetadata(content string) Metadata {
	metaRegex := regexp.MustCompile(`@meta\s+([\s\S]*?)@end`)
	matches := metaRegex.FindStringSubmatch(content)
	if matches == nil || len(matches) < 2 {
		return Metadata{}
	}

	metaBlock := strings.TrimSpace(matches[1])
	jsonText := "{" + metaBlock + "}"

	var meta Metadata
	err := json.Unmarshal([]byte(jsonText), &meta)
	if err != nil {
		log.Printf("⚠️ Failed to parse metadata: %v\n", err)
		return Metadata{}
	}

	return meta
}

