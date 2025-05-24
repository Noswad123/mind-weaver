-- Tools (e.g., git, docker)
CREATE TABLE tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT ''
);

-- Cheats (individual command definitions)
CREATE TABLE cheats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tool_id INTEGER NOT NULL,
    section TEXT DEFAULT '',
    context TEXT DEFAULT '',
    command_stub TEXT NOT NULL,
    flags TEXT DEFAULT '',
    description TEXT NOT NULL,
    optional_info TEXT DEFAULT '',
    FOREIGN KEY (tool_id) REFERENCES tools(id) ON DELETE CASCADE
);

-- Tags (many-to-many between cheats and tags)
CREATE TABLE tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE cheat_tags (
    cheat_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    PRIMARY KEY (cheat_id, tag_id),
    FOREIGN KEY (cheat_id) REFERENCES cheats(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

-- Args (named arguments or placeholders for commands)
CREATE TABLE cheat_args (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cheat_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    required BOOLEAN DEFAULT 0,
    FOREIGN KEY (cheat_id) REFERENCES cheats(id) ON DELETE CASCADE
);

-- Examples (command usage)
CREATE TABLE examples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    cheat_id INTEGER NOT NULL,
    example TEXT NOT NULL,
    notes TEXT DEFAULT '',
    FOREIGN KEY (cheat_id) REFERENCES cheats(id) ON DELETE CASCADE
);
