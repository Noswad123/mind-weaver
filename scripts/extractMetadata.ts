import JSON5 from 'json5';

export function extractMetadata(content: string): Record<string, any> {
  const metaRegex = /@meta\s+([\s\S]*?)@end/;
  const match = content.match(metaRegex);
  if (!match) return {};

  const metaBlock = match[1].trim();

  try {
    // Wrap as an object and parse
    const parsed = JSON5.parse(`{${metaBlock}}`);
    return parsed;
  } catch (err) {
    console.warn('⚠️ Failed to parse metadata:', err);
    return {};
  }
}
