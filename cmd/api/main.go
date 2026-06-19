package main

import (
	"fmt"
	"os"

	"alloy/internal/tui"
)

func main() {
	app := tui.New()
	left, right := app.SplitVertically("SERVER LOGS", "")
	_, docs := right.SplitHorizontally("Server Info", "API DOCS")

	docs.SetContent(``)

	left.SetContent(``)

	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
