# user-service

## Environment variables

Required for core service:

- `DB_DSN`
- `JWT_SECRET`
- `AUTH_GRPC_ADDR`
- `PORT`

Telemetry (RabbitMQ audit events):

- `AMQP_URL` (e.g. `amqp://guest:guest@localhost:5672/`)
- `LOGS_EXCHANGE` (e.g. `logs.events`)
- `SERVICE_NAME` (e.g. `user-service`)
- `ENVIRONMENT` (e.g. `local`)

Audit events are published to the `LOGS_EXCHANGE` topic exchange using routing key `audit.friends`.
