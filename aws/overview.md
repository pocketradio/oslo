# aws overview

## architecture

one Docker image runs three ECS services through different commands:

```text
ALB -> API tasks -> PostgreSQL / Redis
                         |
                 transactional outbox
                         v
                 publisher -> SQS -> worker -> matching
```

- API serves public HTTP traffic.
- publisher sends committed outbox events to SQS.
- worker consumes SQS messages and performs matching.
- PostgreSQL stores users, rides, offers, and outbox events.
- Redis stores driver locations, reservations, and the shared rate-limit counter.

## current verified deployment

- region: `ap-south-1`
- cluster: `oslo-cluster`
- API: 4 running tasks, `oslo-api-runtime:4`, 1 vCPU, 2 GiB
- worker: 4 running tasks, `oslo-worker-runtime:2`
- publisher: 2 running tasks, `oslo-publisher-runtime:2`
- ALB target group: `oslo-api-tg`, API port 8080 only
- data services: RDS `oslo-postgres`, ElastiCache `oslo-redis-001`
- queue: `oslo-matching`
- dead-letter queue: `oslo-matching-dead-letter`

the image is stored in ECR as `oslo-api:9d9d991`. secrets are injected into ECS rather than written into this documentation.

## verified networking

- VPC: `vpc-0950bf2cfed999cd0`
- ECS subnets: `subnet-00a7e0cfaf86ea167` (`ap-south-1a`, `10.0.0.0/20`) and `subnet-04c189ab06a3ab9bb` (`ap-south-1b`, `10.0.16.0/20`)
- ECS security group: `oslo-ecs-sg` (`sg-0861a34dce88850b2`)
- ALB security group: `oslo-alb-sg` (`sg-0beacc53168e11d77`)
- ALB: internet-facing, HTTP 80, DNS `oslo-alb-1941347414.ap-south-1.elb.amazonaws.com`
- API target group: HTTP 8080, health path `/healthz`, IP targets, all four targets healthy at verification

the ECS security group allows TCP 8080 only from the ALB security group. the ALB allows HTTP 80 from `0.0.0.0/0`. the ECS group allows all outbound traffic.

all three ECS services currently use both ECS subnets and `assignPublicIp=ENABLED`. worker and publisher have no ALB, but their tasks can receive public IPs. this is functional but does not meet a strict private-subnet deployment design.

RDS is `db.t4g.micro`, not publicly accessible, and uses `oslo-data-sg` (`sg-0bc99d0174b2c6593`) for ECS access on TCP 5432. Redis is `cache.t4g.micro` and uses the same data security group for TCP 6379. the data group allows those ports only from `oslo-ecs-sg`.

## verified IAM permissions

- all task definitions use `ecsTaskExecutionRole` with the managed `AmazonECSTaskExecutionRolePolicy`.
- API task role has no attached or inline policies returned by the check.
- worker role has `oslo-worker-queue-access`: receive, delete, change visibility, and inspect the matching SQS queue.
- publisher role has `oslo-publisher-queue-access`: send messages and inspect the matching SQS queue.
- worker and publisher policies target only `oslo-matching`.

## runtime behavior

- API starts HTTP middleware, routes, PostgreSQL, Redis, and graceful shutdown.
- worker long-polls SQS with five handler goroutines per task.
- successful messages are deleted using their receipt handles.
- failed messages remain available for redelivery.
- publisher claims outbox rows with `FOR UPDATE SKIP LOCKED` and retries failed sends.
- the current SQS queue uses a 60-second visibility timeout, 20-second long polling, and dead-letters after 3 receives.

## deployment limitations

- CI verifies code, tests, race detection, Compose, and Docker builds. it does not deploy AWS resources.
- worker and publisher have no ALB attachment, but their current public-IP assignment should be corrected if strict private networking is required.
- the deployment has no verified autoscaling or multi-region failover.
