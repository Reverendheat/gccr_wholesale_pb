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
  docs/                   # Deployment and role guides
  .env.example            # Backend env vars
```

## Documentation

- [Deployment and operations](docs/deployment.md)
- [Administrator onboarding](docs/admin-onboarding.md)
- [Staff onboarding](docs/staff-onboarding.md)
- [Customer guide](docs/customer-guide.md)
- [Documentation index and role overview](docs/README.md)

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

## Database Migrations

Go migrations in `pb_migrations/` are embedded in the server binary and run automatically before the HTTP server starts. PocketBase sorts migration filenames lexicographically and records applied filenames in the database.

- Add a new migration file for every schema change; never edit an applied migration.
- Use timestamp filenames (`<unix_timestamp>_<description>.go`) so new migrations sort after existing files.
- Test migrations against an empty data directory as well as existing data before deployment.

## Running Tests

```sh
go test ./...
```
