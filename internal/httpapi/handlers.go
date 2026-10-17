package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/lock"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/models"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/service"
)

// Handlers holds dependencies for the HTTP handlers.
type Handlers struct {
	jobs   *service.JobService
	db     *sql.DB
	locker lock.Locker
	log    *slog.Logger
	nodeID string
}

// NewHandlers constructs the handler set.
func NewHandlers(jobs *service.JobService, db *sql.DB, locker lock.Locker, log *slog.Logger, nodeID string) *Handlers {
	return &Handlers{jobs: jobs, db: db, locker: locker, log: log.With("component", "http"), nodeID: nodeID}
}

// --- Health & readiness ---

// Health is a cheap liveness probe.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"node_id": h.nodeID,
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

// Ready verifies downstream dependencies (database and lock backend).
func (h *Handlers) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := map[string]string{}
	healthy := true

	if err := h.db.PingContext(ctx); err != nil {
		checks["database"] = "down: " + err.Error()
		healthy = false
	} else {
		checks["database"] = "up"
	}

	if err := h.locker.Ping(ctx); err != nil {
		checks["lock_backend"] = "down: " + err.Error()
		healthy = false
	} else {
		checks["lock_backend"] = "up (" + h.locker.Kind() + ")"
	}

	status := http.StatusOK
	if !healthy {
		status = http.StatusServiceUnavailable
	}
	writeJSON(w, status, map[string]any{"ready": healthy, "checks": checks})
}

// --- Jobs ---

// ListJobs handles GET /api/jobs with pagination.
func (h *Handlers) ListJobs(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginationParams(r)
	page, err := h.jobs.List(r.Context(), limit, offset)
	if err != nil {
		writeDomainError(w, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// CreateJob handles POST /api/jobs.
func (h *Handlers) CreateJob(w http.ResponseWriter, r *http.Request) {
	var in models.CreateJobInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	job, err := h.jobs.Create(r.Context(), in)
	if err != nil {
		writeDomainError(w, h.log, err)
		return
	}
	writeJSON(w, http.StatusCreated, job)
}

// GetJob handles GET /api/jobs/{id}.
func (h *Handlers) GetJob(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	job, err := h.jobs.Get(r.Context(), id)
	if err != nil {
		writeDomainError(w, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// UpdateJob handles PATCH /api/jobs/{id}.
func (h *Handlers) UpdateJob(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in models.UpdateJobInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	job, err := h.jobs.Update(r.Context(), id, in)
	if err != nil {
		writeDomainError(w, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// DeleteJob handles DELETE /api/jobs/{id}.
func (h *Handlers) DeleteJob(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.jobs.Delete(r.Context(), id); err != nil {
		writeDomainError(w, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// RunJobNow handles POST /api/jobs/{id}/run.
func (h *Handlers) RunJobNow(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	job, err := h.jobs.RunNow(r.Context(), id)
	if err != nil {
		writeDomainError(w, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// JobHistory handles GET /api/jobs/{id}/history.
func (h *Handlers) JobHistory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	limit, offset := paginationParams(r)
	page, err := h.jobs.History(r.Context(), &id, limit, offset)
	if err != nil {
		writeDomainError(w, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// ListHistory handles GET /api/history (all jobs).
func (h *Handlers) ListHistory(w http.ResponseWriter, r *http.Request) {
	limit, offset := paginationParams(r)
	var jobID *int64
	if q := r.URL.Query().Get("job_id"); q != "" {
		if v, err := strconv.ParseInt(q, 10, 64); err == nil {
			jobID = &v
		}
	}
	page, err := h.jobs.History(r.Context(), jobID, limit, offset)
	if err != nil {
		writeDomainError(w, h.log, err)
		return
	}
	writeJSON(w, http.StatusOK, page)
}

// --- helpers ---

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	raw := r.PathValue("id")
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "invalid job id")
		return 0, false
	}
	return id, true
}

func paginationParams(r *http.Request) (limit, offset int) {
	limit, offset = 20, 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}
