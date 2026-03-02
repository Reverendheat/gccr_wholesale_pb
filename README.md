# Ground Control Wholesale

A PocketBase-backed invoicing app integrated with the Square API, fronted by a React + TypeScript UI.

## Stack

| Layer | Technology |
|---|---|
| Backend | [PocketBase](https://pocketbase.io) (Go) |
| Payments | [Square API](https://developer.squareup.com) |
| Frontend | React + TypeScript (Vite) |

## Project Structure

```
gccr_wholesale/
  cmd/server/main.go      # PocketBase entry point
  internal/square/        # Square API client
  pb_migrations/          # PocketBase schema migrations
  frontend/               # React app (Vite)
  .env.example            # Backend env vars
```

## Getting Started

### Backend

1. Copy and fill in environment variables:
   ```sh
   cp .env.example .env
   ```

2. Run the server:
   ```sh
   go run ./cmd/server serve
   ```
   PocketBase Admin UI is available at `http://127.0.0.1:8090/_/`.

### Frontend

1. Copy and fill in frontend environment variables:
   ```sh
   cp frontend/.env.example frontend/.env
   ```

2. Install dependencies and start dev server:
   ```sh
   cd frontend
   npm install
   npm run dev
   ```
   App runs at `http://localhost:5173`.

## Running Tests

```sh
go test ./...
```
