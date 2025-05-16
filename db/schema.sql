CREATE TABLE IF NOT EXISTS notes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  path TEXT UNIQUE NOT NULL,        -- relative path (e.g., "philosophy/stoicism.norg")
  title TEXT,                       -- first heading or inferred title
  content TEXT,                     -- raw file content
  updated_at TEXT                   -- last sync timestamp
);

CREATE TABLE IF NOT EXISTS links (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  note_id INTEGER NOT NULL,         -- FK to notes.id (was source_path)
  label TEXT,                       -- the display text of the link (e.g., [Neovim])
  target TEXT,                      -- raw destination: "$neovim/index" or URL
  type TEXT CHECK(type IN ('internal', 'external')),
  resolved_path TEXT,               -- full resolved path (only for internal links)
  created_at TEXT DEFAULT CURRENT_TIMESTAMP,

  FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS tags (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  note_id INTEGER NOT NULL,         -- FK to notes.id (was note_path)
  tag TEXT,

  FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS task_groups (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  note_id INTEGER NOT NULL,         -- FK to notes.id
  name TEXT NOT NULL,               -- e.g., "Afterwork checklist"
  level INTEGER NOT NULL,           -- number of asterisks (1 = high, 3 = low)
  derived_group_id INTEGER,         -- FK to parent group
  status TEXT,
  raw_status TEXT,                  -- the symbol: x, !
  line_number INTEGER,

  FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE,
  FOREIGN KEY(derived_group_id) REFERENCES task_groups(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS todos (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  note_id INTEGER NOT NULL,         -- FK to notes.id
  task_group_id INTEGER NOT NULL,   -- FK to task_groups.id
  task TEXT NOT NULL,
  status TEXT NOT NULL,             -- "done", "todo", etc.
  raw_status TEXT NOT NULL,         -- the symbol: x, !
  depth INTEGER NOT NULL,
  line_number INTEGER,

  FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE,
  FOREIGN KEY(task_group_id) REFERENCES task_groups(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_links_target ON links(target);

DROP VIEW IF EXISTS group_subgroups;

CREATE VIEW group_subgroups AS
SELECT parent.id AS parent_id, child.id AS child_id
FROM task_groups parent
JOIN task_groups child ON child.derived_group_id = parent.id;

CREATE TABLE IF NOT EXISTS saved_queries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,       -- human-friendly name of the query
  sql TEXT NOT NULL                -- the SQL command to run
);

INSERT OR IGNORE INTO saved_queries (name, sql) VALUES
  ('All Notes', 'SELECT id, title FROM notes ORDER BY updated_at DESC'),
  ('Notes with Tags', 'SELECT n.id, n.title, GROUP_CONCAT(t.tag) AS tags FROM notes n JOIN tags t ON n.id = t.note_id GROUP BY n.id'),
  ('Notes with TODOs', 'SELECT n.id, n.title FROM notes n JOIN todos td ON n.id = td.note_id WHERE td.status != "done" GROUP BY n.id');
