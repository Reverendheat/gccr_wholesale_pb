# Stage 1: Build the Go binary
FROM golang:1.24-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOGC=off GOMAXPROCS=1 \
    go build -ldflags="-s -w" -o server ./cmd/server

# Stage 2: Minimal runtime image
FROM alpine:3.21
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata wget

COPY --from=backend /app/server ./server
COPY frontend/dist ./pb_public

EXPOSE 8090

VOLUME ["/app/pb_data"]

CMD ["./server", "serve", "--http", "0.0.0.0:8090"]
