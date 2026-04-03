package main

import (
	"context"
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

	gamesPath := os.Getenv("MINEDOCK_GAMES_PATH")
	if gamesPath == "" {
		gamesPath = "games.json"
	}

	templatesDir := os.Getenv("MINEDOCK_TEMPLATES_DIR")
	if templatesDir == "" {
		templatesDir = "templates"
	}

	gameSvc, err := service.NewGameService(gamesPath, templatesDir)
	if err != nil {
		log.Fatalf("init game service: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc := service.NewDockerService(cli, sqliteStore, gameSvc)
	hub := service.NewEventHub(cli, svc.ListInstances)
	go hub.Run(ctx)

	h := api.NewHandler(svc)
	gameHandler := api.NewGameHandler(gameSvc)
	wsHandler := api.NewWsHandler(hub)
	consoleSvc := service.NewConsoleService(cli)
	consoleHandler := api.NewConsoleHandler(consoleSvc)
	router := api.NewRouter(h, gameHandler, wsHandler, consoleHandler)

	addr := ":8080"
	log.Printf("MineDock backend listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("start server: %v", err)
	}
}
