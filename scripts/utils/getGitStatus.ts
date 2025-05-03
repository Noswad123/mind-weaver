import path from 'path';
import { execSync } from 'child_process';

export type GitStatus = 'clean' | 'modified' | 'staged' | 'untracked' | 'deleted' | 'unknown';

export function getGitStatus(filePath: string, gitRoot: string): GitStatus {
  try {
  if(!gitRoot) {
      console.warn(`⚠️ Git root not found for ${filePath}`);
      throw new Error('Git root not found');
    }
    const relativePath = path.relative('/Users/jdawson/Projects/darkness', filePath);
    /*
     * git status --porcelain returns 2-letter status codes:
        * M,M - modified
      * D,D: deleted
      * ??: untracted
    */
    // const result = execSync(`git status --porcelain "${filePath}"`, {
    const result = execSync(`git status --porcelain "${relativePath}"`, {
      encoding: 'utf-8',
      cwd: gitRoot,
    }).trim();

    if (result === '') return 'clean';

    const statusCode = result.slice(0, 2);

    if (statusCode === '??') return 'untracked';
    if (statusCode.includes('D')) return 'deleted';
    if (statusCode.includes('M')) return 'modified';
    if (['A ', ' M', 'AM', 'MM', ' M'].includes(statusCode)) return 'staged';

    return 'unknown';
  } catch (err) {
    console.warn(`⚠️ Git error for ${filePath}:`, err);
    return 'unknown';
  }
}
