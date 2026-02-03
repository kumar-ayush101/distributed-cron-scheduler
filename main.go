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

type JobHistory struct {
	ID int `json:"id"`
	JobID int `json:"job_id"`
	RunAt time.Time `json:"run_at"`
	Status string `json:"status"`
	Details string `json:"details"`
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
	http.HandleFunc("/jobs/run", apiHandler(db))
	http.HandleFunc("/jobs/history", apiHandler(db))

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
	
    w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
    
		//handling preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}


		// POST - creating a new job
		if r.Method == "POST" && r.URL.Path == "/jobs" {
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

		if r.Method == "DELETE" {
			idStr := r.URL.Query().Get("id")
			if idStr == "" {
				http.Error(w, "Missing id parameter", http.StatusBadRequest)
				return
			}
		_,err := db.Exec("DELETE FROM jobs WHERE id = $1", idStr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status":"deleted"})
		return
		}

		if r.Method == "POST" && r.URL.Path == "/jobs/run" {
			idStr := r.URL.Query().Get("id")
			if idStr == "" {
				http.Error(w, "Missiing id parameter", http.StatusBadRequest)
				return
			}
			_,err := db.Exec("UPDATE jobs SET next_run_at = $1 WHERE id = $2",time.Now(), idStr)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status":"scheduled_now"})
			return
		}

		if r.Method == "GET" && r.URL.Path == "/jobs/history" {
			jobID := r.URL.Query().Get("job_id")

			query := "SELECT id, job_id, run_at, status, details FROM job_history"
			var rows *sql.Rows
			var err error

			if jobID != "" {
				query += " WHERE job_id = $1 ORDER BY run_at DESC LIMIT 50"
				rows, err = db.Query(query, jobID)
			} else {
				query += " ORDER BY run_at DESC LIMIT 50"
				rows, err = db.Query(query)
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			defer rows.Close()

			history := []JobHistory{}
			for rows.Next() {
				var h JobHistory
				if err := rows.Scan(&h.ID, &h.JobID, &h.RunAt, &h.Status, &h.Details); err != nil {
					continue
				}
				history = append(history, h)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(history)
			return
		}

		if r.Method == "GET" {
			rows, err := db.Query("SELECT id, name, cron_schedule, next_run_at FROM jobs ORDER BY id DESC")
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


		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}



func initDB(db *sql.DB) {
	queryJobs := `
	CREATE TABLE IF NOT EXISTS jobs (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		cron_schedule TEXT NOT NULL,
		next_run_at TIMESTAMP NOT NULL
	);`
	
	_, err := db.Exec(queryJobs)
	if err != nil {
		log.Fatal("Failed to create table:", err)
	}

	queryHistory := `
	CREATE TABLE IF NOT EXISTS job_history (
	id SERIAL PRIMARY KEY,
	job_id INTEGER REFERENCES jobs(id) ON DELETE CASCADE,
	run_at TIMESTAMP NOT NULL,
	status TEXT NOT NULL,
	details TEXT
	);`

	if _,err := db.Exec(queryHistory); err != nil {
		log.Fatal("Failed to create job_history table :", err)
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

		//new record history
		_,err = db.Exec(`INSERT INTO job_history (job_id, run_at, status, details) VALUES ($1, $2, $3, $4)`, j.ID, time.Now(), "Success", "Executed via scheduler")
		if err != nil {
			log.Println("failed to log job history", err)
		}

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