package mwcli

import (
	"log"

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
