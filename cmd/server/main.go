package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/api"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/database"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/scheduler"
)

func main() {
	// initializing dependencies
	db := database.ConnectPostgres()
	defer db.Close()
	
	rdb := database.ConnectRedis()
	defer rdb.Close()

	database.InitSchema(db)
	database.SeedJobs(db)

	// setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// starting scheduler 
	cronService := scheduler.New(db, rdb)
	go cronService.Start(ctx)

	// starting API Server 
	srv := &http.Server{
		Addr:    ":8080",
		Handler: nil, // We use http.HandleFunc below
	}
	
	handler := api.NewHandler(db)
	http.HandleFunc("/", handler) // Catch-all handler

	go func() {
		fmt.Println("API server started at port 8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server crashed: %v", err)
		}
	}()

	// waiting for shutdown signal
	sig := <-sigChan
	fmt.Printf("\n Received signal: %v. Shutting down...\n", sig)
	
	cancel() // stops scheduler loop
	
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown error: %v", err)
	}

	fmt.Println("Bye, closing the program")
}