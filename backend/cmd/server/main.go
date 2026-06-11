package main

import (
	"log"

	"github.com/frankford824/ZBT/backend/internal/api"
	"github.com/frankford824/ZBT/backend/internal/platform/config"
)

func main() {
	cfg := config.Load()
	router := api.NewRouter(cfg)

	log.Printf("zbt backend listening on %s", cfg.HTTPAddr)
	if err := router.Run(cfg.HTTPAddr); err != nil {
		log.Fatal(err)
	}
}
