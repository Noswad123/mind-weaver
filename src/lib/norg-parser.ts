function resolveInternalLink(workspacePath: string): string {
  // Strip off leading $
  const match = workspacePath.match(/^\$?([^/]+)\/(.+)$/);
  if (!match) return '#'; // fallback if malformed

  const [_, workspaceKey, subpath] = match;
  const rootPath = workspaceMap[workspaceKey];

  if (!rootPath) return '#'; // unknown workspace

  // const fullPath = `${rootPath}/${subpath}.norg`;
  const webSafePath = `${workspaceKey}/${subpath}`; // for routing

  return `/view?file=${encodeURIComponent(webSafePath)}`;
}

export function norgToHtml(text: string): string {
  return text
    .split('\n')
    .map(line => {
      // Internal links: either order
      line = line.replace(
        /\{:\$([^}]+):}\[([^\]]+)]|\[([^\]]+)]\{:\$([^}]+):}/g,
        (_, path1, label1, label2, path2) => {
          const path = path1 || path2;
          const label = label1 || label2;
          const href = resolveInternalLink(path);
          return `<a href="${href}" class="internal-link">${label}</a>`;
        }
      );

      // External links: either order
      line = line.replace(
        /\{(https?:\/\/[^}]+)}\[([^\]]+)]|\[([^\]]+)]\{(https?:\/\/[^}]+)}/g,
        (_, url1, label1, label2, url2) => {
          const url = url1 || url2;
          const label = label1 || label2;
          return `<a href="${url}" target="_blank" rel="noopener noreferrer" class="external-link">${label}</a>`;
        }
      );

      if (/^\*{3} (.+)/.test(line)) return `<h3>${line.replace(/^\*{3} /, '')}</h3>`;
      if (/^\*{2} (.+)/.test(line)) return `<h2>${line.replace(/^\*{2} /, '')}</h2>`;
      if (/^\* (.+)/.test(line)) return `<h1>${line.replace(/^\* /, '')}</h1>`;
      if (/^- (.+)/.test(line)) return `<li>${line.replace(/^- /, '')}</li>`;

      return `<p>${line}</p>`;
    })
    .join('\n');
}
