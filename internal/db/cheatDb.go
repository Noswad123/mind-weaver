
package db

import (
	"database/sql"
	"os"
	"strings"

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
	schemaBytes, err := os.ReadFile(schemaPath)
	if err != nil {
		return err
	}

	statements := strings.Split(string(schemaBytes), ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.conn.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}


func (db *CheatDb) LoadCheatsheets() ([]Cheat, error) {
	return nil, nil
}

