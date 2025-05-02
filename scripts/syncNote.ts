import fs from 'fs';
import path from 'path';
import { db } from './db';
import { parseNorg } from './parseNorg';

function syncNote(notesPath: string, filePath: string) {
  const relativePath = path.relative(notesPath, filePath);
  const content = fs.readFileSync(filePath, 'utf-8');
  const { title, tags, todos, links } = parseNorg(content, relativePath);

  const tx = db.transaction(() => {
    // Upsert note
    db.prepare(`
      INSERT INTO notes (path, title, content, updated_at)
      VALUES (?, ?, ?, datetime('now'))
      ON CONFLICT(path) DO UPDATE SET
        title=excluded.title,
        content=excluded.content,
        updated_at=datetime('now')
    `).run(relativePath, title, content);

    // Get note_id
    const noteIdRow = db.prepare(`SELECT id FROM notes WHERE path = ?`).get(relativePath);
    if (!noteIdRow) {
      throw new Error(`❌ Failed to fetch note ID for ${relativePath}`);
    }
    const noteId = noteIdRow.id as number;

    // Tags
    db.prepare(`DELETE FROM tags WHERE note_id = ?`).run(noteId);
    const insertTag = db.prepare(`INSERT INTO tags (note_id, tag) VALUES (?, ?)`);
    tags.forEach(tag => insertTag.run(noteId, tag));

    // Delete old todos and task groups
    db.prepare(`DELETE FROM todos WHERE note_id = ?`).run(noteId);
    db.prepare(`DELETE FROM task_groups WHERE note_id = ?`).run(noteId);

    // Prepare insert statements
    const insertGroup = db.prepare(`
      INSERT INTO task_groups (note_id, name, level, derived_group_id, status, raw_status, line_number)
      VALUES (?, ?, ?, ?, ?, ?, ?)
    `);

    const insertTodo = db.prepare(`
      INSERT INTO todos (note_id, task_group_id, task, status, raw_status, depth, line_number)
      VALUES (?, ?, ?, ?, ?, ?, ?)
    `);

    // Track inserted groups
    const groupMap = new Map<string, number>();

    // Insert groups
    todos.filter(t => t.isGroup).forEach(group => {
      const parentKey = group.derivedGroup ? `${group.derivedGroup}:${group.level - 1}` : null;
      const derivedGroupId = parentKey ? groupMap.get(parentKey) ?? null : null;

      const groupKey = `${group.group}:${group.level}`;
      const result = insertGroup.run(noteId, group.group, group.level, derivedGroupId, group.status, group.rawStatus, group.line);
      const groupId = result.lastInsertRowid as number;
      groupMap.set(groupKey, groupId);
    });

    // Insert todos
    todos.filter(t => !t.isGroup).forEach(todo => {
      const groupKey = `${todo.group}:${todo.level}`;
      const groupId = groupMap.get(groupKey);
      if (groupId == null) {
        console.warn(`⚠️ No group ID found for task "${todo.task}" (group: ${todo.group})`);
        return;
      }

      insertTodo.run(
        noteId,
        groupId,
        todo.task,
        todo.status,
        todo.rawStatus,
        todo.depth,
        todo.line
      );
    });

    // Links
    db.prepare(`DELETE FROM links WHERE note_id = ?`).run(noteId);
    const insertLink = db.prepare(`
      INSERT INTO links (note_id, label, target, type, resolved_path)
      VALUES (?, ?, ?, ?, ?)
    `);
    links.forEach(l => insertLink.run(noteId, l.label, l.target, l.type, l.resolvedPath));
  });

  tx();
  console.log(`✅ Synced: ${relativePath}`);
}

export { syncNote };
