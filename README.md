# distributed-cron-scheduler
Distributed Cron Job Scheduler
A horizontally scalable, distributed job scheduling system built with Go. This project implements a robust distributed locking mechanism using Redis to ensure idempotent job execution across a cluster of worker nodes, preventing duplicate runs in a multi-node environment.

System Architecture
The system is designed as a distributed cluster consisting of API nodes and Worker nodes. All nodes share a common persistence layer (PostgreSQL) and a synchronization layer (Redis).


