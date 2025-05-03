import fs from 'fs';
import { getGitStatus, type GitStatus } from '../utils';

export type WriteValidator<T> = (arg: T) => boolean;

export function validateGitStatus(filePath: string, gitRoot: string): boolean{
  const fileExists = fs.existsSync(filePath);

  if (!fileExists) return true;

  const status: GitStatus = getGitStatus(filePath, gitRoot);
  switch (status) {
    case 'clean':
      return true;

    case 'deleted':
      console.warn(`⚠️ File is tracked as deleted: ${filePath}`);
      return false;

    case 'modified':
    case 'staged':
    case 'untracked':
      console.warn(`⛔ Skipping write. File has uncommitted changes (${status}): ${filePath}`);
      return false;

    default:
      console.warn(`❓ Unknown Git status. Skipping write: ${filePath}`);
      return false;
  }
}
