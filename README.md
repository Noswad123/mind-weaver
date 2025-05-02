# MindWeaver

## What is MindWeaver?

An app to display my notes on the web

## TODO

- define exactly what you want this app to do
- Maybe break apart app into standalone repos

## Design

  Backend?
  norg -> html
  Svelte
  VPS

  npm install tree-sitter tree-sitter-norg

  1. Treat Files as the Source of Truth
	•	DB changes should be synced back to files immediately.
	•	File changes should trigger reindexing.

2. Use a File Watcher
	•	Tools like chokidar (Node), fswatch, or inotifywait can watch the notes directory for changes.
	•	On any .norg file change: parse → update DB

3. Round-trip Editing From the App
	•	If you edit a note via your app:
	•	Parse from DB → render in editor
	•	On save: write the .norg file → reparse and update DB
	•	This avoids needing complex conflict resolution—just treat the file as authoritative.

4. Git is Your Conflict Resolution
	•	If you edit both in Neovim and the app, Git becomes your “manual sync gate.”
	•	Add a modified_at timestamp in the DB to detect divergence if needed.

## svelte

Everything you need to build a Svelte project, powered by [`create-svelte`](https://github.com/sveltejs/kit/tree/main/packages/create-svelte).


## Developing

Once you've created a project and installed dependencies with `npm install` (or `pnpm install` or `yarn`), start a development server:

```bash
npm run dev

# or start the server and open the app in a new browser tab
npm run dev -- --open
```

## Building

To create a production version of your app:

```bash
npm run build
```

You can preview the production build with `npm run preview`.

> To deploy your app, you may need to install an [adapter](https://kit.svelte.dev/docs/adapters) for your target environment.
