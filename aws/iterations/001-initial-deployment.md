# initial deployment

one Docker image was deployed to ECS Fargate with three commands:

- `api`: public HTTP service behind the ALB.
- `worker`: SQS consumer and matching process.
- `publisher`: transactional outbox publisher.

RDS PostgreSQL, ElastiCache Redis, SQS, ECR, CloudWatch logs, and the ALB were connected manually. no infrastructure-as-code exists in the repository.

the API is the only service attached to the ALB and listens on container port 8080. worker and publisher run the same binary with different ECS commands and do not expose HTTP endpoints.

the Dockerfile builds one stripped Go binary, runs it as a non-root user, and uses `api` as the default command. ECS overrides that command for the worker and publisher services.

this deployment separates public request handling from asynchronous matching and outbox delivery while keeping one codebase and one image.
