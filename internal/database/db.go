package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"
	
	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
)

// connectPostgres handling the retry logic and connection
func ConnectPostgres() *sql.DB {
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" { dbHost = "localhost" }
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" { dbPort = "5432" }
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" { dbUser = "postgres" }
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" { dbPassword = "secret" }
	dbName := os.Getenv("DB_NAME")
	if dbName == "" { dbName = "postgres" }

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)
	
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	// retry logic
	for i := 0; i < 5; i++ {
		if err := db.Ping(); err == nil {
			fmt.Println("Connected to Postgres!")
			return db
		}
		fmt.Println("Waiting for DB...")
		time.Sleep(2 * time.Second)
	}
	log.Fatal("Could not connect to Postgres after retries")
	return nil
}

// connectRedis handling redis connection
func ConnectRedis() *redis.Client {
	redisHost := os.Getenv("REDIS_HOST")
	if redisHost == "" { redisHost = "localhost" }
	redisPort := os.Getenv("REDIS_PORT")
	if redisPort == "" { redisPort = "6379" }

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%s", redisHost, redisPort),
	})
	
	fmt.Println("Connected to Redis")
	return rdb
}

// InitSchema creating the tables
func InitSchema(db *sql.DB) {
	queryJobs := `
	CREATE TABLE IF NOT EXISTS jobs (
		id SERIAL PRIMARY KEY,
		name TEXT NOT NULL,
		cron_schedule TEXT NOT NULL,
		next_run_at TIMESTAMP NOT NULL
	);`
	if _, err := db.Exec(queryJobs); err != nil {
		log.Fatal("Failed to create jobs table:", err)
	}

	queryHistory := `
	CREATE TABLE IF NOT EXISTS job_history (
		id SERIAL PRIMARY KEY,
		job_id INTEGER REFERENCES jobs(id) ON DELETE CASCADE,
		run_at TIMESTAMP NOT NULL,
		status TEXT NOT NULL,
		details TEXT
	);`
	if _, err := db.Exec(queryHistory); err != nil {
		log.Fatal("Failed to create job_history table:", err)
	}
}

//seedJobs inserting initial data
func SeedJobs(db *sql.DB) {
	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM jobs WHERE name = $1", "Send Email").Scan(&count)
	if count == 0 {
		_, _ = db.Exec(`INSERT INTO jobs (name, cron_schedule, next_run_at) VALUES ($1, $2, $3)`,
			"Send Email", "*/1 * * * *", time.Now())
		fmt.Println("Inserted job: 'Send Email'")
	}

	_ = db.QueryRow("SELECT COUNT(*) FROM jobs WHERE name = $1", "Database Backup").Scan(&count)
	if count == 0 {
		_, _ = db.Exec(`INSERT INTO jobs (name, cron_schedule, next_run_at) VALUES ($1, $2, $3)`,
			"Database Backup", "*/2 * * * *", time.Now())
		fmt.Println("Inserted job: 'Database Backup'")
	}
}