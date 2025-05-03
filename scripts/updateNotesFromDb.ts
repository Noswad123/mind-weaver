import { db } from './db';
import fs from 'fs';
import path from 'path';
import { updateNoteFromDb } from './updateNoteFromDb';
import {ensureIndex} from './ensureIndex';
import {syncNeorgWorkspaces} from './syncNeorgWorkspaces';

type Options = {
  id?: number;
  notePath?: string;
  all?: boolean;
  generateIndices?: boolean;
  configFilePath?: string;
  notesPath: string;
};

export function updateNotesFromDb({ id, notePath, all, notesPath, generateIndices: generateIndexes, configFilePath }: Options): void {
  if (all) {
    const rows = db.prepare('SELECT id FROM notes').all();
    if (rows.length === 0) {
      console.warn('⚠️ No notes found in the database.');
      return;
    }
    rows.forEach(row => updateNoteFromDb(row.id, notesPath));

  if (generateIndexes) {
      if(!configFilePath) {
        console.error('❌ Missing required option: --config-file <path>');
        return;
      }
    const subdirs = fs.readdirSync(notesPath, { withFileTypes: true })
      .filter(d => d.isDirectory())
      .map(d => path.join(notesPath, d.name));

    subdirs.forEach(dir => ensureIndex(dir, notesPath));
    syncNeorgWorkspaces(configFilePath, notesPath);
  }
    return;
  }

  if (id != null) {
    updateNoteFromDb(id, notesPath);
    return;
  }

  if (notePath) {
    const row = db.prepare('SELECT id FROM notes WHERE path = ?').get(path);
    if (!row) {
      console.error(`❌ No note found with path: ${path}`);
      process.exit(1);
    }
    updateNoteFromDb(row.id, notesPath);
    return;
  }

  console.error('❌ Missing required option: --id <noteId>, --path <relativePath>, or --update-all');
  process.exit(1);
}
