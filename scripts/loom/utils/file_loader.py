import os
from config import NOTES_DIR

class FileLoader:
    def __init__(self, notes_root=NOTES_DIR):
        self.notes_root = notes_root

    def load_note_content(self, relative_path):
        full_path = os.path.join(self.notes_root, relative_path)
        try:
            with open(full_path, 'r') as f:
                return f"# {full_path}\n\n" + f.read()
        except Exception as e:
            return f"Error loading {full_path}: {e}"
