# auth load test

400 VUs performed registration and login, producing 800 requests.

- 19.5% failed
- average latency: 4.87 seconds
- p95 latency: 9.62 seconds

authentication was CPU-heavy. bcrypt password hashing is the primary code-level suspect.

the test deliberately combined registration and login so it measured password hashing, user persistence, credential lookup, and token issuance together.

it did not exercise ride creation, Redis driver matching, SQS delivery, or the outbox publisher. therefore the result isolates authentication rather than representing normal ride traffic.

the practical responses are controlled login rates, token reuse after authentication, and separate CPU capacity if authentication becomes a distinct production bottleneck.
