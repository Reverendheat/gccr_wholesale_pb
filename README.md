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

1. Copy and fill in the backend and frontend environment files:

   ```sh
   cp .env.example .env
   cp frontend/.env.example frontend/.env
   ```

2. Install frontend dependencies:

   ```sh
   npm --prefix frontend install
   ```

3. Start the complete development stack:

   ```sh
   just dev
   ```

   This starts PocketBase, Vite, and the local Mailpit OTP catcher:

   - Application: `http://localhost:5173`
   - PocketBase Admin: `http://127.0.0.1:8090/_/`
   - OTP inbox: `http://localhost:8025`

Request an OTP for an existing local staff or customer email and read it in the
Mailpit inbox. Stopping `just dev` also stops Mailpit. Mailpit exists only in
`docker-compose.dev.yml`; the production image and Compose stack do not include
it.

To preview the customer delivery promise against a fixed date, sign in locally
as a customer and open `/portal?deliveryNow=<ISO-8601 timestamp>`. This override
is available only in the Vite development build. For example, compare Monday
and Tuesday around the weekly cutoff:

```text
http://localhost:5173/portal?deliveryNow=2026-08-24T20:00:00-04:00
http://localhost:5173/portal?deliveryNow=2026-08-25T12:00:00-04:00
```

## Database Migrations

Go migrations in `pb_migrations/` are embedded in the server binary and run automatically before the HTTP server starts. PocketBase sorts migration filenames lexicographically and records applied filenames in the database.

- Add a new migration file for every schema change; never edit an applied migration.
- Use timestamp filenames (`<unix_timestamp>_<description>.go`) so new migrations sort after existing files.
- Test migrations against an empty data directory as well as existing data before deployment.

## Running Tests

```sh
go test ./...
```
