# Go HTTP Service

A task management service with **REST and gRPC APIs**, built with **Go, MySQL, Docker Compose, and Prometheus**.

A hands-on project exploring backend engineering concepts, including HTTP routing, middleware, concurrency, database persistence, graceful shutdown, automated testing, and containerisation.

## Features

- RESTful CRUD API using Go's standard `net/http` package
- Protocol Buffers and gRPC endpoints for retrieving one task or listing tasks
- MySQL database persistence
- Partial task updates using HTTP PATCH
- Request validation and structured error handling
- Request-scoped contexts and database timeouts
- HTTP middleware for structured request logging, panic recovery, and Prometheus metrics
- Liveness and database readiness endpoints
- Graceful server shutdown
- Unit and integration tests with Go's race detector
- Multi-stage Docker builds
- Docker Compose with persistent MySQL storage
- Versioned SQL database migrations

## Project Structure

```text
http_service/
├── cmd/
│   └── server/          # Application entry point
├── gen/task/v1/         # Generated protobuf messages and gRPC bindings
├── internal/
│   ├── config/          # Environment configuration
│   ├── database/        # MySQL connection management
│   ├── grpcserver/      # gRPC service implementation and integration tests
│   ├── middleware/      # HTTP logging, panic recovery and metrics
│   └── task/            # Task models, handlers, router and repository
├── proto/task/v1/       # Task service protobuf definition
├── migrations/          # Versioned SQL migrations
├── monitoring/
│   └── prometheus.yml   # Prometheus scrape configuration
├── Dockerfile
├── compose.yaml
├── go.mod
└── go.sum
```

## Getting Started

### Prerequisites

- Docker with Docker Compose
- Go (optional, required for running locally or executing tests)

### 1. Clone the repository

```bash
git clone https://github.com/Chibas/http_service.git
cd http_service
```

### 2. Configure environment variables

Create a `.env` file in the project root:

```dotenv
MYSQL_ROOT_PASSWORD=your_local_password
MYSQL_APP_USER=your_app_user
MYSQL_APP_PASSWORD=your_app_password
```

These credentials are used by the local Docker Compose environment. The API uses the application account, which must already exist in MySQL and have access to `task_service`.

The migration runner uses the root account. Do not commit `.env` to Git.

### 3. Start the application

Build and start the Go API, MySQL database, migration runner, and Prometheus:

```bash
docker compose up -d --build
```

Docker Compose automatically:

- Starts MySQL 8.4 with persistent storage.
- Waits for the database to become healthy.
- Applies pending database migrations.
- Starts the Go API after migrations complete.
- Starts Prometheus, configured to scrape the API every 15 seconds.

The API will be available at:

`http://localhost:8000`

Verify the containers:

```bash
docker compose ps -a
```

The migration container is expected to exit successfully after applying migrations.

## API Endpoints

| Method | Endpoint | Description |
|---|---|---|
| GET | `/tasks` | Retrieve all tasks |
| GET | `/tasks/{id}` | Retrieve a task by ID |
| POST | `/tasks` | Create a new task |
| PATCH | `/tasks/{id}` | Partially update a task |
| DELETE | `/tasks/{id}` | Delete a task |
| GET | `/healthz` | Liveness check; returns 200 with `OK` without querying MySQL |
| GET | `/readyz` | Database readiness check; returns 200 with `OK` or 503, using a two-second timeout |
| GET | `/metrics` | Expose metrics for Prometheus |

## gRPC API

The same application runs `task.v1.TaskService` on port `50051`, using the same MySQL repository as the REST API.

| RPC | Request | Response |
|---|---|---|
| `GetTask` | `GetTaskRequest` with an `id` | One `TaskResponse` |
| `GetTasks` | Empty `GetTasksRequest` | `TasksResponse` containing repeated tasks |

The schema is in `proto/task/v1/task.proto`; generated Go bindings are committed under `gen/task/v1`. The gRPC integration tests use an in-memory `bufconn` listener and a MySQL test database.

### Run and call the service locally

The current Compose configuration publishes HTTP port `8000` only. To use the commands below, run the Go application locally with Go installed and a reachable MySQL database. Stop the Compose API first if it is using port `8000`.

Set the application's database environment variables, using an existing database account:

```bash
export DB_HOST=127.0.0.1:3306
export DB_USER=your_app_user
export DB_PASSWORD=your_app_password
export DB_NAME=task_service
go run ./cmd/server
```

Use the actual address of your MySQL instance for `DB_HOST`. The Compose MySQL service currently does not publish port `3306` to the host. Apply the SQL migrations before starting the application.

With `grpcurl` installed, run these commands from the repository root in another terminal:

```bash
grpcurl -plaintext \
  -import-path proto \
  -proto task/v1/task.proto \
  -d '{"id": "1"}' \
  localhost:50051 task.v1.TaskService/GetTask
```

```bash
grpcurl -plaintext \
  -import-path proto \
  -proto task/v1/task.proto \
  -d '{}' \
  localhost:50051 task.v1.TaskService/GetTasks
```

The service currently uses plaintext gRPC. Supplying the proto file lets `grpcurl` call it without server reflection. In the JSON output, `int64` IDs appear as strings, and default-valued fields such as `completed: false` may be omitted.

HTTP request metrics described below instrument the REST API; they do not count gRPC calls.

## Integration tests

Tests use two separate MySQL databases so HTTP and gRPC packages can run concurrently:

```dotenv
DB_TEST_NAME=task_service_test
DB_GRPC_TEST_NAME=task_service_grpc_test
```

Set these in the root `.env` file along with `DB_USER`, `DB_PASSWORD`, and `DB_HOST`. Create both databases and apply the existing `migrations/*.up.sql` files to each in filename order. Use disposable test databases: the gRPC empty-list test deletes all tasks from its database.

Run the full suite repeatedly with the race detector:

```bash
go test -race -count=10 ./...
```

GitHub Actions creates and migrates both databases before running the same command.

## Monitoring with Prometheus

The metrics middleware records:

| Metric | Labels | Description |
|---|---|---|
| `http_requests_total` | `method`, `status` | Total requests handled |
| `http_request_duration_seconds` | `method` | Request duration histogram in seconds, using Prometheus default buckets |

Requests to `/metrics` are excluded from these two metrics so scrapes do not count as application requests. Health and readiness requests are included.

### Check metrics

Generate a request and inspect the API's metrics endpoint:

```bash
curl http://localhost:8000/tasks
curl http://localhost:8000/metrics
```

Open the [Prometheus UI](http://localhost:9090) and check the [targets page](http://localhost:9090/targets). The `go-api` target should show `UP`. Prometheus reaches the API at `api:8000` over the Compose network, using the configuration in `monitoring/prometheus.yml`.

Allow at least one scrape interval after generating requests, then try these queries in the Prometheus UI:

```promql
http_requests_total
```

```promql
sum by (method, status) (rate(http_requests_total[5m]))
```

The second query shows requests per second by method and status; it needs at least two scrapes to calculate a rate.

To inspect Prometheus logs:

```bash
docker compose logs -f prometheus
```

