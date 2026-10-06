# Feed API

A simple REST API built with [Gin](https://github.com/gin-gonic/gin) and PostgreSQL for managing vendor feed data.

## Prerequisites

- [Go](https://go.dev/dl/) 1.27 or later
- [PostgreSQL](https://www.postgresql.org/download/) (running locally or remotely)

## Project Structure

```
cmd/            Application entry point (main.go)
internal/
  config/       Environment/config loading
  database/     Database connection setup
  handler/      HTTP request handlers
  model/        Data models
```

## Setup

1. **Clone the repository**

   ```sh
   git clone <repository-url>
   cd feed-api
   ```

2. **Install dependencies**

   ```sh
   go mod download
   ```

3. **Configure environment variables**

   Copy the example file and update it with your own values:

   ```sh
   copy .env.example .env
   ```

   On macOS/Linux use `cp .env.example .env` instead.

   | Variable      | Description                  | Default     |
   |---------------|-------------------------------|-------------|
   | `DB_HOST`     | PostgreSQL host               | `localhost` |
   | `DB_PORT`     | PostgreSQL port                | `5432`      |
   | `DB_USER`     | PostgreSQL username            | `postgres`  |
   | `DB_PASSWORD` | PostgreSQL password            | *(none)*    |
   | `DB_NAME`     | PostgreSQL database name       | `postgres`  |
   | `SERVER_PORT` | Port the API server listens on | `8080`      |

4. **Create the database and table**

   Make sure the database referenced by `DB_NAME` exists, then create the `vendors` table:

   ```sql
   CREATE TABLE vendors (
       id          SERIAL PRIMARY KEY,
       name        VARCHAR(255) NOT NULL,
       external_id VARCHAR(255) NOT NULL UNIQUE,
       created_at  TIMESTAMP NOT NULL DEFAULT NOW()
   );
   ```

## Running the API

```sh
go run ./cmd/main.go
```

The server will start on the port specified by `SERVER_PORT` (default `8080`) and log a confirmation once it has connected to PostgreSQL.

## API Endpoints

### Create a vendor

```
POST /vendors
Content-Type: application/json
```

Request body:

```json
{
  "name": "Acme Corp",
  "external_id": "acme-001"
}
```

Example using `curl`:

```sh
curl -X POST http://localhost:8080/vendors \
  -H "Content-Type: application/json" \
  -d "{\"name\": \"Acme Corp\", \"external_id\": \"acme-001\"}"
```

Responses:

- `201 Created` – vendor created successfully
- `400 Bad Request` – missing/invalid `name` or `external_id`
- `409 Conflict` – a vendor with the given `external_id` already exists
- `500 Internal Server Error` – database error

## Building a binary

```sh
go build -o bin/feed-api ./cmd/main.go
./bin/feed-api
```
