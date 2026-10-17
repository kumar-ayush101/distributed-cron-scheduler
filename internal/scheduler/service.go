// Package scheduler contains the polling engine that discovers due jobs,
// acquires a distributed lock per job, executes it exactly once across the
// cluster, records history and reschedules the next run.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/lock"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/metrics"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/models"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/repository"
)

// Executor runs the side effect of a job. In this reference implementation it
// is a stub, but it is injectable so real work (HTTP calls, queue publishes,
// shell commands) can be plugged in and unit-tested.
type Executor func(ctx context.Context, job models.Job) error

// defaultExecutor simulates a small unit of work.
func defaultExecutor(_ context.Context, _ models.Job) error { return nil }

// Service is the scheduling engine.
type Service struct {
	repo     *repository.JobRepository
	locker   lock.Locker
	log      *slog.Logger
	metrics  *metrics.Collectors
	nodeID   string
	interval time.Duration
	lockTTL  time.Duration
	batch    int
	execute  Executor
}

// Options configures the scheduler.
type Options struct {
	NodeID       string
	PollInterval time.Duration
	LockTTL      time.Duration
	Batch        int
	Executor     Executor
}

// New builds a scheduler Service.
func New(repo *repository.JobRepository, locker lock.Locker, log *slog.Logger, m *metrics.Collectors, opts Options) *Service {
	if opts.Batch <= 0 {
		opts.Batch = 100
	}
	exec := opts.Executor
	if exec == nil {
		exec = defaultExecutor
	}
	return &Service{
		repo:     repo,
		locker:   locker,
		log:      log.With("component", "scheduler"),
		metrics:  m,
		nodeID:   opts.NodeID,
		interval: opts.PollInterval,
		lockTTL:  opts.LockTTL,
		batch:    opts.Batch,
		execute:  exec,
	}
}

// Run blocks until ctx is cancelled, polling for due jobs each interval.
func (s *Service) Run(ctx context.Context) {
	s.log.Info("scheduler started", "node_id", s.nodeID, "interval", s.interval.String(), "lock_backend", s.locker.Kind())
	if s.metrics != nil {
		s.metrics.ActiveNodes.Set(1)
	}
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.log.Info("scheduler stopping")
			if s.metrics != nil {
				s.metrics.ActiveNodes.Set(0)
			}
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Service) tick(ctx context.Context) {
	jobs, err := s.repo.DueJobs(ctx, time.Now(), s.batch)
	if err != nil {
		s.log.Error("failed to query due jobs", "error", err)
		return
	}
	for _, job := range jobs {
		s.processJob(ctx, job)
	}
}

// processJob attempts to run a single job under a distributed lock.
func (s *Service) processJob(ctx context.Context, job models.Job) {
	lockKey := fmt.Sprintf("cron:job:%d", job.ID)
	ok, release, err := s.locker.Acquire(ctx, lockKey)
	if err != nil {
		s.log.Error("lock acquisition failed", "job_id", job.ID, "error", err)
		return
	}
	if !ok {
		if s.metrics != nil {
			s.metrics.LockConflicts.Inc()
		}
		s.log.Debug("job locked by another node, skipping", "job_id", job.ID)
		return
	}
	defer release()

	// Re-read the job to avoid a stale double-run: another node may have
	// executed and rescheduled it between our query and lock acquisition.
	fresh, err := s.repo.GetByID(ctx, job.ID)
	if err != nil {
		s.log.Error("failed to reload job", "job_id", job.ID, "error", err)
		return
	}
	if fresh.NextRunAt.After(time.Now()) {
		s.log.Debug("job already rescheduled by peer, skipping", "job_id", job.ID)
		return
	}

	s.runOnce(ctx, fresh)
}

func (s *Service) runOnce(ctx context.Context, job models.Job) {
	start := time.Now()
	execErr := s.execute(ctx, job)
	duration := time.Since(start)
	success := execErr == nil

	status := models.ExecSuccess
	details := "executed successfully"
	if !success {
		status = models.ExecFailed
		details = execErr.Error()
	}

	if s.metrics != nil {
		s.metrics.JobsExecuted.WithLabelValues(string(status)).Inc()
		s.metrics.JobDuration.Observe(duration.Seconds())
	}

	_ = s.repo.AddHistory(ctx, models.JobHistory{
		JobID:      job.ID,
		RunAt:      start,
		Status:     status,
		DurationMS: duration.Milliseconds(),
		NodeID:     s.nodeID,
		Details:    details,
	})

	next, err := models.NextRun(job.CronSchedule, time.Now())
	if err != nil {
		s.log.Error("invalid cron while rescheduling", "job_id", job.ID, "error", err)
		return
	}
	if err := s.repo.MarkRun(ctx, job.ID, start, next, success); err != nil {
		s.log.Error("failed to mark run", "job_id", job.ID, "error", err)
		return
	}
	s.log.Info("job executed",
		"job_id", job.ID, "name", job.Name, "status", status,
		"duration_ms", duration.Milliseconds(), "next_run", next.Format(time.RFC3339))
}
