package main

import (
	"database/sql"
	"fmt"
	"log"
	"os" 
	"time"

	_ "github.com/lib/pq"
	"github.com/robfig/cron/v3"
)

type Job struct {
	ID           int
	Name         string
	CronSchedule string
	NextRunAt    time.Time
}

func main() {
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "secret"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "postgres"
	}

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Cannot connect to DB:", err)
	}
	fmt.Println("Connected to Postgres at", dbHost)

	seedJobs(db)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	fmt.Println("Scheduler started. Waiting for jobs... (Ctrl+C to stop)")

	for {
		select {
		case t := <-ticker.C:
			fmt.Println("\n Tick at", t.Format("15:04:05"))
			processJobs(db)
		}
	}
}

func processJobs(db *sql.DB) {
	rows, err := db.Query("SELECT id, name, cron_schedule, next_run_at FROM jobs WHERE next_run_at <= $1", time.Now())
	if err != nil {
		log.Println("Error querying jobs:", err)
		return
	}
	defer rows.Close()

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Name, &j.CronSchedule, &j.NextRunAt); err != nil {
			log.Println("Error scanning job:", err)
			continue
		}

		fmt.Printf("Executing Job [%d]: %s\n", j.ID, j.Name)

		schedule, err := parser.Parse(j.CronSchedule)
		if err != nil {
			fmt.Printf("Error parsing cron for %d: %v\n", j.ID, err)
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
	db.QueryRow("SELECT COUNT(*) FROM jobs WHERE name = $1", "Send Email").Scan(&count)
	if count == 0 {
		db.Exec(`INSERT INTO jobs (name, cron_schedule, next_run_at) VALUES ($1, $2, $3)`,
			"Send Email", "*/1 * * * *", time.Now())
		fmt.Println("Inserted job: 'Send Email'")
	}

	db.QueryRow("SELECT COUNT(*) FROM jobs WHERE name = $1", "Database Backup").Scan(&count)
	if count == 0 {
		db.Exec(`INSERT INTO jobs (name, cron_schedule, next_run_at) VALUES ($1, $2, $3)`,
			"Database Backup", "*/2 * * * *", time.Now())
		fmt.Println("Inserted job: 'Database Backup'")
	}
}