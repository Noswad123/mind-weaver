package db

import (
	_ "embed"
	"os"
)

//go:embed schema.sql
var embeddedNoteSchema string

//go:embed command-schema.sql
var embeddedCommandSchema string

const EmbeddedSchemaPath = "embedded://mind-weaver/schema.sql"
const EmbeddedCommandSchemaPath = "embedded://mind-weaver/command-schema.sql"

func readSchema(schemaPath, embeddedPath, embedded string) (string, error) {
	if schemaPath == "" || schemaPath == embeddedPath {
		return embedded, nil
	}

	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return "", err
	}
	return string(schemaBytes), nil
}
