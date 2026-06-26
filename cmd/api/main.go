package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"

	"alloy/internal/app/boot"
	"alloy/internal/app/config"
	"alloy/internal/platform/messaging"
	"alloy/internal/tui"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

var (
	logs, server_log, apidocslogs, others func(string)
	zeroLogs                               bool
)

func bufLog(fn func(string)) func(string) {
	ch := make(chan string, 512)
	go func() {
		for s := range ch {
			fn(s)
		}
	}()
	return func(s string) { ch <- s }
}

func main() {
	noTUI := flag.Bool("no-tui", false, "run without TUI")
	noVerbose := flag.Bool("no-verbose", false, "suppress verbose log output")
	flag.BoolVar(&zeroLogs, "zero-logs", false, "suppress all log output")
	maxCPU := flag.Int("max-cpu-cores", 0, "maximum CPU cores to use (0 = all)")
	maxRAM := flag.String("max-ram-usage", "", "maximum memory usage (e.g. 512MB, 2GB)")
	flag.Parse()

	if *maxCPU > 0 {
		runtime.GOMAXPROCS(*maxCPU)
	}
	if *maxRAM != "" {
		v, err := parseRAM(*maxRAM)
		if err == nil {
			debug.SetMemoryLimit(v)
		}
	}

	if *noVerbose {
		logs = func(string) {}
	}
	if zeroLogs {
		logs = func(string) {}
		server_log = func(string) {}
		others = func(string) {}
		apidocslogs = func(string) {}
	}
	if *noTUI {
		if logs == nil {
			logs = func(s string) { fmt.Println("[log]", s) }
		}
		if server_log == nil {
			server_log = func(s string) { fmt.Println("[server]", s) }
		}
		if others == nil {
			others = func(s string) { fmt.Println("[info]", s) }
		}
		if apidocslogs == nil {
			apidocslogs = func(s string) { fmt.Println("[api]", s) }
		}
		if err := run_app(*noVerbose); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// TUI mode
	app := tui.New()
	left, right := app.SplitVertically("SERVER LOGS", "")
	infoPane, apiPane := right.SplitHorizontally("Server Info", "API DOCS")
	logPane, serverPane := left.SplitHorizontally("Logs", "Server Logs")
	setLog := func(dst *func(string), src func(string)) {
		if *dst == nil {
			*dst = bufLog(src)
		}
	}
	setLog(&others, infoPane.AppendContent)
	setLog(&apidocslogs, apiPane.AppendContent)
	setLog(&logs, logPane.AppendContent)
	setLog(&server_log, serverPane.AppendContent)
	go run_app(*noVerbose)
	if err := app.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if err := run_app(*noVerbose); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run_app(noVerbose bool) error {
	cfg, err := config.Load()
	if err != nil {
		server_log("Error loading config: " + err.Error())
		return err
	}
	logs(fmt.Sprintf("Loaded config: %+v", cfg))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	api_app, err := boot.Build(ctx, cfg, logs)
	if err != nil {
		server_log("Error building app: " + err.Error())
		return err
	}
	server_log("App Started at http://localhost:" + strconv.Itoa(cfg.App.Port))
	defer api_app.Close()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stop
		server_log("Shutting down...")
		cancel()
	}()

	if !zeroLogs {
		api_app.Bus.Subscribe("log.info", func(ctx context.Context, msg messaging.Message) error {
			others("BUS (log.info): " + msg.Payload.(string))
			return nil
		})

		if !noVerbose {
			api_app.Bus.Subscribe("log", func(ctx context.Context, msg messaging.Message) error {
				logs("BUS (log): " + msg.Payload.(string))
				return nil
			})
		}
	}

	server_log("accounting_core api ready")
	server_log("env : " + cfg.App.Env)
	server_log("port : " + strconv.Itoa(cfg.App.Port))
	return api_app.Run(ctx)
}

func parseRAM(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)
	switch {
	case strings.HasSuffix(s, "TB"):
		v, err := strconv.ParseInt(strings.TrimSuffix(s, "TB"), 10, 64)
		return v * 1 << 40, err
	case strings.HasSuffix(s, "GB"):
		v, err := strconv.ParseInt(strings.TrimSuffix(s, "GB"), 10, 64)
		return v * 1 << 30, err
	case strings.HasSuffix(s, "MB"):
		v, err := strconv.ParseInt(strings.TrimSuffix(s, "MB"), 10, 64)
		return v * 1 << 20, err
	case strings.HasSuffix(s, "KB"):
		v, err := strconv.ParseInt(strings.TrimSuffix(s, "KB"), 10, 64)
		return v * 1 << 10, err
	case strings.HasSuffix(s, "B"):
		return strconv.ParseInt(strings.TrimSuffix(s, "B"), 10, 64)
	default:
		return strconv.ParseInt(s, 10, 64)
	}
}
