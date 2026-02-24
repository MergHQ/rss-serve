#!/bin/bash

# Run database migrations using golang-migrate/migrate

# Check if DATABASE_URL is set, if not use default
export DATABASE_URL="${DATABASE_URL:-postgres://postgres:postgres@localhost:5432/mydatabase?sslmode=disable}"

# Check if migrate tool exists
MIGRATE_TOOL="/home/hugis/go/bin/migrate"
if [ ! -f "$MIGRATE_TOOL" ]; then
    echo "Error: migrate tool not found at $MIGRATE_TOOL"
    echo "Please install it with: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest"
    exit 1
fi

# Run migrations
$MIGRATE_TOOL -path migrations -database "$DATABASE_URL" up

echo "Migrations completed successfully!"
