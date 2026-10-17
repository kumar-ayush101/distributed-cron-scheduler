package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"time"

	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/metrics"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

// statusRecorder captures the response status code for logging and metrics.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.wrote = true
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wrote {
		r.status = http.StatusOK
		r.wrote = true
	}
	return r.ResponseWriter.Write(b)
}

// chain composes middleware so the first argument is the outermost layer.
func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// corsMiddleware sets permissive CORS headers and short-circuits preflight.
func corsMiddleware(allowOrigin string) func(http.Handler) http.Handler {
	if allowOrigin == "" {
		allowOrigin = "*"
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requestIDMiddleware attaches a request id to the context and response.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = newRequestID()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// recoverMiddleware converts panics into 500 responses and logs them.
func recoverMiddleware(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered", "panic", rec, "path", r.URL.Path)
					writeError(w, http.StatusInternalServerError, "internal server error")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// observabilityMiddleware logs each request and records Prometheus metrics.
func observabilityMiddleware(log *slog.Logger, m *metrics.Collectors) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			dur := time.Since(start)
			route := routeLabel(r)
			reqID, _ := r.Context().Value(requestIDKey).(string)

			if m != nil {
				m.HTTPRequests.WithLabelValues(r.Method, route, http.StatusText(rec.status)).Inc()
				m.HTTPDuration.WithLabelValues(r.Method, route).Observe(dur.Seconds())
			}
			log.Info("http request",
				"method", r.Method, "path", r.URL.Path, "status", rec.status,
				"duration_ms", dur.Milliseconds(), "request_id", reqID)
		})
	}
}

// routeLabel reduces cardinality by using the matched pattern when available.
func routeLabel(r *http.Request) string {
	if p := r.Pattern; p != "" {
		return p
	}
	return r.URL.Path
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "req"
	}
	return hex.EncodeToString(b)
}
