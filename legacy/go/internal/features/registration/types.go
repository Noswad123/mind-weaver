package registration

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/core/notefiles"
	"github.com/Noswad123/mind-weaver/internal/markdown"
)

type Entry struct {
	ID   string `json:"id"`
	Path string `json:"path"`
}

type Registry struct {
	Entries map[string]Entry `json:"entries"`
}

type Duplicate struct {
	ID    string
	Paths []string
}

type BuildResult struct {
	Registry    Registry
	Duplicates  []Duplicate
	MissingHub  []string
	MissingMeta []string
}

type scannedNote struct {
	RelPath string
	IsHub   bool
	Content string
}

func Build(notesRoot string) (BuildResult, error) {
	root := filepath.Clean(notesRoot)
	rootBase := filepath.Base(root)

	res := BuildResult{
		Registry: Registry{Entries: map[string]Entry{}},
	}

	seen := map[string][]string{}

	err := walkFiles(root, func(note scannedNote) error {
		id, ok := extractID(note, &res, rootBase)

		if !ok {
			return nil
		}

		id = strings.TrimSpace(id)
		if id == "" {
			return nil
		}

		seen[id] = append(seen[id], note.RelPath)
		return nil
	})

	if err != nil {
		return BuildResult{}, err
	}

	dupSet := detectDuplicates(&res, seen)
	populateRegistry(&res, seen, dupSet)
	sortBuildResult(&res)

	return res, nil
}

func walkFiles(root string, fn func(n scannedNote) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.IsDir() && d.Name() == ".git" {
			return fs.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}

		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)

		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}

		n := scannedNote{
			RelPath: rel,
			IsHub:   notefiles.IsHubNotePath(path),
			Content: string(b),
		}

		return fn(n)
	})
}

// ----------------------------
// ID extraction rules
// ----------------------------

// extractID returns (id, okToRegister).
// okToRegister is false when we intentionally skip registering a note.
func extractID(n scannedNote, res *BuildResult, notesRootBase string) (string, bool) {
	id, hasMeta := markdown.ReadMetaIDFromContent(n.Content)
	id = strings.TrimSpace(id)

	if n.IsHub {
		// hub note defaults to the parent folder name when id is missing.
		if !hasMeta || id == "" {
			derived := deriveHubIDFromPath(n.RelPath, notesRootBase)
			if derived == "" {
				res.MissingHub = append(res.MissingHub, n.RelPath)
				return "", false
			}
			if !hasMeta {
				res.MissingMeta = append(res.MissingMeta, n.RelPath)
			}
			return derived, true
		}
		return id, true
	}

	// non-hub notes: default id to filename if missing
	if !hasMeta || id == "" {
		ext := filepath.Ext(n.RelPath)
		id = strings.TrimSuffix(filepath.Base(n.RelPath), ext)
		id = strings.TrimSpace(id)

		// if it didn't have meta at all, record that (optional)
		if !hasMeta {
			res.MissingMeta = append(res.MissingMeta, n.RelPath)
		}
	}

	return id, true
}

func deriveHubIDFromPath(relPath string, notesRootBase string) string {
	cleanRel := path.Clean(filepath.ToSlash(strings.TrimSpace(relPath)))
	if cleanRel == "." || cleanRel == "/" || cleanRel == "" {
		return ""
	}

	parent := strings.TrimSpace(path.Base(path.Dir(cleanRel)))
	if parent != "" && parent != "." && parent != "/" {
		return parent
	}

	root := strings.TrimSpace(notesRootBase)
	if root == "" || root == "." || root == "/" {
		return ""
	}
	return root
}

// ----------------------------
// duplicate detection + registry fill
// ----------------------------

func detectDuplicates(res *BuildResult, seen map[string][]string) map[string]bool {
	dupSet := map[string]bool{}

	for id, paths := range seen {
		if len(paths) <= 1 {
			continue
		}

		dupSet[id] = true
		sort.Strings(paths)

		res.Duplicates = append(res.Duplicates, Duplicate{
			ID:    id,
			Paths: paths,
		})
	}

	return dupSet
}

func populateRegistry(res *BuildResult, seen map[string][]string, dupSet map[string]bool) {
	for id, paths := range seen {
		if dupSet[id] {
			continue // skip duplicates entirely
		}
		if len(paths) == 0 {
			continue
		}
		res.Registry.Entries[id] = Entry{ID: id, Path: paths[0]}
	}
}

func sortBuildResult(res *BuildResult) {
	sort.Slice(res.Duplicates, func(i, j int) bool { return res.Duplicates[i].ID < res.Duplicates[j].ID })
	sort.Strings(res.MissingHub)
	sort.Strings(res.MissingMeta)
}
