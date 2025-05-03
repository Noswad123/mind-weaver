import fs from 'fs';
import path from 'path';
import { db } from './db';

function toCamelCase(input: string): string {
  return input
    .replace(/[^a-zA-Z0-9]/g, ' ')
    .split(' ')
    .filter(Boolean)
    .map((word) => word[0].toUpperCase() + word.slice(1))
    .join('');
}

export function syncNeorgWorkspaces(configFilePath: string, notesRoot: string): void {
  const rows = db.prepare(`
    SELECT path FROM notes
    WHERE path LIKE '%/index.norg'
  `).all();

  const usedNames = new Set<string>();

  const workspaceEntries = rows
    .filter(r => r.path !== 'index.norg')
    .map(r => {
      const relativeDir = r.path.replace(/\/index\.norg$/, '');
      const segments = relativeDir.split('/');
      const rawName = toCamelCase(segments[segments.length - 1]);

      let name = rawName;
      let counter = 1;
      while (usedNames.has(name)) {
        name = `${rawName}${counter++}`;
      }
      usedNames.add(name);

      const fullPath = path.join(notesRoot, relativeDir);
      return `            ${name} = "${fullPath}",`;
    });

  const luaWorkspaceBlock = `workspaces = {\n${workspaceEntries.join('\n')}\n          },`;

  let config = fs.readFileSync(configFilePath, 'utf-8');
  config = config.replace(/workspaces = \{[\s\S]*?\},/, luaWorkspaceBlock);

  fs.writeFileSync(configFilePath, config);
  console.log('🔄 Synced Neorg workspaces.');
}
