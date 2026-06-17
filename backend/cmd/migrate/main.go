package main

import (
	"database/sql"
	"log"
	"os"

	"github.com/frankford824/ZBT/backend/internal/db/migrations"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: migrate <up|down|status>")
	}

	databaseURL := os.Getenv("MIGRATION_DATABASE_URL")
	if databaseURL == "" {
		databaseURL = os.Getenv("DATABASE_URL")
	}
	if databaseURL == "" {
		log.Fatal("MIGRATION_DATABASE_URL or DATABASE_URL is required")
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := migrations.Run(db, os.Args[1]); err != nil {
		log.Fatal(err)
	}
}
