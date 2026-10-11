package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gnnwatchtower/internal/api"
	"gnnwatchtower/internal/backup"
	"gnnwatchtower/internal/database"
	"gnnwatchtower/internal/monitor"
)

//go:embed dist/*
var embeddedDist embed.FS

func main() {
	log.Println("======================================================")
	log.Println(" GNNwatchtower Network Monitoring Appliance (Port 3800)")
	log.Println(" Architecture: Single Binary • Pure Go SQLite WAL")
	log.Println(" License: GNU Affero General Public License v3 (AGPL-3.0)")
	log.Println("======================================================")

	// 1. Resolve configuration
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "3800"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/gnnwatchtower.db"
	}

	// 2. Initialize Pure Go SQLite Database (WAL Mode)
	db, err := database.InitDB(dbPath)
	if err != nil {
		log.Fatalf("[FATAL] SQLite initialization failed: %v", err)
	}
	defer db.Close()

	// 3. Start Polling Engine (Goroutine worker pool)
	poller := monitor.StartPoller(15 * time.Second)
	defer poller.Stop()
	log.Println("[GNNwatchtower Core] Polling Engine started (15s baseline interval)")

	// 4. Start Nightly Automated Backup Worker (03:00 Backup Rotation)
	backup.StartDailyBackupScheduler("./backups")
	log.Println("[GNNwatchtower Core] Nightly 03:00 Backup Scheduler started")

	// 5. Mount Embedded Web UI Assets (dist/)
	var webFS fs.FS
	subFS, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		log.Printf("[WARN] Failed extracting embedded dist folder: %v", err)
	} else {
		webFS = subFS
		log.Println("[GNNwatchtower Core] Embedded Web UI mounted successfully")
	}

	// 6. Setup Gin Router
	router := api.SetupRouter(webFS)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// 7. Start HTTP Server in background
	go func() {
		log.Printf("[GNNwatchtower Appliance] Dashboard & API listening on http://0.0.0.0:%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] HTTP Server failed: %v", err)
		}
	}()

	// 8. Graceful Shutdown listener
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[GNNwatchtower] Shutting down gracefully...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("[WARN] Server forced to shutdown: %v", err)
	}

	// Checkpoint SQLite WAL mode on clean exit
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE);"); err != nil {
		log.Printf("[WARN] WAL checkpoint failed: %v", err)
	}

	log.Println("[GNNwatchtower] Appliance exited cleanly.")
}
