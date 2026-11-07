# production-mimic validation

The live stack was validated in `ap-south-1` with the existing fixed deployment: API 4/4, worker 4/4, and publisher 2/2.

## combined hot path

The mixed workload ran for 10 minutes at 20 ride creations/sec, 100 driver-location updates/sec, and five concurrent ride-status readers.

- 74,919/74,919 checks passed
- approximately 120.5 requests/sec
- p95 HTTP latency: 32.1 ms
- no business-check failures
- SQS and the dead-letter queue returned to zero
- ECS remained API 4/4, worker 4/4, publisher 2/2

The exercised path was:

```text
ALB -> API -> PostgreSQL/Redis -> transactional outbox -> publisher -> SQS -> worker -> matching
```

The generic k6 `http_req_failed` metric includes expected `409` responses for riders with an active ride. The business checks accepted the documented `201`/`409` behavior; ALB CloudWatch showed no target 5xx datapoints.

## matching contention

Two hundred concurrent riders competed for a small driver pool.

- 600/600 correctness checks passed
- 0% HTTP failures
- p95 HTTP latency: about 101 ms
- no duplicate-match signal was observed

## resource observations

CloudWatch observations during the validation window:

- RDS CPU: approximately 4-6.4% maximum
- Redis engine CPU: approximately 0.30% maximum
- Redis evictions: 0
- ECS API CPU: below 0.5% maximum in the sampled service metric
- ECS API memory: below 0.25% maximum in the sampled service metric
- ALB target response time: approximately 1.3 ms maximum in the sampled datapoints
- ALB target 5xx: no datapoints

## recovery

One worker task was stopped intentionally. ECS replaced it and the worker service returned to 4/4. The main queue and dead-letter queue remained at zero after recovery, and `/healthz` returned 200.

This establishes tested capacity for the stated workload; it does not claim unlimited throughput or multi-region recovery.
