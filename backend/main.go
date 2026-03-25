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

	memStore := store.NewMemoryStore()
	imageName := os.Getenv("MINEDOCK_IMAGE")
	svc := service.NewDockerService(cli, memStore, imageName)
	h := api.NewHandler(svc)
	router := api.NewRouter(h)

	addr := ":8080"
	log.Printf("MineDock backend listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("start server: %v", err)
	}
}
