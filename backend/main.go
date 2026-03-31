package main

import (
	"log"
	"net/http"
	"os"

	"github.com/docker/docker/client"

	"minedock/backend/internal/api"
	"minedock/backend/internal/service"
	"minedock/backend/internal/store"
)

func main() {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("init docker client: %v", err)
	}
	defer cli.Close()

	dbPath := os.Getenv("MINEDOCK_DB_PATH")
	if dbPath == "" {
		dbPath = "data/minedock.db"
	}

	sqliteStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		log.Fatalf("init sqlite store: %v", err)
	}
	defer sqliteStore.Close()

	registryPath := os.Getenv("MINEDOCK_REGISTRY_PATH")
	if registryPath == "" {
		registryPath = "registry.json"
	}

	registrySvc, err := service.NewRegistryService(registryPath)
	if err != nil {
		log.Fatalf("init registry service: %v", err)
	}

	svc := service.NewDockerService(cli, sqliteStore, registrySvc)
	h := api.NewHandler(svc)
	registryHandler := api.NewRegistryHandler(registrySvc)
	router := api.NewRouter(h, registryHandler)

	addr := ":8080"
	log.Printf("MineDock backend listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("start server: %v", err)
	}
}
