package templates

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/markdown"
)

type DomainViolation struct {
	Path   string   `json:"path"`
	Errors []string `json:"errors,omitempty"`
	Warns  []string `json:"warns,omitempty"`
}

func ValidateNote(path string, content string, spec *DomainSpec) *DomainViolation {
	v := &DomainViolation{Path: path}

	// note must declare this domain via meta.domains
	domains := readDomains(content)
	if !domainIncluded(domains, spec.Domain) {
		return nil // not a note of this domain
	}

	if strings.TrimSpace(spec.Meta.Rules.DomainsMustInclude) != "" && !domainIncluded(domains, spec.Meta.Rules.DomainsMustInclude) {
		v.Errors = append(v.Errors, "meta.domains must include "+strings.TrimSpace(spec.Meta.Rules.DomainsMustInclude))
	}
	if spec.Meta.Rules.PathSuffix != "" && !strings.HasSuffix(path, spec.Meta.Rules.PathSuffix) {
		v.Errors = append(v.Errors, "path must end with "+spec.Meta.Rules.PathSuffix)
	}
	// required meta fields
	for _, req := range spec.Meta.Required {
		switch req {
		case "id":
			id, ok := markdown.ReadMetaIDFromContent(content)
			if !ok || strings.TrimSpace(id) == "" {
				v.Errors = append(v.Errors, "missing meta.id")
			}
		case "domains":
			vals, ok := readMetaStringList(content, "domains")
			if !ok || len(vals) == 0 {
				v.Errors = append(v.Errors, "missing meta.domains")
			}
		case "concepts":
			concepts, ok := readMetaStringList(content, "concepts")
			if !ok || len(concepts) == 0 {
				v.Errors = append(v.Errors, "missing meta.concepts")
			} else {
				// style: uid (basic sanity; resolution happens in app/db)
				if spec.Meta.Rules.ConceptsStyle == "uid" {
					for _, c := range concepts {
						if strings.TrimSpace(c) == "" {
							v.Errors = append(v.Errors, "meta.concepts contains empty uid")
							break
						}
					}
				}
			}

		case "language":
			lang, ok := readMetaString(content, "language")
			if !ok || strings.TrimSpace(lang) == "" {
				v.Errors = append(v.Errors, "missing meta.language")
			} else {
				if spec.Meta.Rules.LanguageStyle != "" {
					if lang, ok := readMetaString(content, "language"); ok && strings.TrimSpace(lang) != "" {
						if spec.Meta.Rules.LanguageStyle == "slug" && !isSlug(lang) {
							v.Errors = append(v.Errors, "meta.language must be a slug (lowercase letters/numbers/hyphen)")
						}
					}
				}
			}
		default:
			raw, ok := readMetaRaw(content, req)
			if !ok || strings.TrimSpace(raw) == "" {
				v.Errors = append(v.Errors, "missing meta."+req)
			}
		}
	}

	sections := extractAllSections(content)

	// required_any
	if len(spec.Sections.RequiredAny) > 0 {
		found := false
		for _, s := range spec.Sections.RequiredAny {
			if sections[s] {
				found = true
				break
			}
		}
		if !found {
			v.Errors = append(v.Errors, "missing one of required sections: "+strings.Join(spec.Sections.RequiredAny, ", "))
		}
	}

	// required_all
	for _, s := range spec.Sections.RequiredAll {
		if !sections[s] {
			v.Errors = append(v.Errors, "missing required section: "+s)
		}
	}

	// recommended
	for _, s := range spec.Sections.Recommended {
		if !sections[s] {
			v.Warns = append(v.Warns, "missing recommended section: "+s)
		}
	}

	applyStructureRules(v, content, spec)

	if len(v.Errors) == 0 && len(v.Warns) == 0 {
		return nil
	}
	return v
}

func readDomains(content string) []string {
	out := []string{}
	seen := map[string]bool{}

	vals, ok := readMetaStringList(content, "domains")
	if !ok {
		return out
	}
	for _, d := range vals {
		d = strings.TrimSpace(d)
		if d == "" || seen[d] {
			continue
		}
		seen[d] = true
		out = append(out, d)
	}

	return out
}

func domainIncluded(domains []string, wanted string) bool {
	wanted = strings.TrimSpace(wanted)
	if wanted == "" {
		return false
	}
	for _, d := range domains {
		if strings.TrimSpace(d) == wanted {
			return true
		}
	}
	return false
}

func extractAllSections(content string) map[string]bool {
	sections := map[string]bool{}
	for _, ln := range strings.Split(content, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "#") {
			// This is a bit of a hack, but it handles both "## Todo" and "Todo"
			// by storing both the full line and just the heading name.
			_, name, ok := parseHeadingLine(t)
			if ok {
				sections[name] = true
			}
			sections[t] = true
		}
	}
	return sections
}

