// Package repository provides data-access logic for jobs and execution
// history. All queries are context-aware and parameterized.
package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/models"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

// JobRepository persists jobs and their execution history.
type JobRepository struct {
	db *sql.DB
}

// NewJobRepository constructs a JobRepository.
func NewJobRepository(db *sql.DB) *JobRepository { return &JobRepository{db: db} }

const jobColumns = `id, name, cron_schedule, status, next_run_at, last_run_at,
	run_count, fail_count, created_at, updated_at`

func scanJob(s interface{ Scan(...any) error }) (models.Job, error) {
	var j models.Job
	err := s.Scan(&j.ID, &j.Name, &j.CronSchedule, &j.Status, &j.NextRunAt,
		&j.LastRunAt, &j.RunCount, &j.FailCount, &j.CreatedAt, &j.UpdatedAt)
	return j, err
}

// Create inserts a new job and returns the persisted row.
func (r *JobRepository) Create(ctx context.Context, name, cron string, nextRun time.Time) (models.Job, error) {
	query := `INSERT INTO jobs (name, cron_schedule, status, next_run_at)
		VALUES ($1, $2, 'active', $3)
		RETURNING ` + jobColumns
	row := r.db.QueryRowContext(ctx, query, name, cron, nextRun)
	j, err := scanJob(row)
	if err != nil {
		return models.Job{}, fmt.Errorf("create job: %w", err)
	}
	return j, nil
}

// GetByID returns a single job or ErrNotFound.
func (r *JobRepository) GetByID(ctx context.Context, id int64) (models.Job, error) {
	row := r.db.QueryRowContext(ctx, "SELECT "+jobColumns+" FROM jobs WHERE id = $1", id)
	j, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Job{}, ErrNotFound
	}
	if err != nil {
		return models.Job{}, fmt.Errorf("get job: %w", err)
	}
	return j, nil
}

// List returns a paginated set of jobs ordered by newest first.
func (r *JobRepository) List(ctx context.Context, limit, offset int) (models.Page[models.Job], error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM jobs").Scan(&total); err != nil {
		return models.Page[models.Job]{}, fmt.Errorf("count jobs: %w", err)
	}
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+jobColumns+" FROM jobs ORDER BY id DESC LIMIT $1 OFFSET $2", limit, offset)
	if err != nil {
		return models.Page[models.Job]{}, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()

	items := make([]models.Job, 0, limit)
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return models.Page[models.Job]{}, fmt.Errorf("scan job: %w", err)
		}
		items = append(items, j)
	}
	if err := rows.Err(); err != nil {
		return models.Page[models.Job]{}, err
	}
	return models.Page[models.Job]{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

// Update applies a partial update and returns the updated job.
func (r *JobRepository) Update(ctx context.Context, id int64, in models.UpdateJobInput, nextRun *time.Time) (models.Job, error) {
	query := `UPDATE jobs SET
		name          = COALESCE($2, name),
		cron_schedule = COALESCE($3, cron_schedule),
		status        = COALESCE($4, status),
		next_run_at   = COALESCE($5, next_run_at),
		updated_at    = now()
		WHERE id = $1
		RETURNING ` + jobColumns

	var statusArg any
	if in.Status != nil {
		statusArg = string(*in.Status)
	}
	row := r.db.QueryRowContext(ctx, query, id,
		nullString(in.Name), nullString(in.CronSchedule), statusArg, nullTime(nextRun))
	j, err := scanJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Job{}, ErrNotFound
	}
	if err != nil {
		return models.Job{}, fmt.Errorf("update job: %w", err)
	}
	return j, nil
}

// Delete removes a job. Returns ErrNotFound when no row was affected.
func (r *JobRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, "DELETE FROM jobs WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("delete job: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DueJobs returns active jobs whose next_run_at is at or before now.
func (r *JobRepository) DueJobs(ctx context.Context, now time.Time, limit int) ([]models.Job, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+jobColumns+" FROM jobs WHERE status = 'active' AND next_run_at <= $1 ORDER BY next_run_at ASC LIMIT $2",
		now, limit)
	if err != nil {
		return nil, fmt.Errorf("due jobs: %w", err)
	}
	defer rows.Close()
	var out []models.Job
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

// MarkRun updates counters and schedules the next run after a successful or
// failed execution.
func (r *JobRepository) MarkRun(ctx context.Context, id int64, runAt, nextRun time.Time, success bool) error {
	query := `UPDATE jobs SET
		last_run_at = $2,
		next_run_at = $3,
		run_count   = run_count + 1,
		fail_count  = fail_count + $4,
		updated_at  = now()
		WHERE id = $1`
	failInc := 0
	if !success {
		failInc = 1
	}
	_, err := r.db.ExecContext(ctx, query, id, runAt, nextRun, failInc)
	if err != nil {
		return fmt.Errorf("mark run: %w", err)
	}
	return nil
}

// AddHistory records a single execution.
func (r *JobRepository) AddHistory(ctx context.Context, h models.JobHistory) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO job_history (job_id, run_at, status, duration_ms, node_id, details)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		h.JobID, h.RunAt, h.Status, h.DurationMS, h.NodeID, h.Details)
	if err != nil {
		return fmt.Errorf("add history: %w", err)
	}
	return nil
}

// History returns execution records, optionally filtered by job id.
func (r *JobRepository) History(ctx context.Context, jobID *int64, limit, offset int) (models.Page[models.JobHistory], error) {
	var (
		total int64
		args  []any
		where string
	)
	if jobID != nil {
		where = " WHERE job_id = $1"
		args = append(args, *jobID)
	}
	countQ := "SELECT COUNT(*) FROM job_history" + where
	if err := r.db.QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return models.Page[models.JobHistory]{}, fmt.Errorf("count history: %w", err)
	}

	listQ := fmt.Sprintf(
		"SELECT id, job_id, run_at, status, duration_ms, node_id, details FROM job_history%s ORDER BY run_at DESC LIMIT $%d OFFSET $%d",
		where, len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.QueryContext(ctx, listQ, args...)
	if err != nil {
		return models.Page[models.JobHistory]{}, fmt.Errorf("list history: %w", err)
	}
	defer rows.Close()

	items := make([]models.JobHistory, 0, limit)
	for rows.Next() {
		var h models.JobHistory
		if err := rows.Scan(&h.ID, &h.JobID, &h.RunAt, &h.Status, &h.DurationMS, &h.NodeID, &h.Details); err != nil {
			return models.Page[models.JobHistory]{}, err
		}
		items = append(items, h)
	}
	return models.Page[models.JobHistory]{Items: items, Total: total, Limit: limit, Offset: offset}, rows.Err()
}

// --- helpers ---

func nullString(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return *t
}
