package httpapi

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/metrics"
)

// RouterConfig configures route registration.
type RouterConfig struct {
	Handlers      *Handlers
	Metrics       *metrics.Collectors
	Log           *slog.Logger
	AllowOrigin   string
	ServeFrontend bool
	FrontendDir   string
}

// NewRouter builds the fully wired HTTP handler.
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()
	h := cfg.Handlers

	// Health & readiness
	mux.HandleFunc("GET /health", h.Health)
	mux.HandleFunc("GET /ready", h.Ready)

	// Observability
	mux.Handle("GET /metrics", promhttp.Handler())

	// REST API (Go 1.22+ method + path-value routing)
	mux.HandleFunc("GET /api/jobs", h.ListJobs)
	mux.HandleFunc("POST /api/jobs", h.CreateJob)
	mux.HandleFunc("GET /api/jobs/{id}", h.GetJob)
	mux.HandleFunc("PATCH /api/jobs/{id}", h.UpdateJob)
	mux.HandleFunc("DELETE /api/jobs/{id}", h.DeleteJob)
	mux.HandleFunc("POST /api/jobs/{id}/run", h.RunJobNow)
	mux.HandleFunc("GET /api/jobs/{id}/history", h.JobHistory)
	mux.HandleFunc("GET /api/history", h.ListHistory)

	// Static frontend (single-binary deployment) with SPA fallback.
	if cfg.ServeFrontend {
		mux.Handle("/", spaHandler(cfg.FrontendDir, cfg.Log))
	}

	handler := chain(mux,
		recoverMiddleware(cfg.Log),
		requestIDMiddleware,
		corsMiddleware(cfg.AllowOrigin),
		observabilityMiddleware(cfg.Log, cfg.Metrics),
	)
	return handler
}

// spaHandler serves static files from dir and falls back to index.html for
// client-side routes. API/health/metrics routes are matched more specifically
// above so they never reach this handler.
func spaHandler(dir string, log *slog.Logger) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	index := filepath.Join(dir, "index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Clean and resolve the requested path within dir.
		reqPath := strings.TrimPrefix(filepath.Clean(r.URL.Path), string(os.PathSeparator))
		full := filepath.Join(dir, reqPath)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		// Fall back to index.html for SPA client routes.
		if _, err := os.Stat(index); err != nil {
			writeError(w, http.StatusNotFound, "frontend not built")
			return
		}
		http.ServeFile(w, r, index)
	})
}
