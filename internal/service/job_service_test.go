package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/models"
)

// fakeRepo is an in-memory JobRepo for tests.
type fakeRepo struct {
	jobs      map[int64]models.Job
	nextID    int64
	createErr error
	updateErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{jobs: map[int64]models.Job{}, nextID: 1}
}

func (f *fakeRepo) Create(_ context.Context, name, cron string, nextRun time.Time) (models.Job, error) {
	if f.createErr != nil {
		return models.Job{}, f.createErr
	}
	j := models.Job{ID: f.nextID, Name: name, CronSchedule: cron, Status: models.StatusActive, NextRunAt: nextRun}
	f.jobs[f.nextID] = j
	f.nextID++
	return j, nil
}

func (f *fakeRepo) GetByID(_ context.Context, id int64) (models.Job, error) {
	j, ok := f.jobs[id]
	if !ok {
		return models.Job{}, errNotFound
	}
	return j, nil
}

func (f *fakeRepo) List(_ context.Context, limit, offset int) (models.Page[models.Job], error) {
	items := make([]models.Job, 0, len(f.jobs))
	for _, j := range f.jobs {
		items = append(items, j)
	}
	return models.Page[models.Job]{Items: items, Total: int64(len(items)), Limit: limit, Offset: offset}, nil
}

func (f *fakeRepo) Update(_ context.Context, id int64, in models.UpdateJobInput, nextRun *time.Time) (models.Job, error) {
	if f.updateErr != nil {
		return models.Job{}, f.updateErr
	}
	j, ok := f.jobs[id]
	if !ok {
		return models.Job{}, errNotFound
	}
	if in.Name != nil {
		j.Name = *in.Name
	}
	if in.CronSchedule != nil {
		j.CronSchedule = *in.CronSchedule
	}
	if in.Status != nil {
		j.Status = *in.Status
	}
	if nextRun != nil {
		j.NextRunAt = *nextRun
	}
	f.jobs[id] = j
	return j, nil
}

func (f *fakeRepo) Delete(_ context.Context, id int64) error {
	if _, ok := f.jobs[id]; !ok {
		return errNotFound
	}
	delete(f.jobs, id)
	return nil
}

func (f *fakeRepo) History(_ context.Context, _ *int64, limit, offset int) (models.Page[models.JobHistory], error) {
	return models.Page[models.JobHistory]{Items: []models.JobHistory{}, Limit: limit, Offset: offset}, nil
}

var errNotFound = errors.New("not found")

func TestJobService_Create(t *testing.T) {
	svc := NewJobService(newFakeRepo())

	j, err := svc.Create(context.Background(), models.CreateJobInput{Name: "job", CronSchedule: "*/5 * * * *"})
	if err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if j.ID == 0 || j.Name != "job" {
		t.Fatalf("unexpected job: %+v", j)
	}
	if j.NextRunAt.IsZero() {
		t.Error("NextRunAt should be computed")
	}
}

func TestJobService_Create_ValidationError(t *testing.T) {
	svc := NewJobService(newFakeRepo())
	_, err := svc.Create(context.Background(), models.CreateJobInput{Name: "", CronSchedule: "*/5 * * * *"})
	if !errors.Is(err, models.ErrEmptyName) {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}
}

func TestJobService_Update_RecomputesNextRun(t *testing.T) {
	repo := newFakeRepo()
	svc := NewJobService(repo)
	fixed := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	svc.now = func() time.Time { return fixed }

	created, _ := svc.Create(context.Background(), models.CreateJobInput{Name: "job", CronSchedule: "*/5 * * * *"})

	newCron := "0 0 * * *" // midnight daily
	updated, err := svc.Update(context.Background(), created.ID, models.UpdateJobInput{CronSchedule: &newCron})
	if err != nil {
		t.Fatalf("Update error: %v", err)
	}
	wantNext := fixed.Add(24 * time.Hour)
	if !updated.NextRunAt.Equal(wantNext) {
		t.Fatalf("NextRunAt = %v, want %v", updated.NextRunAt, wantNext)
	}
}

func TestJobService_RunNow(t *testing.T) {
	repo := newFakeRepo()
	svc := NewJobService(repo)
	created, _ := svc.Create(context.Background(), models.CreateJobInput{Name: "job", CronSchedule: "0 0 * * *"})

	before := time.Now()
	updated, err := svc.RunNow(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("RunNow error: %v", err)
	}
	if updated.NextRunAt.Before(before.Add(-time.Second)) {
		t.Fatalf("RunNow did not set next_run to ~now: %v", updated.NextRunAt)
	}
}

func TestJobService_Delete(t *testing.T) {
	repo := newFakeRepo()
	svc := NewJobService(repo)
	created, _ := svc.Create(context.Background(), models.CreateJobInput{Name: "job", CronSchedule: "0 0 * * *"})

	if err := svc.Delete(context.Background(), created.ID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	if _, err := svc.Get(context.Background(), created.ID); err == nil {
		t.Fatal("expected job to be gone")
	}
}
