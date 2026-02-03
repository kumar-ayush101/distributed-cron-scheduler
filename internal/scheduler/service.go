package scheduler

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/models" 
	"github.com/redis/go-redis/v9"
	"github.com/robfig/cron/v3"
)

type Service struct {
	DB  *sql.DB
	RDB *redis.Client
}

func New(db *sql.DB, rdb *redis.Client) *Service {
	return &Service{DB: db, RDB: rdb}
}

// start runs the ticker loop.This is a blocking function, so we run it in goroutine
func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	fmt.Println("Scheduler started. Waiting for jobs...")

	for {
		select {
		case t := <-ticker.C:
			fmt.Println("\nTick at", t.Format("15:04:05"))
			s.ProcessJobs()
		case <-ctx.Done():
			fmt.Println("Stopping scheduler loop")
			return
		}
	}
}

func (s *Service) ProcessJobs() {
	rows, err := s.DB.Query("SELECT id, name, cron_schedule, next_run_at FROM jobs WHERE next_run_at <= $1", time.Now())
	if err != nil {
		log.Println("Error querying jobs:", err)
		return
	}
	defer rows.Close()

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	ctx := context.Background()

	for rows.Next() {
		var j models.Job
		if err := rows.Scan(&j.ID, &j.Name, &j.CronSchedule, &j.NextRunAt); err != nil {
			log.Println("Error scanning job:", err)
			continue
		}

		// Redis Lock
		lockKey := fmt.Sprintf("job_lock:%d", j.ID)
		isLocked, err := s.RDB.SetNX(ctx, lockKey, "locked", 10*time.Second).Result()
		if err != nil {
			log.Println("Redis error:", err)
			continue
		}
		if !isLocked {
			fmt.Printf("Job [%d] locked by another node, skipping\n", j.ID)
			continue
		}

		fmt.Printf("Executing Job [%d]: %s\n", j.ID, j.Name)

		// history recording
		_, err = s.DB.Exec(`INSERT INTO job_history (job_id, run_at, status, details) VALUES ($1, $2, $3, $4)`,
			j.ID, time.Now(), "Success", "Executed via scheduler")
		if err != nil {
			log.Println("Failed to log history:", err)
		}

		// calculating next run
		schedule, err := parser.Parse(j.CronSchedule)
		if err != nil {
			fmt.Printf("Error parsing cron: %v\n", err)
			continue
		}
		nextTime := schedule.Next(time.Now())

		// updating job
		_, err = s.DB.Exec("UPDATE jobs SET next_run_at = $1 WHERE id = $2", nextTime, j.ID)
		if err != nil {
			log.Println("Failed to update job:", err)
		}
		fmt.Printf("Rescheduled for: %v\n", nextTime.Format(time.Kitchen))
	}
}