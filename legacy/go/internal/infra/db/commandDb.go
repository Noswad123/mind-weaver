package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/features/notes/parser"
	_ "github.com/mattn/go-sqlite3"
)

type Tool struct {
	ID          int
	Name        string
	Description string
}

type Command struct {
	ID           int
	ToolID       int
	ToolName     string
	Section      string
	Context      string
	CommandStub  string
	Flags        string
	Description  string
	OptionalInfo string
}

type Example struct {
	ID        int
	CommandID int
	Example   string
	Notes     string
}

type CommandWithExamples struct {
	Command  Command
	Examples []Example
}

type CommandDb struct {
	conn *sql.DB
}

func NewCommandDb(dbPath, schemaPath string) (*CommandDb, error) {
	if dir := filepath.Dir(dbPath); dir != "." && strings.TrimSpace(dir) != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	db := &CommandDb{conn: conn}
	if err := db.createSchema(schemaPath); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *CommandDb) Close() error {
	return db.conn.Close()
}

func (db *CommandDb) createSchema(schemaPath string) error {
	schema, err := readSchema(schemaPath, EmbeddedCommandSchemaPath, embeddedCommandSchema)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	statements := strings.Split(schema, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.conn.Exec(stmt); err != nil {
			return fmt.Errorf("❌ failed to exec schema statement:\n%s\nError: %w", stmt, err)
		}
	}
	return nil
}

func (db *CommandDb) InsertParsedTool(t *parser.ToolYAML) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var toolID int64
	err = tx.QueryRow(`INSERT INTO tools (name, description) VALUES (?, ?) RETURNING id`, t.Name, t.Description).Scan(&toolID)
	if err != nil {
		return fmt.Errorf("insert tool: %w", err)
	}

	for _, cmd := range t.Cheats {
		var commandID int64
		err = tx.QueryRow(`
			INSERT INTO commands (tool_id, section, context, command_stub, flags, description, optional_info)
			VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			toolID, cmd.Section, cmd.Context, cmd.CommandStub, cmd.Flags, cmd.Description, cmd.OptionalInfo,
		).Scan(&commandID)
		if err != nil {
			return fmt.Errorf("insert command: %w", err)
		}

		for _, tag := range cmd.Tags {
			var tagID int64
			err = tx.QueryRow(`INSERT INTO tags (name) VALUES (?) ON CONFLICT(name) DO UPDATE SET name=excluded.name RETURNING id`, tag).Scan(&tagID)
			if err != nil {
				return fmt.Errorf("insert tag: %w", err)
			}
			_, err = tx.Exec(`INSERT INTO command_tags (command_id, tag_id) VALUES (?, ?)`, commandID, tagID)
			if err != nil {
				return fmt.Errorf("link command/tag: %w", err)
			}
		}

		for _, arg := range cmd.Args {
			_, err = tx.Exec(`
				INSERT INTO command_args (command_id, name, description, required)
				VALUES (?, ?, ?, ?)`, commandID, arg.Name, arg.Description, arg.Required,
			)
			if err != nil {
				return fmt.Errorf("insert arg: %w", err)
			}
		}

		for _, ex := range cmd.Examples {
			_, err = tx.Exec(`INSERT INTO examples (command_id, example, notes) VALUES (?, ?, ?)`, commandID, ex.Example, ex.Notes)
			if err != nil {
				return fmt.Errorf("insert example: %w", err)
			}
		}
	}

	return tx.Commit()
}

func (db *CommandDb) GetAllCommandsWithExamples() ([]CommandWithExamples, error) {
	rows, err := db.conn.Query(`
		SELECT
		c.id, c.tool_id, t.name AS tool_name,
		c.section, c.context, c.command_stub,
		c.flags, c.description, c.optional_info
		FROM commands c
		JOIN tools t ON c.tool_id = t.id
		ORDER BY c.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CommandWithExamples

	for rows.Next() {
		var c Command

		if err := rows.Scan(
			&c.ID,
			&c.ToolID,
			&c.ToolName,
			&c.Section,
			&c.Context,
			&c.CommandStub,
			&c.Flags,
			&c.Description,
			&c.OptionalInfo,
		); err != nil {
			continue
		}

		examples, err := db.getExamplesForCommand(c.ID)
		if err != nil {
			examples = []Example{}
		}

		results = append(results, CommandWithExamples{
			Command:  c,
			Examples: examples,
		})
	}

	return results, nil
}

func (db *CommandDb) getExamplesForCommand(commandID int) ([]Example, error) {
	rows, err := db.conn.Query(`
		SELECT id, command_id, example, notes
		FROM examples
		WHERE command_id = ?
		ORDER BY id
	`, commandID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var examples []Example
	for rows.Next() {
		var e Example
		if err := rows.Scan(&e.ID, &e.CommandID, &e.Example, &e.Notes); err != nil {
			continue
		}
		examples = append(examples, e)
	}
	return examples, nil
}
