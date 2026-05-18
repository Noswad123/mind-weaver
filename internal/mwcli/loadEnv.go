package mwcli

import (
	mwconfig "github.com/Noswad123/mind-weaver/internal/config"
)

type config struct {
	loadErr            error
	appConfig          mwconfig.Config
	notesDir           string
	notesDBPath        string
	commandsDBPath     string
	notesSchemaPath    string
	commandsSchemaPath string
	inboxPath          string
	dashboardPath      string
}

func loadEnv() config {
	appCfg, err := mwconfig.Load()
	if err != nil {
		appCfg = mwconfig.Default()
	}

	return config{
		loadErr:            err,
		appConfig:          appCfg,
		notesDir:           appCfg.NotesDir,
		commandsDBPath:     appCfg.CommandsDBPath,
		notesDBPath:        appCfg.DBPath,
		notesSchemaPath:    appCfg.NotesSchemaPath,
		commandsSchemaPath: appCfg.CommandsSchemaPath,
		inboxPath:          appCfg.InboxPath,
		dashboardPath:      appCfg.DashboardPath,
	}
}
