package main

import (
	"context"
	"database/sql"
	"log"

	"github.com/frankford824/ZBT/backend/internal/api"
	"github.com/frankford824/ZBT/backend/internal/db/migrations"
	"github.com/frankford824/ZBT/backend/internal/platform/aicall"
	"github.com/frankford824/ZBT/backend/internal/platform/approval"
	"github.com/frankford824/ZBT/backend/internal/platform/bid"
	"github.com/frankford824/ZBT/backend/internal/platform/compliance"
	"github.com/frankford824/ZBT/backend/internal/platform/config"
	"github.com/frankford824/ZBT/backend/internal/platform/cost"
	"github.com/frankford824/ZBT/backend/internal/platform/dashboard"
	platformdb "github.com/frankford824/ZBT/backend/internal/platform/db"
	platformfile "github.com/frankford824/ZBT/backend/internal/platform/file"
	"github.com/frankford824/ZBT/backend/internal/platform/knowledge"
	"github.com/frankford824/ZBT/backend/internal/platform/project"
	"github.com/frankford824/ZBT/backend/internal/platform/saas"
	"github.com/frankford824/ZBT/backend/internal/platform/tender"
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

	fileService, err := platformfile.NewService(ctx, cfg, pool)
	if err != nil {
		log.Fatal(err)
	}

	aiCallStore := aicall.NewStore(pool)
	router := api.NewRouter(cfg, saas.NewStore(pool), fileService, knowledge.NewStore(cfg, pool, aiCallStore), bid.NewStore(cfg, pool), tender.NewStore(pool), project.NewStore(pool), cost.NewStore(pool), compliance.NewStore(pool), approval.NewStore(pool), dashboard.NewStore(pool), aiCallStore)

	log.Printf("zbt backend listening on %s", cfg.HTTPAddr)
	if err := router.Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}
