# ENIE database setup without installing PostgreSQL locally

ENIE uses PostgreSQL-specific SQL. Go alone is not a database, so a running
PostgreSQL server is required. You do not need `sudo`, Docker, or a local
PostgreSQL installation if you use a hosted PostgreSQL database.

## Option A: reuse the database already connected to the deployed ENIE app

1. Open the hosting dashboard for the existing ENIE deployment (for example,
   the Render service dashboard).
2. Open the service's Environment settings and locate `DATABASE_URL`. Do not
   paste the value into chat or commit it to Git.
3. If the value is an internal-only database URL, it will not work from your
   Ubuntu computer. Use the database provider's external/public connection URL
   for local development, if the provider offers one.
4. Copy `.env.example` to `.env` in the project root and put the external URL
   into `DATABASE_URL`.
5. Start the app from the project root with `go run .`.

## Option B: use a new hosted PostgreSQL database

Create a PostgreSQL database with a provider you trust, copy its connection
URL into `.env`, and start ENIE from the project root. On an empty database,
ENIE applies the checked-in `schema.sql` before running the additive migrations.
Back up any existing database before changing a production environment.

## Local environment file

```bash
cp .env.example .env
nano .env
```

Replace the placeholder `DATABASE_URL` with the actual URL from your provider.
The placeholders do not connect to a database by themselves. Keep `.env` private;
`.gitignore` already excludes it. Never send database URLs, passwords, payment
keys, OAuth secrets, or AI keys in a public issue or chat.

## Run checks

```bash
go version
go test ./...
go run .
```

This project declares Go 1.26.0 in `go.mod`; install/use that toolchain through
your managed development environment if your system's Go version is older.

## AI vendor writing assistant

Set `OPENAI_API_KEY` in `.env` or the deployment environment to enable the
optional vendor writing assistant at `/vendor/ai`. `OPENAI_MODEL` is optional
and defaults to `gpt-4o-mini`. The tool drafts menu descriptions and social
captions only; vendors must review the result and publish it themselves. Never
expose the key in frontend JavaScript or commit it to source control.
