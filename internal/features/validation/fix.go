package validation

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Noswad123/mind-weaver/internal/core/registry"
	mdregistry "github.com/Noswad123/mind-weaver/internal/features/registration"
	"github.com/urfave/cli/v2"
)

const fixCacheSchemaVersion = "1"

type FixIssue struct {
	Path     string `json:"path"`
	UID      string `json:"uid,omitempty"`
	Reason   string `json:"reason"`
	Severity string `json:"severity"`
	Source   string `json:"source"`
}

type FixCache struct {
	SchemaVersion string     `json:"schema_version"`
	GeneratedAt   string     `json:"generated_at"`
	NotesDir      string     `json:"notes_dir"`
	Items         []FixIssue `json:"items"`
}

func Fix(ctx *cli.Context, notesDir string, reg registry.Reader) error {
	notesDir = filepath.Clean(notesDir)
	useCached := ctx.Bool("cached")
	outputJSON := ctx.Bool("json")
	includeWarn := ctx.Bool("all")
	noOpen := ctx.Bool("no-open")
	noFuzzy := ctx.Bool("no-fuzzy")

	var (
		payload FixCache
		err     error
	)

	if useCached {
		payload, err = loadFixCache(notesDir)
		if err != nil {
			return cli.Exit("❌ failed to read cached conflicts: "+err.Error(), 1)
		}
	} else {
		items, collectErr := CollectFixIssues(ctx.Context, notesDir, reg, includeWarn)
		if collectErr != nil {
			return cli.Exit("❌ failed to collect conflicts: "+collectErr.Error(), 1)
		}

		payload = FixCache{
			SchemaVersion: fixCacheSchemaVersion,
			GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
			NotesDir:      notesDir,
			Items:         items,
		}

		if _, cacheErr := writeFixCache(notesDir, payload); cacheErr != nil {
			log.Printf("⚠️ failed to write fix cache: %v", cacheErr)
		}
	}

	if outputJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			return cli.Exit("❌ failed to write JSON output: "+err.Error(), 1)
		}
		return nil
	}

	if len(payload.Items) == 0 {
		log.Println("✅ No conflicts to fix.")
		return nil
	}

	selected := payload.Items
	if !noFuzzy && isTTY(os.Stdin) && isTTY(os.Stdout) && hasExecutable("fzf") {
		picked, pickErr := pickIssuesWithFuzzyFinder(payload.Items)
		if pickErr != nil {
			return cli.Exit("❌ failed during fuzzy select: "+pickErr.Error(), 1)
		}
		if len(picked) == 0 {
			log.Println("ℹ️ No files selected.")
			return nil
		}
		selected = picked
	}

	if noOpen {
		for _, it := range selected {
			if it.UID != "" {
				fmt.Printf("[%s] %s (%s)\n", it.Reason, it.Path, it.UID)
				continue
			}
			fmt.Printf("[%s] %s\n", it.Reason, it.Path)
		}
		return nil
	}

	if !hasExecutable("nvim") {
		return cli.Exit("❌ nvim not found in PATH (use --no-open to print selected files)", 1)
	}

	if err := openIssuesInNvimQuickfix(notesDir, selected); err != nil {
		return cli.Exit("❌ failed to open nvim quickfix: "+err.Error(), 1)
	}

	return nil
}

