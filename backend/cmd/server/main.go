package main

import (
	"context"
	"database/sql"
	"log"

	"github.com/frankford824/ZBT/backend/internal/api"
	"github.com/frankford824/ZBT/backend/internal/db/migrations"
	"github.com/frankford824/ZBT/backend/internal/platform/config"
	platformdb "github.com/frankford824/ZBT/backend/internal/platform/db"
	"github.com/frankford824/ZBT/backend/internal/platform/saas"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg := config.Load()
	ctx := context.Background()

	sqlDB, err := sql.Open("pgx", cfg.MigrationDatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer sqlDB.Close()
	if err := migrations.Up(sqlDB); err != nil {
		log.Fatal(err)
	}

	pool, err := platformdb.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	router := api.NewRouter(cfg, saas.NewStore(pool))

	log.Printf("zbt backend listening on %s", cfg.HTTPAddr)
	if err := router.Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}
