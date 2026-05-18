package validation

import (
	"encoding/json"
	"os"

	"github.com/Noswad123/mind-weaver/internal/schema/templates"
	"github.com/Noswad123/mind-weaver/internal/infra/db"
	"github.com/urfave/cli/v2"
)

func ValidateDomainCommand(c *cli.Context, noteDb *db.NoteDb, domain string) error {
	data, err := templates.FS.ReadFile(domain + ".yaml")
	if err != nil {
		return cli.Exit("❌ unknown domain schema: "+domain, 1)
	}

	spec, err := templates.LoadDomainSpec(data)
	if err != nil {
		return cli.Exit("❌ invalid domain schema: "+err.Error(), 1)
	}

	rows, err := noteDb.ListNotesForDomainValidation()
	if err != nil {
		return cli.Exit("❌ failed to load notes: "+err.Error(), 1)
	}

	var violations []templates.DomainViolation
	hasError := false

	for _, n := range rows {
		v := templates.ValidateNote(n.Path, n.Content, spec)
		if v == nil {
			continue
		}
		if len(v.Errors) > 0 {
			hasError = true
		}
		violations = append(violations, *v)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(violations)

	if hasError {
		return cli.Exit("❌ domain validation failed", 1)
	}
	return nil
}
