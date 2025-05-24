package helper

import (
	"strings"
)
func Deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func SplitAndTrim(input string) []string {
	raw := strings.Split(input, ",")
	var result []string
	for _, r := range raw {
		trimmed := strings.TrimSpace(r)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
