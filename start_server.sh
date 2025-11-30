#!/bin/bash

# Load environment variables if .env exists
if [ -f .env ]; then
  export $(cat .env | xargs)
fi

echo "Starting gqlgen REST API server..."
echo "Press Ctrl+C to stop"

export PORT=8088
go run cmd/server/main.go
