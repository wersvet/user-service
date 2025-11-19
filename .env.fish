# database connection string
set -gx DB_DSN "postgres://user_service_user:password123@localhost:5432/user_service?sslmode=disable"

# JWT secret (должен совпадать с auth-service!)
set -gx JWT_SECRET "SUPERSECRETKEY123"

# auth service URL
set -gx AUTH_SERVICE_URL "http://localhost:8081"

# port for this service
set -gx PORT 8082