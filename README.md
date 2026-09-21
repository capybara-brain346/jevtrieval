# jevtrieval

## Local setup

Requirements: Go 1.26+ and Docker with the Compose plugin. PostgreSQL is run only in Docker.

```sh
cp .env.example .env
set -a
. ./.env
set +a

docker info
docker compose config
docker compose up -d --wait postgres
docker compose ps
docker compose exec postgres pg_isready -U jevtrieval -d jevtrieval
```

The database is available at the `DATABASE_URL` in `.env`. Compose stores its data in the `postgres_data` volume.

## Backend commands

Run these from the repository root:

```sh
# Apply the idempotent schema migration.
docker compose exec -T postgres \
  psql -U jevtrieval -d jevtrieval \
  < db/migrations/001_init.sql

# Import and embed the SciFact corpus (use -replace to change embedding models).
go run ./cmd/import-scifact

# Run the test suite and static checks.
go test ./...
go vet ./...

# Run an offline evaluation when SciFact qrels are available.
go run ./cmd/eval

# Start the HTTP API.
go run ./cmd/server
```

The API listens on `HTTP_ADDR` (default `:8080`). The frontend origin is controlled by `ALLOWED_ORIGIN`; do not use a wildcard origin.
