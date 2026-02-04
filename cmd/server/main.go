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

	// Creating a new ServeMux (Router) instead of using the global default
	mux := http.NewServeMux()

	handler := api.NewHandler(db)
	mux.HandleFunc("/", handler)

	// wrap the entire mux with the CORS middleware
	corsHandler := enableCORS(mux)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: corsHandler, //  CORS-wrapped router
	}

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

// enableCORS middleware
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handling Preflight requests 
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}