package mwcli

import (
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/features/todos"
	"github.com/urfave/cli/v2"
)

func buildTodosCommand(d deps) *cli.Command {
	return &cli.Command{
		Name:  "todos",
		Usage: "Do typical task things with notes",
		Subcommands: []*cli.Command{
			{
				Name:  "sync",
				Usage: "Sync task-index todos into dashboard",
				Action: d.action(func(c *cli.Context, d deps) error {
					return runTodosSync(d.cfg.notesDir, d.cfg.dashboardPath)
				}),
			},
			{
				Name:  "toggle",
				Usage: "Toggle a task-index todo by query id and refresh dashboard",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "id", Usage: "Todo id returned by query todos", Required: true},
				},
				Action: d.action(func(c *cli.Context, d deps) error {
					return runTodosToggle(d.cfg.notesDir, d.cfg.dashboardPath, c.String("id"))
				}),
			},
			{
				Name:  "inspect",
				Usage: "Inspect a source-backed task-index todo as JSON",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "id", Usage: "Todo id returned by query todos", Required: true},
				},
				Action: d.action(func(c *cli.Context, d deps) error {
					return runTodosInspect(d.cfg.notesDir, c.String("id"))
				}),
			},
			{
				Name:  "update",
				Usage: "Update task-index todo text or metadata and refresh dashboard",
				Flags: []cli.Flag{
					&cli.StringSliceFlag{Name: "id", Usage: "Todo id returned by query todos; repeat for bulk edits", Required: true},
					&cli.StringFlag{Name: "title", Usage: "Replace task text; single id only"},
					&cli.StringFlag{Name: "area", Usage: "Set todo area"},
					&cli.StringFlag{Name: "priority", Usage: "Set priority p1..p5"},
					&cli.StringFlag{Name: "energy", Usage: "Set energy xsm|s|m|l|xl"},
					&cli.StringFlag{Name: "weight", Usage: "Set explicit weight override"},
					&cli.StringFlag{Name: "due", Usage: "Set due date YYYY-MM-DD"},
					&cli.StringFlag{Name: "start", Usage: "Set start date YYYY-MM-DD"},
					&cli.StringFlag{Name: "estimate", Aliases: []string{"est"}, Usage: "Set estimate minutes"},
					&cli.StringFlag{Name: "metadata", Usage: "Replace metadata sub-bullet with raw metadata text"},
					&cli.StringSliceFlag{Name: "clear", Usage: "Clear metadata key: area,priority,energy,weight,due,start,estimate"},
				},
				Action: d.action(func(c *cli.Context, d deps) error {
					return runTodosUpdate(d.cfg.notesDir, d.cfg.dashboardPath, c)
				}),
			},
			{
				Name:  "archive",
				Usage: "Archive completed todos to life-log by week and area",
				Action: d.action(func(c *cli.Context, d deps) error {
					return runTodosArchive(d.cfg.notesDir, d.cfg.dashboardPath)
				}),
			},
		},
		Flags: flagsForInteractive,
		Action: d.action(func(c *cli.Context, d deps) error {
			interactive := c.Bool("interactive")
			changed, err := todos.ViewDashboard(interactive, d.cfg.notesDir, d.cfg.dashboardPath, d.cfg.inboxPath)
			if err != nil {
				return err
			}
			if interactive && changed {
				log.Printf("🔁 interactive dashboard changed, running todos sync")
				return runTodosSync(d.cfg.notesDir, d.cfg.dashboardPath)
			}
			return nil
		}),
	}
}

func runTodosSync(notesDir, dashboardPath string) error {
	stats, err := todos.SyncDashboardFromTaskIndexNotes(notesDir, dashboardPath)
	if err != nil {
		return err
	}
	log.Printf("✅ synced %d task(s) from %d active task-index note(s) (%d markdown note(s) scanned)", stats.SyncedTasks, stats.ActiveTaskIndexNotes, stats.ScannedMarkdownNotes)
	for _, group := range []string{"Code", "Action", "Reading", "Amusement", "Music", "Exercise", "Love"} {
		if n := stats.TasksByArea[group]; n > 0 {
			log.Printf("  • %s: %d", group, n)
		}
	}
	if stats.SourceWritebacks > 0 {
		log.Printf("↩️ applied %d completion update(s) back to %d source note(s)", stats.SourceWritebacks, stats.SourceFilesUpdated)
	}
	log.Printf("📝 dashboard updated: %s", dashboardPath)
	return nil
}

func runTodosToggle(notesDir, dashboardPath, todoID string) error {
	todo, done, err := todos.ToggleTaskIndexTodo(notesDir, dashboardPath, todoID)
	if err != nil {
		return err
	}

	state := "open"
	if done {
		state = "done"
	}
	log.Printf("✅ toggled todo %q to %s (%s:%d)", todo.Text, state, todo.SourcePath, todo.Line)
	return nil
}

func runTodosInspect(notesDir, todoID string) error {
	todo, err := todos.GetActiveTaskIndexTodo(notesDir, todoID)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(todo)
}

func runTodosUpdate(notesDir, dashboardPath string, c *cli.Context) error {
	params := todos.TodoUpdateParams{
		IDs:   c.StringSlice("id"),
		Clear: c.StringSlice("clear"),
	}
	if c.IsSet("title") {
		v := c.String("title")
		params.Title = &v
	}
	if c.IsSet("area") {
		v := c.String("area")
		params.Area = &v
	}
	if c.IsSet("priority") {
		v := c.String("priority")
		params.Priority = &v
	}
	if c.IsSet("energy") {
		v := c.String("energy")
		params.Energy = &v
	}
	if c.IsSet("weight") {
		v := c.String("weight")
		params.Weight = &v
	}
	if c.IsSet("due") {
		v := c.String("due")
		params.Due = &v
	}
	if c.IsSet("start") {
		v := c.String("start")
		params.Start = &v
	}
	if c.IsSet("estimate") {
		v := c.String("estimate")
		params.Estimate = &v
	}
	if c.IsSet("metadata") {
		v := c.String("metadata")
		params.Metadata = &v
	}

	updated, err := todos.UpdateTaskIndexTodos(notesDir, dashboardPath, params)
	if err != nil {
		return err
	}
	ids := []string{}
	for _, todo := range updated {
		ids = append(ids, todo.ID)
	}
	log.Printf("✅ updated %d todo(s): %s", len(updated), strings.Join(ids, ", "))
	return nil
}

func runTodosArchive(notesDir, dashboardPath string) error {
	log.Printf("🔁 syncing dashboard selections back to source notes before archive")
	if _, err := todos.SyncDashboardFromTaskIndexNotes(notesDir, dashboardPath); err != nil {
		return err
	}

	stats, err := todos.ArchiveCompletedToLifeLog(notesDir)
	if err != nil {
		return err
	}

	if stats.ArchivedTasks == 0 {
		log.Printf("📦 no completed todos found to archive")
		return nil
	}

	log.Printf("📦 archived %d completed task(s) from %d active task-index note(s)", stats.ArchivedTasks, stats.ActiveTaskIndexNotes)
	for _, group := range []string{"Code", "Action", "Reading", "Amusement", "Music", "Exercise", "Love"} {
		if n := stats.ArchivedByArea[group]; n > 0 {
			log.Printf("  • %s: %d", group, n)
		}
	}
	log.Printf("🗂 updated %d life-log month file(s), pruned %d source note(s)", stats.MonthFilesUpdated, stats.SourceFilesUpdated)
	log.Printf("🔁 refreshing dashboard projection after archive")
	return runTodosSync(notesDir, dashboardPath)
}
