# Stage 1: Compile your Go application
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o server ./cmd/server

# Stage 2: Run the compiled application
FROM alpine:3.22

WORKDIR /app

COPY --from=builder /app/server ./server

ENTRYPOINT ["./server"]
