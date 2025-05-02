import { resolveInternalLink } from './resolveInternalLinks';
import { extractTodos } from './extractTodos';
import { extractMetadata } from './extractMetadata';
import path from 'path';

function parseNorg(content: string, filePath: string) {
  const title = path.basename(filePath, path.extname(filePath));

  const metadata = extractMetadata(content);
  const metadataTags = Array.isArray(metadata.tags) ? metadata.tags : [];

  const tags = [...metadataTags, ...content.matchAll(/:([a-zA-Z0-9_-]+):/g)].map(m => m[1]);
  const todos = extractTodos(content);

  const links = [];
  const internal = /\{\:([^\}]+)\:\}\[([^\]]+)]|\[([^\]]+)]\{\:([^\}]+)\:\}/g;
  const external = /\{(https?:\/\/[^\}]+)}\[([^\]]+)]|\[([^\]]+)]\{(https?:\/\/[^\}]+)}/g;

  let m;
  while ((m = internal.exec(content))) {
    const rawPath = m[1] || m[4];
    const label = m[2] || m[3];
    const resolvedPath = resolveInternalLink(rawPath);
    links.push({ type: 'internal', target: rawPath, label, resolvedPath });
  }
  while ((m = external.exec(content))) {
    const url = m[1] || m[4];
    const label = m[2] || m[3];
    links.push({ type: 'external', target: url, label, resolvedPath: null });
  }

  return { title, tags, todos, links, content };
}

export { parseNorg };