var (
	linkRe           = regexp.MustCompile(`https?://[^\s\}\]>]+`)
	codeRe           = regexp.MustCompile(`(?ms)^\s*@code(?:\s+([^\s]+))?\s*$.*?^\s*@end\s*$`)
	checkboxBulletRe = regexp.MustCompile(`^\s*-\s*\[[ xX]\]\s+.+$`)
)

// applyStructureRules enforces spec.Sections.Structure[<SectionName>] when that section exists.
func applyStructureRules(v *DomainViolation, content string, spec *DomainSpec) {
	for sectionName, rules := range spec.Sections.Structure {
		block, ok := extractTopLevelSectionBlock(content, sectionName)
		if !ok {
			continue
		}

		// min_bullets
		if rules.MinBullets != nil && *rules.MinBullets > 0 {
			min := *rules.MinBullets
			if countBullets(block) < min {
				v.Errors = append(v.Errors, sectionName+" must contain at least "+strconv.Itoa(min)+" bullet(s)")
			}
		}

		// min_links
		if rules.MinLinks != nil && *rules.MinLinks > 0 {
			min := *rules.MinLinks
			n := countLinks(block)
			if n < min {
				v.Errors = append(v.Errors, sectionName+" must contain at least "+strconv.Itoa(min)+" link(s)")
			}
		}

		// require_code_block
		if rules.RequireCodeBlock != nil && *rules.RequireCodeBlock {
			if !hasCodeBlock(block) {
				v.Errors = append(v.Errors, sectionName+" must contain at least one @code ... @end block")
			}
		}

		// code_block rules
		if rules.CodeBlock != nil {
			blocks := findCodeBlocks(block)

			// If require_code_block isn't set, only validate code rules when code exists
			if len(blocks) > 0 || (rules.RequireCodeBlock != nil && *rules.RequireCodeBlock) {

				// require_language_tag
				if rules.CodeBlock.RequireLanguageTag != nil && *rules.CodeBlock.RequireLanguageTag {
					for _, cb := range blocks {
						if strings.TrimSpace(cb.Language) == "" {
							v.Errors = append(v.Errors, sectionName+" contains a @code block missing a language tag (expected: @code <lang>)")
							break
						}
					}
				} else {
					// default_language_from_meta
					if rules.CodeBlock.DefaultLanguageFromMeta != nil && *rules.CodeBlock.DefaultLanguageFromMeta {
						lang, ok := readMetaString(content, "language")
						lang = strings.TrimSpace(lang)

						if !ok || lang == "" {
							// only complain if there is a code block missing lang
							for _, cb := range blocks {
								if strings.TrimSpace(cb.Language) == "" {
									v.Errors = append(v.Errors, sectionName+" has @code block(s) without language and meta.language is missing")
									break
								}
							}
						}
					}
				}

				// allowed_directives (only if you later support @something else)
				// Right now your regex only matches "@code", so this is future-proofing.
				// If you expand directives, store directive name in codeBlock and validate here.
			}
		}

		needChildren := len(rules.RequiredChildren) > 0 || rules.MinChildren != nil || rules.ChildRules != nil
		if !needChildren {
			continue
		}

		children := extractImmediateChildrenBlocks(block)

		if len(rules.RequiredChildren) > 0 {
			for _, childName := range rules.RequiredChildren {
				if strings.TrimSpace(childName) == "" {
					continue
				}
				if !containsChild(children, childName) {
					v.Errors = append(v.Errors, sectionName+" section missing required child: "+childName)
				}
			}
		}

		if rules.MinChildren != nil && *rules.MinChildren > 0 {
			min := *rules.MinChildren
			if len(children) < min {
				v.Errors = append(v.Errors, sectionName+" section must contain at least "+strconv.Itoa(min)+" child section(s) (** ... / ## ...)")
			}
		}

		if rules.ChildRules == nil {
			continue
		}

		if rules.ChildRules.MinBullets != nil && *rules.ChildRules.MinBullets > 0 {
			cmin := *rules.ChildRules.MinBullets
			for _, child := range children {
				if countBullets(child.Block) < cmin {
					v.Errors = append(v.Errors, sectionName+" child '"+child.Name+"' must contain at least "+strconv.Itoa(cmin)+" bullet(s)")
				}
			}
		}

		if rules.ChildRules.CheckboxBulletsOnly != nil && *rules.ChildRules.CheckboxBulletsOnly {
			for _, child := range children {
				if !childHasOnlyCheckboxBullets(child.Block) {
					v.Errors = append(v.Errors, sectionName+" child '"+child.Name+"' must use markdown checkboxes for bullets")
				}
			}
		}

		if len(rules.ChildRules.RequiredFields) > 0 || len(rules.ChildRules.RecommendedFields) > 0 {
			for _, child := range children {
				fields := parseChildFields(child.Block)

				for _, reqKey := range rules.ChildRules.RequiredFields {
					if strings.TrimSpace(reqKey) == "" {
						continue
					}
					if _, ok := fields[reqKey]; !ok {
						v.Errors = append(v.Errors, sectionName+" child '"+child.Name+"' missing required field: "+reqKey)
					}
				}

				for _, recKey := range rules.ChildRules.RecommendedFields {
					if strings.TrimSpace(recKey) == "" {
						continue
					}
					if _, ok := fields[recKey]; !ok {
						v.Warns = append(v.Warns, sectionName+" child '"+child.Name+"' missing recommended field: "+recKey)
					}
				}
			}
		}
	}
}

