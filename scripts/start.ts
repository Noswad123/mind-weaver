import { envVars } from './loadEnv';
import { startWatcher } from './watcher';
import { reindexAllNotes } from './reindexAllNotes';
import { updateNotesFromDb } from './updateNotesFromDb';

const notesPath = envVars.NOTES_DIR;
const args = process.argv.slice(2);
const configFilePath = envVars.NEORG_CONFIG;

if (args.includes('--reindex')) {
  reindexAllNotes(notesPath);

} else if (args.includes('--update-all')) {
  const generateIndices = args.includes('--generate-indices')
  updateNotesFromDb({ generateIndices, all: true, notesPath, configFilePath });

} else if (args.includes('--update')) {
  const idIndex = args.indexOf('--id');
  const pathIndex = args.indexOf('--path');

  const noteId = idIndex !== -1 ? parseInt(args[idIndex + 1], 10) : undefined;
  const notePath = pathIndex !== -1 ? args[pathIndex + 1] : undefined;

  updateNotesFromDb({ id: noteId, notePath: notePath, notesPath });

} else {
  startWatcher(notesPath);
}
