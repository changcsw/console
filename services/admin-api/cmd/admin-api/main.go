package main

import (
	"log"

	"github.com/csw/console/services/admin-api/internal/infra/config"
	"github.com/csw/console/services/admin-api/internal/transport/httpserver"
)

func main() {
	cfg := config.MustLoad()
	server := httpserver.New(cfg)

	log.Printf("admin-api listening on %s", cfg.HTTPAddress)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

