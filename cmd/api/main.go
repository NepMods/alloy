package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

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

	api_app, err := boot.Build(ctx, cfg, logs.AppendContent)
	if err != nil {
		return err
	}
	server_log.AppendContent("App Started at http://localhost:" + strconv.Itoa(cfg.App.Port))
	defer api_app.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		server_log.AppendContent("Shutting down...")
		cancel()
	}()

	server_log.AppendContent("accounting_core api ready")
	server_log.AppendContent("env : " + cfg.App.Env)
	server_log.AppendContent("port : " + strconv.Itoa(cfg.App.Port))

	return api_app.Run(ctx, logs.AppendContent)
}
