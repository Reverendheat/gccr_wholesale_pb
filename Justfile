set dotenv-load

default: dev

# Start both the PocketBase backend and the React dev server
dev:
    #!/usr/bin/env bash
    set -e
    go run ./cmd/server serve &
    BACKEND_PID=$!
    trap "kill $BACKEND_PID 2>/dev/null" EXIT
    cd frontend && npm run dev

# PocketBase only
backend:
    go run ./cmd/server serve

# React dev server only
frontend:
    cd frontend && npm run dev

# Run all Go tests
test:
    go test ./...

# Build the Go binary
build:
    go build -o bin/server ./cmd/server

# Build and start the production Docker stack
up:
    docker compose up -d --build

# Tear down the production Docker stack (keeps volumes)
down:
    docker compose down

# Tail logs from the running stack
logs:
    docker compose logs -f
