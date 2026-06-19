package main

import (
	"context"
	"fmt"
	"os"

	"alloy/internal/app/boot"
	"alloy/internal/tui"
)

var (
	left, right, docs *tui.Pane
)

func main() {
	app := tui.New()
	left, right = app.SplitVertically("SERVER LOGS", "")
	_, docs = right.SplitHorizontally("Server Info", "API DOCS")

	docs.SetContent(``)

	left.SetContent(`test\n\n\n\n\n\n\n\n\n\n\n\n\n\n\n`)
	go run_app()
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run_app() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api_app, err := boot.Build(ctx)
	if err != nil {
		return err
	}
	docs.AppendContent("App Started")
	defer api_app.Close()
	return nil
}
