package models

import "time"

type Job struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	CronSchedule string    `json:"cron_schedule"`
	NextRunAt    time.Time `json:"next_run_at"`
}

type JobHistory struct {
	ID      int       `json:"id"`
	JobID   int       `json:"job_id"`
	RunAt   time.Time `json:"run_at"`
	Status  string    `json:"status"`
	Details string    `json:"details"`
}