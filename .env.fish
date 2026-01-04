# database connection string
set -gx DB_DSN "postgres://user_service_user:password123@localhost:5432/user_service?sslmode=disable"

# JWT secret (должен совпадать с auth-service!)
set -gx JWT_SECRET "/2+XnmJGz1j3ehIVI/5P9kl+CghrE3DcS7rnT+qar5w"

# auth service URL
set -gx AUTH_SERVICE_URL "http://localhost:8081"
set -gx AUTH_GRPC_ADDR "localhost:8084"
# port for this service
set -gx PORT 8082

set -gx AMQP_URL "amqp://guest:guest@localhost:5672/"
