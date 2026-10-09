# ride burst

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
