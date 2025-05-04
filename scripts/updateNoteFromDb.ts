import fs from 'fs';
import path from 'path';
import { db } from './db';
import {validateGitStatus} from './validators';

export function updateNoteFromDb(noteId: number, notesPath: string) {
  const row = db.prepare(`SELECT path, title, content FROM notes WHERE id = ?`).get(noteId);
  if (!row) throw new Error(`Note with ID ${noteId} not found`);

  const fullPath = path.join(notesPath, row.path);
  console.log(notesPath)

  const originalContent = fs.readFileSync(fullPath, 'utf-8');

  let newContent = originalContent;

  // Example: inject @meta block if missing
  if (!/@meta\s+[\s\S]*?@end/.test(originalContent)) {
    const tags = db.prepare(`SELECT tag FROM tags WHERE note_id = ?`).all(noteId).map(r => r.tag);
    const metaBlock = `@meta\n  tags = ${JSON.stringify(tags)}\n@end\n\n`;
    newContent = metaBlock + originalContent;
  }

  if (!validateGitStatus(fullPath, notesPath)) {
    return;
  };

  fs.writeFileSync(fullPath, newContent);
  console.log(`📝 Updated note from DB: ${row.path}`);
}
