package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
	"net/http"
	"encoding/json"

	_ "github.com/lib/pq"
	"github.com/robfig/cron/v3"
	"github.com/redis/go-redis/v9"
)

type Job struct {
	ID           int     `json:"id"`
	Name         string      `json:"name"`
	CronSchedule string      `json:"cron_schedule"`
	NextRunAt    time.Time      `json:"next_run_at"`
}

//redis and postgres complementing each other, postgres permanently stores the data but slow for locking but redis is good for locking data if any job using it but data vanishes  if server crashes or restart because it is in memory database and we can afford losing the locks because they are temporary but not the data which is crucial so we store data in postgres here and for locking we use redis and also the queries to find specific jobs this is best to be done with postgres 

var rdb *redis.Client

func main() {

  //redis connectioon
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" {
		redisHost = "localhost"
	}
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" {
		redisPort = "6379"
	}
	fmt.Printf("connecting to redis at %s: %s", redisHost, redisPort)

	rdb = redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s",redisHost,redisPort),
	})



	ctx := context.Background()
	_,err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatal("cannot connect to redis:", err)
	}
	fmt.Println("connected to redis ")


  //postgres connection
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" { dbHost = "localhost" }
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" { dbPort = "5432" }
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" { dbUser = "postgres" }
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" { dbPassword = "secret" }
	dbName := os.Getenv("DB_NAME")
	if dbName == "" { dbName = "postgres" }

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	for i := 0; i < 5; i++ {
		if err := db.Ping(); err == nil {
			break
		}
		fmt.Println("Waiting for DB...")
		time.Sleep(2 * time.Second)
	}
	fmt.Println("Connected to Postgres!")

	initDB(db)
	seedJobs(db)

	//logic for graceful shutdown

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT , syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//starting api server

	//here custom http.Server is used for shutting it down gracefully later

  srv := &http.Server{
		Addr : ":8080",
		Handler: nil,
	}

	//to register the route
	http.HandleFunc("/jobs", apiHandler(db))

  go func() {
		fmt.Println("api server started at port 8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server crashed : %v", err)
		}
	}()

	//starting the scheduler

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	fmt.Println("Scheduler started. Waiting for jobs...")

	go func() {
     for {
		         select {
						 case t := <-ticker.C:
							fmt.Println("\n Tick at ", t.Format("15:04:05"))
							processJobs(db)
						 case <-ctx.Done():
						  fmt.Println("Stopping the scheduler loop")
							return
		}
	}
	}()
 
	//waiting for signal
	sig := <-sigChan
	fmt.Printf("\n Received signal : %v . Shutting down gracefully \n",sig)

	//stopping sheduler loop
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx) ; err != nil {
		log.Printf("http shutdown error : %v", err)
	} else {
		fmt.Println("api server stopped")
	}

	fmt.Println("Bye, closing the program")

}

func apiHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			rows, err := db.Query("SELECT id, name, cron_schedule, next_run_at FROM jobs")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			var jobs []Job
			for rows.Next() {
				var j Job
				if err := rows.Scan(&j.ID, &j.Name, &j.CronSchedule, &j.NextRunAt); err != nil {
					continue
				}
				jobs = append(jobs, j)
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(jobs)
			return
		}

		// POST - creating a new job
		if r.Method == "POST" {
			var j Job
			if err := json.NewDecoder(r.Body).Decode(&j); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}

			// calculate first run time immediately
			parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
			schedule, err := parser.Parse(j.CronSchedule)
			if err != nil {
				http.Error(w, "Invalid Cron Schedule", http.StatusBadRequest)
				return
			}
			j.NextRunAt = schedule.Next(time.Now())

			query := `INSERT INTO jobs (name, cron_schedule, next_run_at) VALUES ($1, $2, $3) RETURNING id`
			err = db.QueryRow(query, j.Name, j.CronSchedule, j.NextRunAt).Scan(&j.ID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(j)
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}



func initDB(db *sql.DB) {
	query := `
	CREATE TABLE IF NOT EXISTS jobs (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		cron_schedule TEXT NOT NULL,
		next_run_at TIMESTAMP NOT NULL
	);`
	
	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("Failed to create table:", err)
	}
	fmt.Println("Database initialized (Table 'jobs' exists)")
}



func processJobs(db *sql.DB) {
	rows, err := db.Query("SELECT id, name, cron_schedule, next_run_at FROM jobs WHERE next_run_at <= $1", time.Now())
	if err != nil {
		log.Println("Error querying jobs:", err)
		return
	}
	defer rows.Close()

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	ctx := context.Background()

	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Name, &j.CronSchedule, &j.NextRunAt); err != nil {
			log.Println("Error scanning job:", err)
			continue
		}

		lockKey := fmt.Sprintf("job_lock:%d",j.ID)

		isLocked , err := rdb.SetNX(ctx, lockKey, "locked", 10*time.Second).Result()
		if err != nil {
			log.Println("redis error in locking", err)
			continue
		}

		if !isLocked {
			fmt.Printf("job [%d] is already locked by another node, therefore skipping", j.ID)
			continue
		}

		fmt.Printf("Executing Job [%d]: %s\n", j.ID, j.Name)

		schedule, err := parser.Parse(j.CronSchedule)
		if err != nil {
			fmt.Printf("Error parsing cron: %v\n", err)
			continue
		}
		nextTime := schedule.Next(time.Now())

		_, err = db.Exec("UPDATE jobs SET next_run_at = $1 WHERE id = $2", nextTime, j.ID)
		if err != nil {
			log.Println("Failed to update job:", err)
		}

		fmt.Printf("Rescheduled for: %v\n", nextTime.Format(time.Kitchen))
	}
}



func seedJobs(db *sql.DB) {
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM jobs WHERE name = $1", "Send Email").Scan(&count)
	
	if count == 0 {
		_, err := db.Exec(`INSERT INTO jobs (name, cron_schedule, next_run_at) VALUES ($1, $2, $3)`, 
			"Send Email", "*/1 * * * *", time.Now())
		if err == nil {
			fmt.Println("Inserted job: 'Send Email'")
		}
	}
	
	_ = db.QueryRow("SELECT COUNT(*) FROM jobs WHERE name = $1", "Database Backup").Scan(&count)
	if count == 0 {
		_, err := db.Exec(`INSERT INTO jobs (name, cron_schedule, next_run_at) VALUES ($1, $2, $3)`, 
			"Database Backup", "*/2 * * * *", time.Now())
		if err == nil {
			fmt.Println("Inserted job: 'Database Backup'")
		}
	}
}