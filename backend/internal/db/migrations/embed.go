package migrations

import (
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var fs embed.FS

func Up(db *sql.DB) error {
	return Run(db, "up")
}

func Run(db *sql.DB, command string) error {
	goose.SetBaseFS(fs)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Run(command, db, ".")
}
