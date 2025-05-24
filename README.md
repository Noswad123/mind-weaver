# MindWeaver

![Black Wizard](img/black-wiz.png)

## What is MindWeaver?

An app to display my notes on the web

### Astral Loom (Python visualizer)

- A visual representation of woven thoughtforms

### Grimoire (tool for viewing cmds for other tools)

### Scrye (interface to query my notes)

- maybe a sqlite/psql wrapper?

## Lore (documentation)

familiars = tags

### CLI incantations

mw --banish -> sync notes with database
mw --gaze -> monitor notes directory for changes
mw --engrave -> maintains index files
mw --summon -> fetch a note
mw loom -> start visuzlizer
mw transmute -> convert between norg and markdown
mw scrye -> start query interface
mw bind -> add tags to notes
mw attune -> config oriented spell
mw channel -> not sure
mw grimoire -> tool for viewing with (docker, git, tmux etc) cmds

## Development
- Steps I took to develop this app

## Go

### init

go mod init github.com/Noswad123/mind-weaver (this created my go.mod file)

### tidy 

go mod tidy (downloads external modules)

### Write some code

### Run

go run . --reindex --watch
go run . --ensure-indices

## makefile

- .PHONY, tells you what commands are available
- ie, make all will build and install the app
- then you will be able to run the cli via mw

## Python

- using mw visualize will run the visualizer made using PyQt

## TODO

Create a new sqlite schema for grimmoire (cheatsheets table, etc.)
Add a second helper.OpenGrimmoireDB() function
Switch logic in RunTUI based on mode
In TUI, load cheatsheets as list.Items and wire them to the viewport + textarea for viewing/editing.
A note metadata query tool (mode == "void")
spirits summoned from the void
A cheatsheet manager (mode == "grimmoire")
incantations/spells summoned from the grimmoire
SQL schema + Go parser for these additions?
A full YAML schema file you can document against?
A prototype function that validates and prompts for required args?
- separate incantations from spirits
- reviewed/reviewedAt metadata
- The ability to convert between norg and markdown
- an interface to query my notes
