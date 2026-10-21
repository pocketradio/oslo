# ride path validation

400 unique riders were created and logged in before measurement. each VU used its own token for one ride creation and one ride read.

- 400 VUs, 800 requests
- 800/800 checks passed
- 0% HTTP failures
- p95 latency: about 157 ms
- SQS and dead-letter queue drained to 0
- ECS remained API 4/4, worker 4/4, publisher 2/2

setup traffic was throttled and excluded from the measured metrics. tokens were never reused between measured riders, avoiding the one-active-ride-per-rider constraint.

ride creation still wrote PostgreSQL state and the transactional outbox. background processing then exercised the publisher, SQS, worker, Redis, and matching flow.

this final validation separated the earlier authentication bottleneck from the core ride path. the deployed system handled the tested 400-user workload cleanly, while the result does not claim unlimited capacity.
