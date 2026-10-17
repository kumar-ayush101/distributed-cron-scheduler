// Package models defines the core domain types for the scheduler.
package models

import (
	"errors"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// cronParser accepts standard 5-field cron expressions (minute precision).
var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// JobStatus represents the lifecycle state of a job.
type JobStatus string

const (
	StatusActive   JobStatus = "active"
	StatusPaused   JobStatus = "paused"
	StatusDisabled JobStatus = "disabled"
)

// Job is a scheduled unit of work.
type Job struct {
	ID           int64     `json:"id"`
	Name         string    `json:"name"`
	CronSchedule string    `json:"cron_schedule"`
	Status       JobStatus `json:"status"`
	NextRunAt    time.Time `json:"next_run_at"`
	LastRunAt    *time.Time `json:"last_run_at,omitempty"`
	RunCount     int64     `json:"run_count"`
	FailCount    int64     `json:"fail_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CreateJobInput is the payload for creating a job.
type CreateJobInput struct {
	Name         string `json:"name"`
	CronSchedule string `json:"cron_schedule"`
}

// UpdateJobInput is the payload for partial updates. Nil fields are ignored.
type UpdateJobInput struct {
	Name         *string    `json:"name,omitempty"`
	CronSchedule *string    `json:"cron_schedule,omitempty"`
	Status       *JobStatus `json:"status,omitempty"`
}

// ExecutionStatus describes the outcome of a single run.
type ExecutionStatus string

const (
	ExecSuccess ExecutionStatus = "success"
	ExecFailed  ExecutionStatus = "failed"
)

// JobHistory is a single execution record.
type JobHistory struct {
	ID         int64           `json:"id"`
	JobID      int64           `json:"job_id"`
	RunAt      time.Time       `json:"run_at"`
	Status     ExecutionStatus `json:"status"`
	DurationMS int64           `json:"duration_ms"`
	NodeID     string          `json:"node_id"`
	Details    string          `json:"details"`
}

// Page is a generic pagination envelope.
type Page[T any] struct {
	Items  []T   `json:"items"`
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

// Validation errors.
var (
	ErrEmptyName    = errors.New("name is required")
	ErrNameTooLong  = errors.New("name must be 120 characters or fewer")
	ErrEmptyCron    = errors.New("cron_schedule is required")
	ErrInvalidCron  = errors.New("cron_schedule is not a valid 5-field cron expression")
	ErrInvalidState = errors.New("status must be one of: active, paused, disabled")
)

// Validate checks the create input and returns the first validation error.
func (in CreateJobInput) Validate() error {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return ErrEmptyName
	}
	if len(name) > 120 {
		return ErrNameTooLong
	}
	if strings.TrimSpace(in.CronSchedule) == "" {
		return ErrEmptyCron
	}
	if _, err := ParseSchedule(in.CronSchedule); err != nil {
		return ErrInvalidCron
	}
	return nil
}

// Validate checks the update input.
func (in UpdateJobInput) Validate() error {
	if in.Name != nil {
		n := strings.TrimSpace(*in.Name)
		if n == "" {
			return ErrEmptyName
		}
		if len(n) > 120 {
			return ErrNameTooLong
		}
	}
	if in.CronSchedule != nil {
		if strings.TrimSpace(*in.CronSchedule) == "" {
			return ErrEmptyCron
		}
		if _, err := ParseSchedule(*in.CronSchedule); err != nil {
			return ErrInvalidCron
		}
	}
	if in.Status != nil && !in.Status.Valid() {
		return ErrInvalidState
	}
	return nil
}

// Valid reports whether the status is one of the known values.
func (s JobStatus) Valid() bool {
	switch s {
	case StatusActive, StatusPaused, StatusDisabled:
		return true
	default:
		return false
	}
}

// ParseSchedule parses a 5-field cron expression.
func ParseSchedule(expr string) (cron.Schedule, error) {
	return cronParser.Parse(strings.TrimSpace(expr))
}

// NextRun computes the next execution time after the given reference time.
func NextRun(expr string, after time.Time) (time.Time, error) {
	sched, err := ParseSchedule(expr)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(after), nil
}
