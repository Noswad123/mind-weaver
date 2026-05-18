package validation

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/core/note"
	"github.com/Noswad123/mind-weaver/internal/core/registry"
	mdregistry "github.com/Noswad123/mind-weaver/internal/features/registration"
	"github.com/Noswad123/mind-weaver/internal/schema/templates"
	"github.com/urfave/cli/v2"
)

func Run(ctx *cli.Context, notesDir string, docs note.DocumentLister) error {
	log.Println("🧪 Validating notes...")

	hasError, err := validateFilesystem(notesDir)
	if err != nil {
		return cli.Exit("❌ "+err.Error(), 1)
	}

	domain := strings.TrimSpace(ctx.String("domain"))
	if domain != "" {
		if docs == nil {
			return cli.Exit("❌ domain validation requires database services", 1)
		}

		domainHasError, err := validateDomain(ctx, docs, domain)
		if err != nil {
			return cli.Exit("❌ "+err.Error(), 1)
		}
		if domainHasError {
			hasError = true
		}
	}

	if err := SyncFixCache(ctx.Context, notesDir, nil); err != nil {
		log.Printf("⚠️ failed to refresh fix cache: %v", err)
	}

	return finishValidation(hasError)
}

func RunRegistry(ctx *cli.Context, notesDir string, reg registry.Reader) error {
	log.Println("🧪 Validating registry conflicts...")

	hasError, err := validateRegistry(ctx, reg)
	if err != nil {
		return cli.Exit("❌ "+err.Error(), 1)
	}

	if cacheErr := SyncFixCache(ctx.Context, notesDir, reg); cacheErr != nil {
		log.Printf("⚠️ failed to refresh fix cache: %v", cacheErr)
	}

	return finishValidation(hasError)
}

func validateFilesystem(notesDir string) (bool, error) {
	root := filepath.Clean(notesDir)

	res, err := mdregistry.Build(root)
	if err != nil {
		return false, fmt.Errorf("failed to scan notes on disk: %v", err)
	}

	hasError := false

	for _, rel := range res.MissingHub {
		hasError = true
		log.Printf("[ERROR] %s  %s\n", rel, "MISSING_HUB_ID")
	}

	for _, dup := range res.Duplicates {
		hasError = true
		for _, rel := range dup.Paths {
			log.Printf("[ERROR] %s  %s %s\n", rel, "DUPLICATE_ID", dup.ID)
		}
	}

	return hasError, nil
}

func validateRegistry(ctx *cli.Context, r registry.Reader) (bool, error) {
	conflicts, err := r.ListConflicts(ctx.Context)
	if err != nil {
		return false, fmt.Errorf("failed to query registry conflicts: %v", err)
	}

	hasError := false
	for _, conflict := range conflicts {
		sev := registrySeverity(conflict.Reason)
		if sev == "ERROR" {
			hasError = true
		}
		logRegistryConflict(sev, conflict.Path, conflict.Reason, conflict.UID)
	}

	return hasError, nil
}

func validateDomain(ctx *cli.Context, docs note.DocumentLister, domain string) (bool, error) {
	spec, err := loadDomainSpec(domain)
	if err != nil {
		return false, fmt.Errorf("unknown domain schema: %s", domain)
	}

	rows, err := docs.ListDocuments(ctx.Context)
	if err != nil {
		return false, fmt.Errorf("failed to query notes: %v", err)
	}

	violations, hasError := collectDomainViolations(rows, spec)
	if len(violations) > 0 {
		_ = writeJSON(violations)
	}
	return hasError, nil
}

func collectDomainViolations(
	rows []note.Document,
	spec *templates.DomainSpec,
) ([]templates.DomainViolation, bool) {
	violations := make([]templates.DomainViolation, 0)
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

	return violations, hasError
}

func logRegistryConflict(sev, path, reason string, uidPtr *string) {
	uid := ""
	if uidPtr != nil {
		uid = *uidPtr
	}
	log.Printf("[%s] %s  %s %s\n", sev, path, reason, uid)
}

func finishValidation(hasError bool) error {
	if hasError {
		return cli.Exit("❌ Validation failed", 1)
	}
	log.Println("✅ Validation passed.")
	return nil
}

func registrySeverity(reason string) string {
	switch reason {
	case "MISSING_HUB_ID":
		return "ERROR"
	case "DUPLICATE_ID":
		return "ERROR"
	default:
		return "WARN"
	}
}

func loadDomainSpec(domain string) (*templates.DomainSpec, error) {
	specBytes, err := readDomainSchemaBytes(domain)
	if err != nil {
		return nil, err
	}
	return templates.LoadDomainSpec(specBytes)
}

func readDomainSchemaBytes(domain string) ([]byte, error) {
	candidates := []string{
		domain + ".yaml",
		domain + ".yml",
		slugToLowerCamel(domain) + ".yaml",
		slugToLowerCamel(domain) + ".yml",
	}

	var lastErr error
	for _, name := range candidates {
		b, err := templates.FS.ReadFile(name)
		if err == nil {
			return b, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func slugToLowerCamel(s string) string {
	parts := strings.FieldsFunc(strings.TrimSpace(s), func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		p := parts[i]
		if p == "" {
			continue
		}
		out += strings.ToUpper(p[:1]) + p[1:]
	}
	return out
}

func writeJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
