package templates

type DomainSpec struct {
	Domain  string `yaml:"domain"`
	Version string `yaml:"version"`

	Meta struct {
		Required []string  `yaml:"required"`
		Allowed  []string  `yaml:"allowed"`
		Rules    MetaRules `yaml:"rules"`
	} `yaml:"meta"`

	Sections struct {
		RequiredAny []string `yaml:"required_any"`
		RequiredAll []string `yaml:"required_all"`
		Recommended []string `yaml:"recommended"`
		Optional    []string `yaml:"optional"`

		Structure map[string]SectionRules `yaml:"structure"`
	} `yaml:"sections"`
}

type SectionRules struct {
	MinBullets      *int  `yaml:"min_bullets,omitempty"`
	AllowParagraphs *bool `yaml:"allow_paragraphs,omitempty"`

	RequireCodeBlock *bool `yaml:"require_code_block,omitempty"`
	AllowMultiple    *bool `yaml:"allow_multiple,omitempty"`

	MinLinks         *int            `yaml:"min_links,omitempty"`
	MinChildren      *int            `yaml:"min_children,omitempty"`
	RequiredChildren []string        `yaml:"required_children,omitempty"`
	ChildRules       *ChildRules     `yaml:"child_rules,omitempty"`
	CodeBlock        *CodeBlockRules `yaml:"code_block,omitempty"`
}

type ChildRules struct {
	MinBullets          *int                 `yaml:"min_bullets,omitempty"`
	AllowParagraphs     *bool                `yaml:"allow_paragraphs,omitempty"`
	MinLinks            *int                 `yaml:"min_links,omitempty"`
	CheckboxBulletsOnly *bool                `yaml:"checkbox_bullets_only,omitempty"`
	RequiredFields      []string             `yaml:"required_fields,omitempty"`
	RecommendedFields   []string             `yaml:"recommended_fields,omitempty"`
	Fields              map[string]FieldRule `yaml:"fields,omitempty"`
}

type FieldRule struct {
	Type     string `yaml:"type"`
	Required *bool  `yaml:"required,omitempty"`
}

type CodeBlockRules struct {
	RequireLanguageTag      *bool    `yaml:"require_language_tag,omitempty"`
	DefaultLanguageFromMeta *bool    `yaml:"default_language_from_meta,omitempty"`
	AllowedDirectives       []string `yaml:"allowed_directives,omitempty"`
}

type MetaRules struct {
	DomainsMustInclude  string `yaml:"domains_must_include,omitempty"`
	IDStyle             string `yaml:"id_style,omitempty"`
	LanguageStyle       string `yaml:"language_style,omitempty"`
	ConceptsStyle       string `yaml:"concepts_style,omitempty"`
	ConceptsMustResolve *bool  `yaml:"concepts_must_resolve,omitempty"`
	PathSuffix          string `yaml:"path_suffix,omitempty"`
}
