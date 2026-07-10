package formatter

func FormatHubNote(dir string, notesRoot string) error {
	_, _, err := applyHubUpdate(dir, notesRoot, true)
	return err
}
