# ride burst

## Full functional workflow

```bash
k6 run --env BASE_URL=http://localhost:8080 k6/functional-workflow.js
```

This covers authentication, fare estimation, driver location, ride creation, idempotent duplicate creation, offer polling, acceptance, arrival, start, completion, and final ride retrieval.

## Cancellation and idempotency

```bash
k6 run --env BASE_URL=http://localhost:8080 k6/cancellation-idempotency.js
```

## Offer rejection and retry

```bash
k6 run --env BASE_URL=http://localhost:8080 k6/offer-retry.js
```

## Run the repeatable suite

```powershell
./k6/run-suite.ps1 -BaseURL http://localhost:8080
```

For sustained performance runs:

```powershell
./deploy/run-performance.ps1 -BaseURL http://localhost:8080 -RideRate 20 -LocationRate 100 -Duration 10m
```

Run :

```bash
k6 run k6/ride-burst.js
```

k6 automatically reads the exported `options` object and uses it to schedule the default func.

by default, the test sends about five ride requests per second for 30 seconds. `preAllocatedVUs` is the starting number of virtual users, while `maxVUs` is the upper limit k6 can use to maintain the target rate.

The workload can be changed without editing the file:

eg

```bash
RATE=20 DURATION=60s k6 run k6/ride-burst.js
```

## 200-user matching test results

```bash
VUS=200 k6 run k6/matching-race.js
```

the local run used 200 concurrent riders competing for two drivers.

- 200 rides created
- 600/600 checks passed
- 0% http failures
- 2.24s p95 latency
