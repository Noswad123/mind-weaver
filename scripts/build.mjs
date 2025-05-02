import { build } from 'esbuild';

await build({
  entryPoints: ['scripts/start.ts'],
  bundle: true,
  platform: 'node',
  format: 'cjs', // or 'cjs' if you want CommonJS output
  outfile: 'dist/scripts/start.cjs',
  target: 'node20',
  sourcemap: true,
  logLevel: 'info',
  external: ['better-sqlite3']
});