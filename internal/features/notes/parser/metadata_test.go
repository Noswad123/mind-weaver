package parser

import (
	"reflect"
	"testing"
)

func TestExtractMetadata_ParsesDomainsAndTags(t *testing.T) {
	content := `---
domains: [glossary, biology]
tags: "['term:immunity', 'topic:biology']"
id: "Immunity"
---

# Immunity
`

	m := ExtractMetadata(content)

	if !reflect.DeepEqual(m.Domains, []string{"glossary", "biology"}) {
		t.Fatalf("unexpected domains: %#v", m.Domains)
	}

	if !reflect.DeepEqual(m.Tags, []string{"term:immunity", "topic:biology"}) {
		t.Fatalf("unexpected tags: %#v", m.Tags)
	}
}

func TestParseListLikeValue_SupportsBracketAndCSV(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "bracket list",
			input: "[glossary, biology]",
			want:  []string{"glossary", "biology"},
		},
		{
			name:  "quoted bracket list",
			input: "['tool:git', 'topic:vcs']",
			want:  []string{"tool:git", "topic:vcs"},
		},
		{
			name:  "csv list",
			input: "one, two, three",
			want:  []string{"one", "two", "three"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseListLikeValue(tt.input)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseListLikeValue(%q) = %#v; want %#v", tt.input, got, tt.want)
			}
		})
	}
}
