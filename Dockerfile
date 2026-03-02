# Stage 1: Build the React frontend
FROM node:20-alpine AS frontend
WORKDIR /app/frontend
# NODE_ENV=development ensures devDependencies (tsc, vite, etc.) are installed.
ENV NODE_ENV=development
# Expose local bin so tsc/vite are always resolvable regardless of npm PATH behaviour.
ENV PATH=/app/frontend/node_modules/.bin:$PATH
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
# Build the production bundle; vite sets NODE_ENV=production internally.
RUN tsc -b && vite build

# Stage 2: Build the Go binary
FROM golang:1.24-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server ./cmd/server

# Stage 3: Minimal runtime image
FROM alpine:3.21
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata

# Copy the binary and the built frontend (served via PocketBase --publicDir)
COPY --from=backend /app/server ./server
COPY --from=frontend /app/frontend/dist ./pb_public

EXPOSE 8090

# pb_data holds the SQLite database and PocketBase uploads — mount this as a volume
VOLUME ["/app/pb_data"]

CMD ["./server", "serve", "--http", "0.0.0.0:8090"]
