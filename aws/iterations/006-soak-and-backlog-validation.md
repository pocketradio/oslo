# soak and backlog validation

## ten-minute soak

The combined workload was repeated for a separate 10-minute soak at 20 rides/sec, 100 location updates/sec, and five status readers.

- 74,919/74,919 checks passed
- approximately 120.5 requests/sec
- p95 HTTP latency: 32.1 ms
- seven dropped iterations across the run
- ECS remained API 4/4, worker 4/4, publisher 2/2
- all four ALB targets remained healthy
- `/healthz` returned 200
- main SQS queue: 0 visible, 0 in-flight
- dead-letter queue: 0

## bounded overload attempt

A 30-second burst was run using 300 pre-authenticated riders and a 300 ride-attempts/sec target. The harness was corrected to rotate through the full rider cohort and to use a broad synthetic source-address range.

- 9,038 iterations completed
- 9,058/9,058 business checks passed
- 3 dropped iterations
- SQS visible depth: 0
- SQS in-flight depth: 0 at the sampled checkpoint
- dead-letter queue: 0
- ECS and ALB remained healthy

The generic k6 failed-request metric was approximately 84% during this burst because the production API rate limiter rejected excess traffic before it could reach the outbox/SQS path. This is protection behavior, not evidence of worker or queue failure.

Therefore the exact worker saturation threshold remains unmeasured. A future dedicated backlog test should use a pre-seeded authenticated producer that is explicitly exempted from the API rate limiter, or a controlled worker-capacity fault injection. Neither was applied to the live stack during this validation.

## conclusion

The stack passed the production-mimic soak and remained protected under an aggressive burst. The tested capacity is the mixed-workload rate documented in iteration 005; the theoretical queue/worker maximum is intentionally not claimed.
