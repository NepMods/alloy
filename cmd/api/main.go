package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"alloy/internal/app/boot"
	"alloy/internal/app/config"
	"alloy/internal/tui"
)

var (
	logs, server_log, docs, others *tui.Pane
)

func main() {
	app := tui.New()
	left, right := app.SplitVertically("SERVER LOGS", "")
	others, docs = right.SplitHorizontally("Server Info", "API DOCS")
	logs, server_log = left.SplitHorizontally("Logs", "Server Logs")
	go run_app()
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run_app() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	logs.AppendContent(fmt.Sprintf("Loaded config: %+v", cfg))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api_app, err := boot.Build(ctx)
	if err != nil {
		return err
	}
	server_log.AppendContent("App Started at http://localhost:" + strconv.Itoa(cfg.App.Port))
	defer api_app.Close()
	return nil
}
