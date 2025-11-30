#!/bin/bash

# Start the server in the background
echo "Starting server..."
export PORT=8088
go run cmd/server/main.go &
SERVER_PID=$!

# Wait for server to be ready
echo "Waiting for server to be ready..."
max_retries=30
count=0
while ! curl -s http://localhost:8088/api/health > /dev/null; do
    sleep 1
    count=$((count+1))
    if [ $count -ge $max_retries ]; then
        echo "Server failed to start"
        kill $SERVER_PID
        exit 1
    fi
done
echo "Server is ready!"

# Test /api/version
echo -e "\nTesting /api/version..."
curl -s http://localhost:8088/api/version | grep "gqlgen-rest-api" && echo "Version check passed" || echo "Version check failed"

# Test /api/generate (JSON)
echo -e "\nTesting /api/generate (JSON)..."
SCHEMA='type Todo { id: ID! text: String! done: Boolean! } type Query { todos: [Todo!]! }'
RESPONSE=$(curl -s -X POST http://localhost:8088/api/generate \
  -H "Content-Type: application/json" \
  -d "{\"schema\": \"$SCHEMA\", \"config\": {\"package_name\": \"generated\"}}")

if echo "$RESPONSE" | grep "models_gen.go" > /dev/null; then
    echo "JSON generation passed"
else
    echo "JSON generation failed"
    echo "$RESPONSE"
fi

# Test /api/generate/zip (Multipart)
echo -e "\nTesting /api/generate/zip (Multipart)..."
echo "$SCHEMA" > test_schema.graphqls
curl -s -X POST http://localhost:8088/api/generate/zip \
  -F "schema=@test_schema.graphqls" \
  -F "config={\"package_name\": \"generated\"}" \
  -o generated.zip

if [ -f generated.zip ] && [ -s generated.zip ]; then
    echo "Zip generation passed"
    rm generated.zip
else
    echo "Zip generation failed"
fi
rm test_schema.graphqls

# Clean up
echo -e "\nStopping server..."
kill $SERVER_PID
