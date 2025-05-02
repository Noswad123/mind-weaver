import Database from 'better-sqlite3';
import { json } from '@sveltejs/kit';

const db = new Database('./db/mind-weaver.db');

export async function GET({ params }) {
  const path = decodeURIComponent(params.path);
  const note = db.prepare('SELECT * FROM notes WHERE path = ?').get(path);

  if (!note) return new Response('Not found', { status: 404 });

  return json(note);
}