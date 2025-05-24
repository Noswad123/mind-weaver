package helper

import (
	"os"
	"log"

	"github.com/joho/godotenv"
	"path/filepath"
)

type Config struct {
	NotesDir   string
	NoteDBPath     string
	CheatDBPath     string
	NoteSchemaPath string
	CheatSchemaPath string
	ConfigPath string
	RunMode string
	PythonPath string
	LoomPath string
}

func LoadEnv() Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("❌ Could not get home directory")
	}
	mindWeaverPath := filepath.Join(homeDir, "Projects","mind-weaver")
	envPath := filepath.Join(homeDir, "Projects","mind-weaver",".env")
	
	if err := godotenv.Load(envPath); err != nil {
		log.Fatal("❌ Could not load .env from current or fallback path")
	}

	runMode := os.Getenv("RunMode")
	if runMode == "" {
		runMode = "dev"
		log.Println("Running in dev Mode by default")
	}

	loomPath := filepath.Join(mindWeaverPath, "scripts", "loom", "main.py")
	if runMode == "prod" {
		execPath, err := os.Executable()
		if err != nil {
			log.Fatal("Failed to find executable path")
		}

		execDir := filepath.Dir(execPath)
		loomPath = filepath.Join(execDir, "loom")
	}

	pythonPath := os.Getenv("PYTHON_PATH")
	if pythonPath == "" {
		pythonPath = "python3"
	}

	notesDir := os.Getenv("NOTES_DIR")
	if notesDir == "" {
		log.Fatal("NOTES_DIR not set in .env file")
	}

	noteDbPath := os.Getenv("NOTE_DB_PATH")
	cheatDbPath := os.Getenv("CHEAT_DB_PATH")

	schemaPath := os.Getenv("SCHEMA_PATH")
	if schemaPath == "" {
		log.Fatal("SCHEMA_PATH not set in .env file")
	}
	noteSchemaPath := filepath.Join(schemaPath, "schema.sql")
	cheatSchemaPath := filepath.Join(schemaPath, "cheat-schema.sql")

	configPath := os.Getenv("NEORG_CONFIG")
	if configPath == "" {
		log.Fatal("NEORG_CONFIG not set in .env file")
	}
	return Config {
		NotesDir: notesDir,
		CheatDBPath: cheatDbPath,
		NoteDBPath: noteDbPath,
		NoteSchemaPath: noteSchemaPath,
		CheatSchemaPath: cheatSchemaPath,
		ConfigPath: configPath,
		LoomPath: loomPath,
		RunMode: runMode,
		PythonPath: pythonPath,
	}
}
