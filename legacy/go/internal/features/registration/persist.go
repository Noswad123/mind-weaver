package registration

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func WriteToDisk(notesRoot string, reg Registry) error {
	outPath := filepath.Join(filepath.Clean(notesRoot), ".mw", "registry.json")
	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("mkdir .mw: %w", err)
	}

	b, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal registry: %w", err)
	}

	if err := os.WriteFile(outPath, b, 0644); err != nil {
		return fmt.Errorf("write registry: %w", err)
	}
	return nil
}
