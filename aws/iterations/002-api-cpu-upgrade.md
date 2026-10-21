# api cpu upgrade

the API moved from 0.5 vCPU / 1 GiB to 1 vCPU / 2 GiB.

five tasks exceeded the available Fargate quota, so the service currently runs four API tasks. worker and publisher remained at four and two tasks.

the 10-vCPU quota request remains `CASE_OPENED`. its scheduler was removed.

the replacement used API task definition revision 4 with 1024 CPU units and 2048 MiB of memory. the image and application configuration stayed the same.

the old API tasks were stopped before the replacement was started, so this was an intentional outage-based change rather than a zero-downtime rollout.

the four new API targets became healthy behind the ALB before testing continued. revision 3 remains available as the rollback task definition.
