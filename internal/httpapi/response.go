// Package httpapi wires together the HTTP handlers, middleware and router.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/models"
	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/repository"
)

// errorResponse is the standard error envelope.
type errorResponse struct {
	Error string `json:"error"`
}

// writeJSON serialises v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// writeError writes a JSON error with the given status code.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// writeDomainError maps domain/validation errors to appropriate status codes.
func writeDomainError(w http.ResponseWriter, log *slog.Logger, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		writeError(w, http.StatusNotFound, "resource not found")
	case errors.Is(err, models.ErrEmptyName),
		errors.Is(err, models.ErrNameTooLong),
		errors.Is(err, models.ErrEmptyCron),
		errors.Is(err, models.ErrInvalidCron),
		errors.Is(err, models.ErrInvalidState):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		log.Error("internal error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}
