package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"cfst-server/internal/api"
	"cfst-server/internal/config"
	"cfst-server/internal/scheduler"
)

func fatalWithPause(format string, v ...interface{}) {
	log.Printf(format, v...)
	if runtime.GOOS == "windows" {
		fmt.Println("\n按回车键退出...")
		fmt.Scanln()
	}
	os.Exit(1)
}

//go:embed web/*
var webFS embed.FS

func main() {
	configPath := flag.String("c", "config.yaml", "Config file path")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		fatalWithPause("Failed to load config: %v", err)
	}

	log.Printf("[Config] Loaded from %s", *configPath)
	log.Printf("[Config] DNS Record: %s", cfg.DNS.RecordName)

	handler := api.NewHandler(cfg)

	handler.LoadDomains()

	if cfg.Speedtest.Schedule != "" {
		sched := scheduler.New()
		if err := sched.AddTask(cfg.Speedtest.Schedule, func() {
			log.Println("[Scheduler] Running auto speed test...")
		}); err != nil {
			log.Printf("[Scheduler] Error: %v", err)
		} else {
			sched.Start()
			defer sched.Stop()
		}
	}

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	webSub, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("Failed to load web assets: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(webSub)))

	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("[Server] Starting on %s", addr)

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("[Server] Shutting down...")
		os.Exit(0)
	}()

	if err := http.ListenAndServe(addr, mux); err != nil {
		fatalWithPause("[Server] Error: %v", err)
	}
}
