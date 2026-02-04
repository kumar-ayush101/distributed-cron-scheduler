# Distributed Cron Job Scheduler

![Go](https://img.shields.io/badge/Go-1.25-blue)
![Docker](https://img.shields.io/badge/Docker-Containerized-blue)
![Redis](https://img.shields.io/badge/Redis-Distributed%20Locking-red)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15-blue)
![CI](https://img.shields.io/github/actions/workflow/status/kumar-ayush101/distributed-cron-scheduler/ci.yml?branch=main)
![License](https://img.shields.io/badge/License-MIT-green)

A **horizontally scalable, distributed cron job scheduler** built with **Go**, designed to run jobs **exactly once** across multiple worker nodes using **Redis-based distributed locking**.

This system prevents duplicate execution in multi-node environments while supporting dynamic scheduling, fault tolerance, and execution history tracking.

---

## Table of Contents

- [System Architecture](#system-architecture)
- [Key Features](#key-features)
- [Tech Stack](#tech-stack)
- [Project Structure](#project-structure)
- [Getting Started](#getting-started)
- [Distributed Locking Explained](#distributed-locking-explained)
- [Execution Workflow](#execution-workflow)
- [Continuous Integration](#continuous-integration)
- [Future Improvements](#future-improvements)
- [License](#license)

---

## System Architecture

<img width="910" height="414" alt="System Architecture Diagram" src="https://github.com/user-attachments/assets/3382e302-98e6-4adf-b501-251976b564e5" />

The system runs as a **distributed cluster** consisting of:

- **API Nodes** – Handle job creation, updates, and scheduling
- **Worker Nodes** – Execute scheduled jobs
- **PostgreSQL** – Persistent storage for jobs and execution history
- **Redis** – Distributed locking & coordination

All nodes are **stateless**, enabling easy horizontal scaling.

---

## Key Features

- **Exactly-Once Job Execution**  
  Redis `SET NX` locks ensure no job runs more than once, even with multiple workers.

- **Horizontal Scalability**  
  Add more worker containers without changing application logic.

- **Fault Tolerance**  
  Redis locks use TTLs to automatically recover from node crashes.

- **Dynamic Scheduling**  
  Create, update, or delete jobs at runtime via REST API or Dashboard.

- **Execution History & Observability**  
  Full audit trail of job runs stored in PostgreSQL.

- **CI/CD Ready**  
  Automated testing and Docker build validation via GitHub Actions.

---

## Tech Stack

| Layer            | Technology |
|------------------|------------|
| Backend          | Go (Golang) 1.25 |
| Frontend         | React (Vite) |
| Database         | PostgreSQL 15 |
| Distributed Lock | Redis 7 |
| Infrastructure   | Docker, Docker Compose |
| CI/CD            | GitHub Actions |

---

## Project Structure

```text
/distributed-cron-scheduler
├── .github/workflows   # CI/CD pipelines
├── client              # React frontend
├── cmd/server          # Application entry point
├── internal
│   ├── api             # REST APIs & routing
│   ├── database        # DB connection & migrations
│   ├── models          # Domain models
│   └── scheduler       # Cron engine & Redis locking
├── Dockerfile          # Multi-stage build
└── docker-compose.yml  # Local cluster setup


```


# Getting Started
## Prerequisites
- Docker
- Docker Compose

## Installation
- Clone the repository:
```text
git clone https://github.com/kumar-ayush101/distributed-cron-scheduler.git
cd distributed-cron-scheduler
```
## Run the Cluster
- This command spins up: PostgreSQL, Redis, API Server, Multiple Worker Nodes

```
docker-compose up -d --build --scale worker=2
```

## Access the Dashboard
- Open your browser and navigate to:
```
http://localhost:5173
```

Use the UI to manage jobs and inspect execution history.

## Verify Services

Check that all containers are running:
```
docker ps
```

## Distributed Locking Explained

The biggest challenge in distributed scheduling is race conditions and double execution.
- Locking Strategy
1. Trigger

    Cron schedules fire on all worker nodes simultaneously.

2. Acquire Lock

    Each worker attempts to acquire a Redis lock:

3. Decision

    OK → Lock acquired → execute the job

    nil → Lock exists → skip execution

4. Fail Safety

    The lock TTL guarantees automatic release if a node crashes.

    This ensures exactly-once execution without a central coordinator.


## Execution Workflow   

<img width="968" height="779" alt="image" src="https://github.com/user-attachments/assets/25b6a18f-6199-4892-9459-580fd87408d8" />



## Continuous Integration
- CI is handled via GitHub Actions (.github/workflows/ci.yml).

- On every push to the main branch, the pipeline:
1. Sets up the Go environment
2. Starts PostgreSQL service containers
3. Builds the application
4. Runs unit and integration tests
5. Validates Docker image builds

## Future Improvements

- Leader election for optimized scheduling

- Retry and backoff strategies

- Metrics & alerting (Prometheus + Grafana)

- Kubernetes-native deployment (Helm)

- Role-based access control (RBAC)   


## License
- This project is licensed under the MIT License.







