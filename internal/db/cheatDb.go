package db

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/Noswad123/mind-weaver/internal/parser"
	_ "github.com/mattn/go-sqlite3"
)

type Cheat struct {
	Name string
	SQL  string
}

type CheatDb struct {
	conn *sql.DB
}

func NewCheatDb(dbPath, schemaPath string) (*CheatDb, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	db := &CheatDb{conn: conn}
	if err := db.createSchema(schemaPath); err != nil {
		return nil, err
	}

	return db, nil
}

func (db *CheatDb) Close() error {
	return db.conn.Close()
}

func (db *CheatDb) createSchema(schemaPath string) error {
	fmt.Println("Loading schema from:", schemaPath)
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file: %w", err)
	}

	schema := string(schemaBytes)
	fmt.Println("Loaded schema content:\n", schema)

	statements := strings.Split(schema, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		fmt.Printf("➡ Executing statement:\n%s\n", stmt)
		if _, err := db.conn.Exec(stmt); err != nil {
			return fmt.Errorf("❌ failed to exec schema statement:\n%s\nError: %w", stmt, err)
		}
	}
	return nil
}


func (db *CheatDb) LoadCheatsheets() ([]Cheat, error) {
	return nil, nil
}

func (db *CheatDb)InsertToolYAML(t *parser.ToolYAML) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Insert tool
	var toolID int64
	err = tx.QueryRow(`INSERT INTO tools (name, description) VALUES (?, ?) RETURNING id`, t.Name, t.Description).Scan(&toolID)
	if err != nil {
		return fmt.Errorf("insert tool: %w", err)
	}

	for _, cheat := range t.Cheats {
		// Insert cheat
		var cheatID int64
		err = tx.QueryRow(`
			INSERT INTO cheats (tool_id, section, context, command_stub, flags, description, optional_info)
			VALUES (?, ?, ?, ?, ?, ?, ?) RETURNING id`,
			toolID, cheat.Section, cheat.Context, cheat.CommandStub, cheat.Flags, cheat.Description, cheat.OptionalInfo,
		).Scan(&cheatID)
		if err != nil {
			return fmt.Errorf("insert cheat: %w", err)
		}

		// Insert tags
		for _, tag := range cheat.Tags {
			var tagID int64
			err = tx.QueryRow(`INSERT INTO tags (name) VALUES (?) ON CONFLICT(name) DO UPDATE SET name=excluded.name RETURNING id`, tag).Scan(&tagID)
			if err != nil {
				return fmt.Errorf("insert tag: %w", err)
			}
			_, err = tx.Exec(`INSERT INTO cheat_tags (cheat_id, tag_id) VALUES (?, ?)`, cheatID, tagID)
			if err != nil {
				return fmt.Errorf("link cheat/tag: %w", err)
			}
		}

		// Insert args
		for _, arg := range cheat.Args {
			_, err = tx.Exec(`
				INSERT INTO cheat_args (cheat_id, name, description, required)
				VALUES (?, ?, ?, ?)`, cheatID, arg.Name, arg.Description, arg.Required,
			)
			if err != nil {
				return fmt.Errorf("insert arg: %w", err)
			}
		}

		// Insert examples
		for _, ex := range cheat.Examples {
			_, err = tx.Exec(`INSERT INTO examples (cheat_id, example, notes) VALUES (?, ?, ?)`, cheatID, ex.Example, ex.Notes)
			if err != nil {
				return fmt.Errorf("insert example: %w", err)
			}
		}
	}

	return tx.Commit()
}
