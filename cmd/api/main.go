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
	"alloy/internal/modules/hello_world"
	"alloy/internal/platform/messaging"
	server "alloy/internal/server"
	"alloy/internal/tui"
	"alloy/models/apidocs"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var (
	logs, server_log, apidocslogs, others *tui.Pane
)

func main() {
	app := tui.New()
	left, right := app.SplitVertically("SERVER LOGS", "")
	others, apidocslogs = right.SplitHorizontally("Server Info", "API DOCS")
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
		server_log.AppendContent("Error loading config: " + err.Error())
		return err
	}
	logs.AppendContent(fmt.Sprintf("Loaded config: %+v", cfg))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api_app, err := boot.Build(ctx, cfg, logs.AppendContent)
	if err != nil {
		server_log.AppendContent("Error building app: " + err.Error())
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

	api_app.Bus.Subscribe("log.info", func(ctx context.Context, msg messaging.Message) error {
		others.AppendContent("BUS (log.info): " + msg.Payload.(string))
		return nil
	})

	api_app.Bus.Subscribe("log", func(ctx context.Context, msg messaging.Message) error {
		logs.AppendContent("BUS (log): " + msg.Payload.(string))
		return nil
	})

	server_log.AppendContent("accounting_core api ready")
	server_log.AppendContent("env : " + cfg.App.Env)
	server_log.AppendContent("port : " + strconv.Itoa(cfg.App.Port))
	printAPIDocs()
	return api_app.Run(ctx)
}

func printAPIDocs() {
	var docs []apidocs.RouteDoc

	cm := hello_world.Module{}
	docs = append(docs, server.RouteDocs()...)
	docs = append(docs, cm.RouteDocs()...)

	if len(docs) == 0 {
		return
	}

	apidocs.PrintAPIRoutes(docs, apidocslogs.AppendContent)
}
