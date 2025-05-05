import sqlite3
from config import DB_PATH

class NoteRepository:
    def __init__(self, db_path=DB_PATH):
        self.db_path = db_path

    def load_notes(self):
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        notes = {
            row['id']: {'path': row['path'], 'title': row['title'] or row['path']}
            for row in conn.execute("SELECT id, path, title FROM notes")
        }
        return notes

    def load_links(self):
        conn = sqlite3.connect(self.db_path)
        conn.row_factory = sqlite3.Row
        links = [
            (row['note_id'], row['resolved_path'])
            for row in conn.execute("SELECT note_id, resolved_path FROM links WHERE type='internal'")
        ]
        return links
