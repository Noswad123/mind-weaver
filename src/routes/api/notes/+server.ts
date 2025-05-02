import { json } from '@sveltejs/kit';
import Database from 'better-sqlite3';

const db = new Database('./db/mind-weaver.db');

export function GET() {
  const notesWithPolarity = db.prepare(`
    SELECT
      n.path,
      n.title,
      COUNT(l.id) AS polarity
    FROM notes n
    LEFT JOIN links l ON l.source_path = n.path OR l.resolved_path = n.path
    GROUP BY n.path
  `).all();

  return json(notesWithPolarity);
}