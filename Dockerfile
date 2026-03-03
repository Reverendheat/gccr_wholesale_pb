# Stage 1: Build the React frontend
FROM node:20-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci --include=dev
COPY frontend/ ./
RUN ./node_modules/.bin/tsc -b && ./node_modules/.bin/vite build

# Stage 2: Build the Go binary
FROM golang:1.24-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# GOGC=off disables the GC during the build, reducing peak RSS on small instances.
# GOMAXPROCS=1 avoids parallel compilation spikes that can OOM a 1-2GB server.
RUN CGO_ENABLED=0 GOOS=linux GOGC=off GOMAXPROCS=1 \
    go build -ldflags="-s -w" -o server ./cmd/server

# Stage 3: Minimal runtime image
FROM alpine:3.21
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata wget

# Copy the binary and the built frontend (served via PocketBase --publicDir)
COPY --from=backend /app/server ./server
COPY --from=frontend /app/frontend/dist ./pb_public

EXPOSE 8090

# pb_data holds the SQLite database and PocketBase uploads — mount this as a volume
VOLUME ["/app/pb_data"]

CMD ["./server", "serve", "--http", "0.0.0.0:8090"]
