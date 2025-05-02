import fs from 'fs';
import path from 'path';
import { syncNote } from './syncNote';
import {envVars} from './loadEnv';
import { startWatcher } from './watcher';
import { db } from './db';

function reindexAllNotes(notesPath: string) {
  console.log('🧹 Wiping existing data...');
  db.exec(`
    DELETE FROM notes;
    DELETE FROM tags;
    DELETE FROM todos;
    DELETE FROM links;
  `);
  console.log('🔁 Reindexing all .norg files...');
  const files: string[] = [];

  function walk(dir: any) {
    for (const entry of fs.readdirSync(dir)) {
      const fullPath = path.join(dir, entry);
      const stat = fs.statSync(fullPath);

      if (stat.isDirectory()) {
        walk(fullPath);
      } else if (entry.endsWith('.norg')) {
        files.push(fullPath);
      }
    }
  }

  walk(notesPath);

  files.forEach((file)=>syncNote(notesPath, file));
  console.log(`✅ Reindexed ${files.length} files`);
}

const notesPath = envVars.NOTES_DIR;
// === Startup ===
if (process.argv.includes('--reindex')) {
  reindexAllNotes(notesPath);
} else {
  startWatcher(notesPath);
}
