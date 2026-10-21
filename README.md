# Oslo

Oslo is a ride hailing backend written in Go that coordinates real time driver discovery, matching, offers, and trip progression from the initial request to completion.

## Core features

* Go `net/http` API with request IDs, deadlines, body limits, rate limiting, and graceful shutdown.
* PostgreSQL transactions, row locks, constraints, and idempotent ride creation.
* Redis geospatial driver discovery, reservations, stale location cleanup, and shared rate limiting.
* Transactional outbox delivery to SQS with worker retries and dead letter handling.
* Explicit ride and offer state transitions.
* Docker Compose for PostgreSQL, Redis, LocalStack SQS, migrations, API, worker, and publisher.
* CI checks for formatting, vetting, tests, race detection, builds, Compose, and the production image.

## Run locally

```bash
docker compose up --build
```

The API is available at `http://localhost:8080`. Compose starts migrations before the API, worker, and publisher services.

To run a smoke test:

```bash
k6 run k6/smoke.js
```

Other scenarios are documented in [k6/README.md](k6/README.md).

## AWS validation

The deployment uses ECS Fargate, ECR, a public ALB, RDS PostgreSQL, ElastiCache Redis, SQS, a dead letter queue, and CloudWatch logs.

The final AWS test used 400 distinct riders authenticated before the measured run and completed 800 ride/read requests with 0% HTTP failures and approximately 157 ms p95 latency.

Deployment details, AWS decisions, and test evidence are in [aws/overview.md](aws/overview.md) and [aws/iterations](aws/iterations).

