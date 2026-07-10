package templates

import (
	"fmt"

	"io/fs"
	"gopkg.in/yaml.v3"
)

func LoadDomainSpec(data []byte) (*DomainSpec, error) {
	var spec DomainSpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse domain schema: %w", err)
	}

	if spec.Domain == "" {
		return nil, fmt.Errorf("domain schema missing 'domain'")
	}

	return &spec, nil
}

func LoadDomainSpecByDomain(domain string) (*DomainSpec, error) {
	matches, err := fs.Glob(FS, "*.yaml")
	if err != nil {
		return nil, err
	}

	for _, name := range matches {
		b, err := FS.ReadFile(name)
		if err != nil {
			continue
		}

		spec, err := LoadDomainSpec(b) // your existing []byte loader
		if err != nil {
			continue
		}

		if spec != nil && spec.Domain == domain {
			return spec, nil
		}
	}

	return nil, fmt.Errorf("unknown domain schema: %s", domain)
}
