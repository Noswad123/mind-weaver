package helper

import (
	"os"
	"log"

	"github.com/joho/godotenv"
	"path/filepath"
)

type Config struct {
	NotesDir   string
	DBPath     string
	SchemaPath string
	ConfigPath string
}

func LoadEnv() Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("❌ Could not get home directory")
	}
	envPath := filepath.Join(homeDir, "Projects","mind-weaver",".env")
	
	envLoaded := false
	if err := godotenv.Load(envPath); err == nil {
    	envLoaded = true
	}

	if !envLoaded {
		log.Fatal("❌ Could not load .env from current or fallback path")
	}

	notesDir := os.Getenv("NOTES_DIR")
	if notesDir == "" {
		log.Fatal("NOTES_DIR not set in .env file")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		log.Fatal("DB_PATH not set in .env file")
	}

	schemaPath := os.Getenv("SCHEMA_PATH")
	if schemaPath == "" {
		log.Fatal("SCHEMA_PATH not set in .env file")
	}

	configPath := os.Getenv("NEORG_CONFIG")
	if configPath == "" {
		log.Fatal("NEORG_CONFIG not set in .env file")
	}
	return Config {
		NotesDir: notesDir,
		DBPath: dbPath,
		SchemaPath: schemaPath,
		ConfigPath: configPath,
	}
}