func countLinks(block string) int {
	return len(linkRe.FindAllStringIndex(block, -1))
}

/* -----------------------------
   Helpers
----------------------------- */

// extractTopLevelSectionBlock returns the text for a level-1 section ("* Name" or "# Name")
// up to (but not including) the next level-1 section.
func extractTopLevelSectionBlock(content, name string) (string, bool) {
	lines := strings.Split(content, "\n")

	start := -1
	for i, ln := range lines {
		t := strings.TrimSpace(ln)
		level, heading, ok := parseHeadingLine(t)
		if ok && level == 1 && heading == name {
			start = i
			break
		}
	}
	if start == -1 {
		return "", false
	}

	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		level, _, ok := parseHeadingLine(t)
		if ok && level == 1 {
			end = i
			break
		}
	}

	return strings.Join(lines[start:end], "\n"), true
}

// countBullets counts lines starting with "- " anywhere in the block (indented OK).
func countBullets(block string) int {
	n := 0
	for ln := range strings.SplitSeq(block, "\n") {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "- ") {
			n++
		}
	}
	return n
}

func hasCodeBlock(block string) bool {
	return codeRe.FindStringIndex(block) != nil
}

type codeBlock struct {
	Language string
	Raw      string
}

func findCodeBlocks(block string) []codeBlock {
	out := []codeBlock{}
	matches := codeRe.FindAllStringSubmatchIndex(block, -1)
	for _, m := range matches {
		// m[0],m[1] = full match; m[2],m[3] = group 1 (language) if present
		raw := block[m[0]:m[1]]
		lang := ""
		if len(m) >= 4 && m[2] != -1 && m[3] != -1 {
			lang = block[m[2]:m[3]]
		}
		out = append(out, codeBlock{Language: lang, Raw: raw})
	}
	return out
}

type childBlock struct {
	Name  string
	Block string // includes the level-2 heading and everything until the next level-2 heading or end of parent block
}

// extractImmediateChildrenBlocks splits a top-level section block into immediate level-2 child blocks
// for Markdown child headings such as "## Child".
func extractImmediateChildrenBlocks(sectionBlock string) []childBlock {
	lines := strings.Split(sectionBlock, "\n")
	children := []childBlock{}

	var curName string
	curStart := -1

	flush := func(end int) {
		if curStart == -1 || strings.TrimSpace(curName) == "" {
			return
		}
		children = append(children, childBlock{
			Name:  curName,
			Block: strings.Join(lines[curStart:end], "\n"),
		})
	}

	for i := range lines {
		t := strings.TrimSpace(lines[i])
		level, heading, ok := parseHeadingLine(t)
		if !ok || level != 2 {
			continue
		}

		flush(i)
		curName = heading
		curStart = i
	}

	flush(len(lines))
	return children
}

func parseHeadingLine(trimmed string) (int, string, bool) {
	if trimmed == "" {
		return 0, "", false
	}

	marker := byte(0)
	switch trimmed[0] {
	case '#':
		marker = '#'
	case '*':
		marker = '*'
	default:
		return 0, "", false
	}

	i := 0
	for i < len(trimmed) && trimmed[i] == marker {
		i++
	}
	if i == 0 || i >= len(trimmed) || trimmed[i] != ' ' {
		return 0, "", false
	}

	name := strings.TrimSpace(trimmed[i+1:])
	if name == "" {
		return 0, "", false
	}
	return i, name, true
}

func containsChild(children []childBlock, wanted string) bool {
	for _, child := range children {
		if child.Name == wanted {
			return true
		}
	}
	return false
}

func childHasOnlyCheckboxBullets(block string) bool {
	for ln := range strings.SplitSeq(block, "\n") {
		t := strings.TrimSpace(ln)
		if !strings.HasPrefix(t, "- ") {
			continue
		}
		if !checkboxBulletRe.MatchString(t) {
			return false
		}
	}
	return true
}

var slugRe = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func isSlug(s string) bool {
	return slugRe.MatchString(strings.TrimSpace(s))
}
