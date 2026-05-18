CREATE TABLE IF NOT EXISTS notes (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  path TEXT UNIQUE NOT NULL,        -- relative path (e.g., "philosophy/stoicism.md")
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

CREATE TABLE IF NOT EXISTS note_domains (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  note_id INTEGER NOT NULL,
  domain TEXT NOT NULL,

  FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE,
  UNIQUE(note_id, domain)
);

CREATE INDEX IF NOT EXISTS idx_note_domains_domain ON note_domains(domain);

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

CREATE TABLE IF NOT EXISTS note_ids (
  note_id INTEGER NOT NULL UNIQUE,     -- FK to notes.id (one id per note)
  note_uid TEXT NOT NULL UNIQUE,       -- the "id" from @meta (or filename fallback for non-hub notes)
  path TEXT NOT NULL UNIQUE,           -- redundant but makes querying easy; mirrors notes.path
  is_hub INTEGER NOT NULL DEFAULT 0,   -- 1 if hub.md
  updated_at TEXT DEFAULT CURRENT_TIMESTAMP,

  FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_note_ids_uid ON note_ids(note_uid);

CREATE TABLE IF NOT EXISTS note_id_conflicts (
  note_id INTEGER,                     -- nullable for "missing hub id" if needed
  note_uid TEXT,                       -- the offending id (or NULL if missing)
  path TEXT NOT NULL,
  is_hub INTEGER NOT NULL DEFAULT 0,
  reason TEXT NOT NULL CHECK(reason IN ('DUPLICATE_ID', 'MISSING_HUB_ID', 'NOTE_NOT_IN_DB')),
  detected_at TEXT DEFAULT CURRENT_TIMESTAMP,

  FOREIGN KEY(note_id) REFERENCES notes(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_note_id_conflicts_uid ON note_id_conflicts(note_uid);
CREATE INDEX IF NOT EXISTS idx_note_id_conflicts_reason ON note_id_conflicts(reason);

CREATE TABLE IF NOT EXISTS sync_outbox (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  op_id TEXT NOT NULL UNIQUE,
  idempotency_key TEXT NOT NULL UNIQUE,
  entity_type TEXT NOT NULL CHECK(entity_type IN ('note', 'todo')),
  entity_key TEXT NOT NULL,
  op_type TEXT NOT NULL CHECK(op_type IN ('upsert', 'delete')),
  payload TEXT,
  payload_hash TEXT NOT NULL,
  base_version INTEGER NOT NULL DEFAULT 0,
  status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'acked', 'failed')),
  attempt_count INTEGER NOT NULL DEFAULT 0,
  last_error TEXT,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  acked_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_sync_outbox_status_created ON sync_outbox(status, created_at);
CREATE INDEX IF NOT EXISTS idx_sync_outbox_entity ON sync_outbox(entity_type, entity_key);

CREATE TABLE IF NOT EXISTS sync_state (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sync_conflicts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  entity_type TEXT NOT NULL CHECK(entity_type IN ('note', 'todo')),
  entity_key TEXT NOT NULL,
  local_payload TEXT,
  remote_payload TEXT,
  reason TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  resolved_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_sync_conflicts_entity ON sync_conflicts(entity_type, entity_key);

CREATE TABLE IF NOT EXISTS sync_entity_versions (
  entity_type TEXT NOT NULL CHECK(entity_type IN ('note', 'todo')),
  entity_key TEXT NOT NULL,
  version INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY(entity_type, entity_key)
);

CREATE INDEX IF NOT EXISTS idx_sync_entity_versions_key ON sync_entity_versions(entity_type, entity_key);

CREATE TABLE IF NOT EXISTS sync_todos (
  id TEXT PRIMARY KEY,
  source_id TEXT NOT NULL DEFAULT '',
  source_path TEXT,
  task_scope TEXT,
  task_area TEXT,
  todo_section TEXT NOT NULL DEFAULT 'Inbox',
  task_text TEXT NOT NULL DEFAULT '',
  is_done INTEGER NOT NULL DEFAULT 0,
  meta TEXT,
  task_order INTEGER NOT NULL DEFAULT 0,
  line_number INTEGER,
  updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
  payload TEXT NOT NULL DEFAULT '{}'
);

CREATE INDEX IF NOT EXISTS idx_sync_todos_updated ON sync_todos(updated_at DESC);