func CollectFixIssues(ctx context.Context, notesDir string, reg registry.Reader, includeWarn bool) ([]FixIssue, error) {
	items := make([]FixIssue, 0)
	seen := map[string]bool{}

	add := func(it FixIssue) {
		it.Path = filepath.ToSlash(strings.TrimSpace(it.Path))
		it.UID = strings.TrimSpace(it.UID)
		it.Reason = strings.TrimSpace(it.Reason)
		it.Severity = strings.TrimSpace(it.Severity)
		it.Source = strings.TrimSpace(it.Source)

		if it.Path == "" || it.Reason == "" {
			return
		}

		key := it.Path + "\x00" + it.Reason
		if seen[key] {
			return
		}
		seen[key] = true
		items = append(items, it)
	}

	fsRes, err := mdregistry.Build(notesDir)
	if err != nil {
		return nil, err
	}

	for _, rel := range fsRes.MissingHub {
		add(FixIssue{
			Path:     rel,
			Reason:   "MISSING_HUB_ID",
			Severity: "ERROR",
			Source:   "filesystem",
		})
	}

	for _, dup := range fsRes.Duplicates {
		for _, rel := range dup.Paths {
			add(FixIssue{
				Path:     rel,
				UID:      dup.ID,
				Reason:   "DUPLICATE_ID",
				Severity: "ERROR",
				Source:   "filesystem",
			})
		}
	}

	if reg != nil {
		conflicts, listErr := reg.ListConflicts(ctx)
		if listErr != nil {
			return nil, listErr
		}

		for _, c := range conflicts {
			sev := registrySeverity(c.Reason)
			if !includeWarn && sev != "ERROR" {
				continue
			}

			uid := ""
			if c.UID != nil {
				uid = *c.UID
			}

			add(FixIssue{
				Path:     c.Path,
				UID:      uid,
				Reason:   c.Reason,
				Severity: sev,
				Source:   "registry",
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Reason < items[j].Reason
		}
		return items[i].Path < items[j].Path
	})

	return items, nil
}

func SyncFixCache(ctx context.Context, notesDir string, reg registry.Reader) error {
	notesDir = filepath.Clean(notesDir)
	items, err := CollectFixIssues(ctx, notesDir, reg, false)
	if err != nil {
		return err
	}

	payload := FixCache{
		SchemaVersion: fixCacheSchemaVersion,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		NotesDir:      notesDir,
		Items:         items,
	}

	_, err = writeFixCache(notesDir, payload)
	return err
}

func fixCachePath(notesDir string) string {
	return filepath.Join(filepath.Clean(notesDir), ".mw", "cache", "notes-fix.json")
}

func writeFixCache(notesDir string, payload FixCache) (string, error) {
	cachePath := fixCachePath(notesDir)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return "", err
	}

	b, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(cachePath, b, 0o644); err != nil {
		return "", err
	}

	return cachePath, nil
}

func loadFixCache(notesDir string) (FixCache, error) {
	cachePath := fixCachePath(notesDir)
	b, err := os.ReadFile(cachePath)
	if err != nil {
		return FixCache{}, err
	}

	var payload FixCache
	if err := json.Unmarshal(b, &payload); err != nil {
		return FixCache{}, err
	}
	if payload.Items == nil {
		payload.Items = []FixIssue{}
	}
	if payload.NotesDir == "" {
		payload.NotesDir = filepath.Clean(notesDir)
	}

	return payload, nil
}

func pickIssuesWithFuzzyFinder(items []FixIssue) ([]FixIssue, error) {
	lines := make([]string, 0, len(items))
	for i, it := range items {
		line := strings.Join([]string{
			strconv.Itoa(i),
			it.Severity,
			it.Reason,
			it.UID,
			it.Path,
		}, "\t")
		lines = append(lines, line)
	}

	cmd := exec.Command("fzf", "--multi", "--delimiter", "\t", "--with-nth", "2,3,4,5", "--prompt", "mw notes fix> ")
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")

	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		return nil, err
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}

	selected := make([]FixIssue, 0)
	for _, line := range strings.Split(raw, "\n") {
		parts := strings.SplitN(line, "\t", 5)
		if len(parts) < 1 {
			continue
		}
		idx, convErr := strconv.Atoi(parts[0])
		if convErr != nil || idx < 0 || idx >= len(items) {
			continue
		}
		selected = append(selected, items[idx])
	}

	return selected, nil
}

func openIssuesInNvimQuickfix(notesDir string, items []FixIssue) error {
	tmp, err := os.CreateTemp("", "mw-notes-fix-*.qf")
	if err != nil {
		return err
	}
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}()

	for _, it := range items {
		abs := strings.TrimSpace(it.Path)
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(notesDir, filepath.FromSlash(abs))
		}

		msg := "[" + it.Reason + "]"
		if it.UID != "" {
			msg += " " + it.UID
		}
		if it.Source != "" {
			msg += " {" + it.Source + "}"
		}

		if _, err := fmt.Fprintf(tmp, "%s:1:1:%s\n", abs, msg); err != nil {
			return err
		}
	}

	if err := tmp.Sync(); err != nil {
		return err
	}

	cmd := exec.Command("nvim", "-q", tmp.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func hasExecutable(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func isTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
