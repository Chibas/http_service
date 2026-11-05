# Go HTTP Service

A RESTful task management API built with **Go, MySQL, and Docker Compose**.

A hands-on project exploring backend engineering concepts, including HTTP routing, middleware, concurrency, database persistence, graceful shutdown, automated testing, and containerisation.

## Features

- RESTful CRUD API using Go's standard `net/http` package
- MySQL database persistence
- Partial task updates using HTTP PATCH
- Request validation and structured error handling
- Request-scoped contexts and database timeouts
- HTTP middleware for request logging and panic recovery
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
│   ├── middleware/      # HTTP logging and panic recovery
│   └── task/            # Task models, handlers, router and repository
├── migrations/          # Versioned SQL migrations
├── practice/            # Standalone Go concurrency exercises
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
```

This password is used by the local Docker Compose environment.

**Security:** The root account is currently used for local development only. Do not commit `.env` to Git or use this configuration for production deployments.

### 3. Start the application

Build and start the Go API, MySQL database and migration runner:

```bash
docker compose up -d --build
```

Docker Compose automatically:

- Starts MySQL 8.4 with persistent storage.
- Waits for the database to become healthy.
- Applies pending database migrations.
- Starts the Go API after migrations complete.

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

