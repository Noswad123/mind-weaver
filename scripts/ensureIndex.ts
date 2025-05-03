import fs from 'fs';
import path from 'path';
import { validateGitStatus } from './validators';

function insertTopics(content: string, dirPath: string): string {
  const topicsHeader = '* Topics';
  const topicIndex = content.indexOf(topicsHeader);
  if (topicIndex === -1) return content;

  const existingLinks = new Set(
    [...content.matchAll(/\*\* \{(:|\$:)([^}]+):\}/g)].map(m => m[2])
  );

  const items = fs.readdirSync(dirPath, { withFileTypes: true });

  const topicLinks = items
    .filter(item => {
      const name = item.name;
      return (
        (item.isDirectory() && !existingLinks.has(`${name}/index`)) ||
        (item.isFile() && name !== 'index.norg' && name.endsWith('.norg') && !existingLinks.has(name.replace(/\.norg$/, '')))
      );
    })
    .map(item => {
      if (item.isDirectory()) return `** {:$${item.name}/index:}`;
      if (item.isFile()) {
        const base = item.name.replace(/\.norg$/, '');
        return `** {:${base}:}`;
      }
      return null;
    })
    .filter(Boolean);

  // Insert after the Topics header
  const lines = content.split('\n');
  const newLines: string[] = [];
  let inserted = false;

  for (let i = 0; i < lines.length; i++) {
    newLines.push(lines[i]);

    if (!inserted && lines[i].trim() === topicsHeader) {
      newLines.push(...topicLinks);
      inserted = true;
    }
  }

  return newLines.join('\n');
}
export function ensureIndex(dirPath: string, notesRoot: string): void {
  const indexPath = path.join(dirPath, 'index.norg');
  const relative = path.relative(notesRoot, indexPath);

  if (relative === 'index.norg') return;

  let existing = '';
  if (fs.existsSync(indexPath)) {
    existing = fs.readFileSync(indexPath, 'utf-8');
  }

  let updated = existing;

  if (!/@meta[\s\S]*?@end/.test(existing)) {
    updated = `@meta\ntags: []\n@end\n\n` + updated;
  }

  const requiredHeaders = [
    '* Todo',
    '* Topics',
    '* Research',
    '* Resources',
  ];

  for (const header of requiredHeaders) {
    if (!updated.includes(header)) {
      updated += `\n${header}`;
    }
  }

  updated = insertTopics(updated, dirPath);

  if (!validateGitStatus(indexPath, notesRoot)) {
    return;
  };

  fs.writeFileSync(indexPath, updated.trim() + '\n');
  console.log(`📁 Ensured index.norg: ${relative}`);
}
