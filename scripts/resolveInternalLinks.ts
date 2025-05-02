import path from 'path';
import  { envVars, workspaceMap } from './loadEnv';

function resolveInternalLink(rawPath: string) {
  const match = rawPath.match(/^(\$?)([^\/]+)\/(.+)$/);
  if (!match) return null;

  const [, dollar, base, subpath] = match;
  const normalized = subpath.endsWith('.norg') ? subpath : `${subpath}.norg`;

  if (dollar && workspaceMap[base]) {
    return path.resolve(workspaceMap[base], normalized);
  }

  return path.resolve(envVars.NOTES_DIR, `${base}/${normalized}`);
}

export { resolveInternalLink };
