package markdown

import (
	"regexp"
)

var (
	// YAML frontmatter at the top of the file
	MetaBlockRe = regexp.MustCompile(`(?s)^---\s*\n(.*?)\n?---\s*(?:\n|$)`)
	// captures: id: 'value' | id: "value" | id: value | (legacy) id = value
	// also matches empty id declarations (e.g. id:, id: "", id: ''),
	// and allows trailing comments.
	MetaIDRe = regexp.MustCompile(`(?m)^[ \t]*id[ \t]*(?:=|:)[ \t]*(?:'([^']*)'|"([^"]*)"|([^\s#]+))?[ \t]*(?:#.*)?$`)

	// [[id]]
	WikiLinkRe = regexp.MustCompile(`\[\[([^\]]+)\]\]`)

	// {:note:}
	RelLinkRe = regexp.MustCompile(`\{\:([^}:]+)\:\}`)
)
