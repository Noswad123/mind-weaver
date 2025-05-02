import chokidar from 'chokidar';
import { syncNote } from './syncNote';

export function startWatcher(notesPath: string) {
    chokidar
        .watch(notesPath, { persistent: true, ignoreInitial: false, ignored: /(^|[/\\])\../ })
        .on('add', syncNote)
        .on('change', syncNote);

    console.log(`👀 Watching notes in ${notesPath}...`);
}
