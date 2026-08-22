set dotenv-load

default: dev

# Start PocketBase, the React dev server, and the local OTP inbox
dev:
    #!/usr/bin/env bash
    set -e
    BACKEND_PID=""
    cleanup() {
        if [ -n "$BACKEND_PID" ]; then
            kill "$BACKEND_PID" 2>/dev/null || true
        fi
        docker compose -f docker-compose.dev.yml down >/dev/null 2>&1 || true
    }
    trap cleanup EXIT
    trap 'exit 130' INT
    trap 'exit 143' TERM

    docker compose -f docker-compose.dev.yml up -d --wait mailpit
    echo "Mailpit inbox: http://localhost:8025"

    PB_SMTP_ENABLED=true PB_SMTP_HOST=127.0.0.1 PB_SMTP_PORT=1025 PB_SMTP_TLS=false \
        go run ./cmd/server serve &
    BACKEND_PID=$!
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
