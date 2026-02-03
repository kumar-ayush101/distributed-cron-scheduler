package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"github.com/kumar-ayush101/distributed-cron-scheduler/internal/models" 
	"github.com/robfig/cron/v3"
)

// newHandler returning a configured http handler
func NewHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		//health check

		if r.Method == "GET" && r.URL.Path == "/health" {
			if err := db.Ping(); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				w.Write([]byte("Database Down"))
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
			return
		}

		//  GET /jobs
		if r.Method == "GET" && r.URL.Path == "/jobs" {
			rows, err := db.Query("SELECT id, name, cron_schedule, next_run_at FROM jobs ORDER BY id DESC")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			var jobs []models.Job
			for rows.Next() {
				var j models.Job
				if err := rows.Scan(&j.ID, &j.Name, &j.CronSchedule, &j.NextRunAt); err != nil {
					continue
				}
				jobs = append(jobs, j)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(jobs)
			return
		}

		// POST /jobs
		if r.Method == "POST" && r.URL.Path == "/jobs" {
			var j models.Job
			if err := json.NewDecoder(r.Body).Decode(&j); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}
			parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
			schedule, err := parser.Parse(j.CronSchedule)
			if err != nil {
				http.Error(w, "Invalid Cron Schedule", http.StatusBadRequest)
				return
			}
			j.NextRunAt = schedule.Next(time.Now())

			query := `INSERT INTO jobs (name, cron_schedule, next_run_at) VALUES ($1, $2, $3) RETURNING id`
			err = db.QueryRow(query, j.Name, j.CronSchedule, j.NextRunAt).Scan(&j.ID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(j)
			return
		}

		//  DELETE /jobs
		if r.Method == "DELETE" {
			idStr := r.URL.Query().Get("id")
			_, err := db.Exec("DELETE FROM jobs WHERE id = $1", idStr)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
			return
		}

		// POST /jobs/run
		if r.Method == "POST" && r.URL.Path == "/jobs/run" {
			idStr := r.URL.Query().Get("id")
			_, err := db.Exec("UPDATE jobs SET next_run_at = $1 WHERE id = $2", time.Now(), idStr)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{"status": "scheduled_now"})
			return
		}

		// GET /jobs/history
		if r.Method == "GET" && r.URL.Path == "/jobs/history" {
			jobID := r.URL.Query().Get("job_id")
			query := "SELECT id, job_id, run_at, status, details FROM job_history"
			var rows *sql.Rows
			var err error

			if jobID != "" {
				query += " WHERE job_id = $1 ORDER BY run_at DESC LIMIT 50"
				rows, err = db.Query(query, jobID)
			} else {
				query += " ORDER BY run_at DESC LIMIT 50"
				rows, err = db.Query(query)
			}

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer rows.Close()

			history := []models.JobHistory{}
			for rows.Next() {
				var h models.JobHistory
				if err := rows.Scan(&h.ID, &h.JobID, &h.RunAt, &h.Status, &h.Details); err != nil {
					continue
				}
				history = append(history, h)
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(history)
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}