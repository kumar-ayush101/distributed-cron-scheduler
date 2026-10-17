package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/models"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/repository"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/service"
)

// fakeRepo implements service.JobRepo for handler tests.
type fakeRepo struct {
	jobs   map[int64]models.Job
	nextID int64
}

func newFakeRepo() *fakeRepo { return &fakeRepo{jobs: map[int64]models.Job{}, nextID: 1} }

func (f *fakeRepo) Create(_ context.Context, name, cron string, nextRun time.Time) (models.Job, error) {
	j := models.Job{ID: f.nextID, Name: name, CronSchedule: cron, Status: models.StatusActive, NextRunAt: nextRun}
	f.jobs[f.nextID] = j
	f.nextID++
	return j, nil
}
func (f *fakeRepo) GetByID(_ context.Context, id int64) (models.Job, error) {
	j, ok := f.jobs[id]
	if !ok {
		return models.Job{}, repository.ErrNotFound
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
	j, ok := f.jobs[id]
	if !ok {
		return models.Job{}, repository.ErrNotFound
	}
	if in.Name != nil {
		j.Name = *in.Name
	}
	if nextRun != nil {
		j.NextRunAt = *nextRun
	}
	f.jobs[id] = j
	return j, nil
}
func (f *fakeRepo) Delete(_ context.Context, id int64) error {
	if _, ok := f.jobs[id]; !ok {
		return repository.ErrNotFound
	}
	delete(f.jobs, id)
	return nil
}
func (f *fakeRepo) History(_ context.Context, _ *int64, limit, offset int) (models.Page[models.JobHistory], error) {
	return models.Page[models.JobHistory]{Items: []models.JobHistory{}, Limit: limit, Offset: offset}, nil
}

func testRouter(repo service.JobRepo) http.Handler {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := service.NewJobService(repo)
	h := NewHandlers(svc, nil, nil, log, "test-node")

	// Build a mux without the DB-dependent ready/metrics routes.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /api/jobs", h.ListJobs)
	mux.HandleFunc("POST /api/jobs", h.CreateJob)
	mux.HandleFunc("GET /api/jobs/{id}", h.GetJob)
	mux.HandleFunc("PATCH /api/jobs/{id}", h.UpdateJob)
	mux.HandleFunc("DELETE /api/jobs/{id}", h.DeleteJob)
	mux.HandleFunc("POST /api/jobs/{id}/run", h.RunJobNow)
	return chain(mux, requestIDMiddleware, corsMiddleware("*"))
}

func TestHealth(t *testing.T) {
	rr := httptest.NewRecorder()
	testRouter(newFakeRepo()).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "ok") {
		t.Fatalf("body = %s", rr.Body.String())
	}
}

func TestCreateJob_Success(t *testing.T) {
	r := testRouter(newFakeRepo())
	body := `{"name":"backup","cron_schedule":"*/5 * * * *"}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201, body=%s", rr.Code, rr.Body.String())
	}
	var job models.Job
	if err := json.Unmarshal(rr.Body.Bytes(), &job); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if job.Name != "backup" {
		t.Errorf("name = %q", job.Name)
	}
}

func TestCreateJob_InvalidCron(t *testing.T) {
	r := testRouter(newFakeRepo())
	body := `{"name":"backup","cron_schedule":"nope"}`
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(body))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestCreateJob_BadJSON(t *testing.T) {
	r := testRouter(newFakeRepo())
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader("{not json"))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestGetJob_NotFound(t *testing.T) {
	r := testRouter(newFakeRepo())
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/999", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestGetJob_InvalidID(t *testing.T) {
	r := testRouter(newFakeRepo())
	req := httptest.NewRequest(http.MethodGet, "/api/jobs/abc", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestJobLifecycle(t *testing.T) {
	repo := newFakeRepo()
	r := testRouter(repo)

	// Create
	req := httptest.NewRequest(http.MethodPost, "/api/jobs", strings.NewReader(`{"name":"job","cron_schedule":"*/1 * * * *"}`))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	var created models.Job
	_ = json.Unmarshal(rr.Body.Bytes(), &created)

	// Get
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/jobs/1", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("get status = %d", rr.Code)
	}

	// Run now
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/jobs/1/run", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("run status = %d", rr.Code)
	}

	// Delete
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/jobs/1", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("delete status = %d", rr.Code)
	}

	// Confirm gone
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/jobs/1", nil))
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", rr.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	r := testRouter(newFakeRepo())
	req := httptest.NewRequest(http.MethodOptions, "/api/jobs", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)
	if rr.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", rr.Code)
	}
	if rr.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("missing CORS header")
	}
}
