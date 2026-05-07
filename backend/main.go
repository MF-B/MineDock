package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/docker/docker/client"

	"minedock/backend/internal/api"
	"minedock/backend/internal/service"
	"minedock/backend/internal/store"
)

func main() {
	logSink, err := service.NewSystemLogSink(os.Getenv("MINEDOCK_LOG_PATH"))
	if err != nil {
		log.Fatalf("init system logger: %v", err)
	}
	defer logSink.Close()
	slog.Info("system logger initialized", "path", logSink.Path())

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		fatal("init docker client", err)
	}
	defer cli.Close()

	dbPath := os.Getenv("MINEDOCK_DB_PATH")
	if dbPath == "" {
		dbPath = "data/minedock.db"
	}

	sqliteStore, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		fatal("init sqlite store", err)
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

	dataDir := os.Getenv("MINEDOCK_DATA_DIR")
	if dataDir == "" {
		dataDir = "data/instances"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fatal("init data dir", err)
	}

	gameSvc, err := service.NewGameService(gamesPath, templatesDir)
	if err != nil {
		fatal("init game service", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc := service.NewDockerServiceWithDataDir(cli, sqliteStore, gameSvc, dataDir)
	hub := service.NewEventHub(cli, svc.ListInstances)
	go hub.Run(ctx)

	h := api.NewHandler(svc)
	gameHandler := api.NewGameHandler(gameSvc)
	wsHandler := api.NewWsHandler(hub)
	consoleSvc := service.NewConsoleService(cli)
	consoleHandler := api.NewConsoleHandler(consoleSvc)
	configHandler := api.NewConfigHandler(svc)
	fileSvc := service.NewFileService(sqliteStore, gameSvc, dataDir)
	filesHandler := api.NewFilesHandler(fileSvc)
	monitorHandler := api.NewMonitorHandler(service.NewMonitorService())
	containerStatsHandler := api.NewContainerStatsHandler(service.NewContainerStatsService(cli))
	minecraftVersionHandler := api.NewMinecraftVersionHandler(service.NewMinecraftVersionService())
	systemLogHandler := api.NewSystemLogHandler(service.NewSystemLogService(logSink.Path()))
	router := api.NewRouter(
		h,
		gameHandler,
		wsHandler,
		consoleHandler,
		configHandler,
		filesHandler,
		monitorHandler,
		containerStatsHandler,
		minecraftVersionHandler,
		systemLogHandler,
	)

	addr := ":8080"
	slog.Info("backend listening", "addr", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		fatal("start server", err)
	}
}

func fatal(message string, err error) {
	slog.Error(message, "error", err)
	os.Exit(1)
}
