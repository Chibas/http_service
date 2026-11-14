# Go HTTP Service

A RESTful task management API built with **Go, MySQL, Docker Compose, and Prometheus**.

A hands-on project exploring backend engineering concepts, including HTTP routing, middleware, concurrency, database persistence, graceful shutdown, automated testing, and containerisation.

## Features

- RESTful CRUD API using Go's standard `net/http` package
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
├── internal/
│   ├── config/          # Environment configuration
│   ├── database/        # MySQL connection management
│   ├── middleware/      # HTTP logging, panic recovery and metrics
│   └── task/            # Task models, handlers, router and repository
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

