package main

import (
	"database/sql"
	"fmt"
	"log"
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

	connStr := "postgres://postgres:secret@localhost:5432/postgres?sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Cannot connect to DB:", err)
	}
	fmt.Println("Connected to Postgres!")

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS jobs (
            id SERIAL PRIMARY KEY,
            name TEXT NOT NULL,
            cron_schedule TEXT NOT NULL,
            next_run_at TIMESTAMP NOT NULL
        );
    `)
    if err != nil {
        log.Fatal("Error creating table:", err)
    }

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM jobs WHERE name = $1", "Send Email").Scan(&count)
	if err != nil {
		log.Fatal(err)
	}

	if count == 0 {
		_, err = db.Exec(`
			INSERT INTO jobs (name, cron_schedule, next_run_at) 
			VALUES ($1, $2, $3)`, 
			"Send Email", "*/5 * * * *", time.Now())
		
		if err != nil {
			log.Fatal("Error inserting:", err)
		}
		fmt.Println("Inserted a test job")
	} else {
		fmt.Println("Job 'Send Email' already exists. Skipping insert.")
	}
	rows, err := db.Query("SELECT id, name, cron_schedule, next_run_at FROM jobs WHERE next_run_at <= $1", time.Now())
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("\n--- PROCESSING JOBS ---")
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Name, &j.CronSchedule, &j.NextRunAt); err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Executing Job [%d]: %s\n", j.ID, j.Name)

		schedule, err := parser.Parse(j.CronSchedule)
		if err != nil {
			fmt.Printf(" Error parsing cron for %d: %v\n", j.ID, err)
			continue
		}

		nextTime := schedule.Next(time.Now())

		_, err = db.Exec("UPDATE jobs SET next_run_at = $1 WHERE id = $2", nextTime, j.ID)
		if err != nil {
			log.Fatal("failed to update the job", err)
		}
		
		fmt.Printf("Rescheduled for: %v\n", nextTime.Format(time.Kitchen))
	}
}