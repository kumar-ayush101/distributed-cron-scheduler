// Package service contains application business logic that sits between the
// HTTP handlers and the repository.
package service

import (
	"context"
	"time"

	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/models"
)

// JobRepo is the subset of repository behaviour the service depends on. It is
// an interface so the service can be unit-tested with a fake.
type JobRepo interface {
	Create(ctx context.Context, name, cron string, nextRun time.Time) (models.Job, error)
	GetByID(ctx context.Context, id int64) (models.Job, error)
	List(ctx context.Context, limit, offset int) (models.Page[models.Job], error)
	Update(ctx context.Context, id int64, in models.UpdateJobInput, nextRun *time.Time) (models.Job, error)
	Delete(ctx context.Context, id int64) error
	History(ctx context.Context, jobID *int64, limit, offset int) (models.Page[models.JobHistory], error)
}

// JobService orchestrates job operations and enforces domain rules.
type JobService struct {
	repo JobRepo
	now  func() time.Time
}

// NewJobService constructs a JobService.
func NewJobService(repo JobRepo) *JobService {
	return &JobService{repo: repo, now: time.Now}
}

// Create validates input and persists a new job with its first scheduled run.
func (s *JobService) Create(ctx context.Context, in models.CreateJobInput) (models.Job, error) {
	if err := in.Validate(); err != nil {
		return models.Job{}, err
	}
	next, err := models.NextRun(in.CronSchedule, s.now())
	if err != nil {
		return models.Job{}, models.ErrInvalidCron
	}
	return s.repo.Create(ctx, in.Name, in.CronSchedule, next)
}

// Get returns a job by id.
func (s *JobService) Get(ctx context.Context, id int64) (models.Job, error) {
	return s.repo.GetByID(ctx, id)
}

// List returns a paginated list of jobs.
func (s *JobService) List(ctx context.Context, limit, offset int) (models.Page[models.Job], error) {
	return s.repo.List(ctx, limit, offset)
}

// Update validates and applies a partial update. When the cron expression
// changes, the next run time is recomputed.
func (s *JobService) Update(ctx context.Context, id int64, in models.UpdateJobInput) (models.Job, error) {
	if err := in.Validate(); err != nil {
		return models.Job{}, err
	}
	var next *time.Time
	if in.CronSchedule != nil {
		n, err := models.NextRun(*in.CronSchedule, s.now())
		if err != nil {
			return models.Job{}, models.ErrInvalidCron
		}
		next = &n
	}
	return s.repo.Update(ctx, id, in, next)
}

// Delete removes a job.
func (s *JobService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

// RunNow schedules a job to run on the next scheduler tick by setting its
// next_run_at to now.
func (s *JobService) RunNow(ctx context.Context, id int64) (models.Job, error) {
	now := s.now()
	return s.repo.Update(ctx, id, models.UpdateJobInput{}, &now)
}

// History returns execution history, optionally scoped to a single job.
func (s *JobService) History(ctx context.Context, jobID *int64, limit, offset int) (models.Page[models.JobHistory], error) {
	return s.repo.History(ctx, jobID, limit, offset)
}
