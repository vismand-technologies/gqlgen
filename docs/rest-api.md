# gqlgen REST API Documentation

This document describes the REST API endpoints for the gqlgen code generation service.

## Base URL

```
http://localhost:8080
```

## Authentication

GitHub sync endpoints require a GitHub Personal Access Token with `repo` scope. You can provide it either:
- As an environment variable: `GITHUB_TOKEN`
- In the request body for the `/api/generate/github` endpoint

## Endpoints

### GET /

Returns API information and available endpoints.

**Response:**
```json
{
  "message": "gqlgen REST API",
  "version": "v1.0.0",
  "endpoints": [
    "/api/health",
    "/api/version",
    "/api/generate",
    "/api/generate/zip",
    "/api/generate/github"
  ]
}
```

---

### GET /api/health

Health check endpoint.

**Response:**
```json
{
  "status": "healthy",
  "service": "gqlgen-rest-api"
}
```

---

### GET /api/version

Returns the gqlgen version.

**Response:**
```json
{
  "version": "v0.17.x",
  "service": "gqlgen-rest-api"
}
```

---

### POST /api/generate

Generate GraphQL code from a schema.

**Content-Type:** `application/json` or `multipart/form-data`

**JSON Request Body:**
```json
{
  "schema": "type Query { hello: String! }",
  "config": {
    "package_name": "generated",
    "skip_validation": false
  }
}
```

**Multipart Form Request:**
```bash
curl -X POST http://localhost:8080/api/generate \
  -F "schema=@schema.graphqls" \
  -F 'config={"package_name":"generated"}'
```

**Response:**
```json
{
  "message": "Code generated successfully",
  "data": {
    "files": [
      "generated/generated.go",
      "generated/models_gen.go",
      "resolver.go"
    ],
    "count": 3
  }
}
```

---

### POST /api/generate/zip

Generate GraphQL code and return as a zip file.

**Content-Type:** `application/json` or `multipart/form-data`

**JSON Request Body:**
```json
{
  "schema": "type Query { hello: String! }",
  "config": {
    "package_name": "generated"
  }
}
```

**Multipart Form Request:**
```bash
curl -X POST http://localhost:8080/api/generate/zip \
  -F "schema=@schema.graphqls" \
  -o generated.zip
```

**Response:**
- Content-Type: `application/zip`
- Content-Disposition: `attachment; filename=generated-code.zip`
- Binary zip file containing all generated files

---

### POST /api/generate/github

Generate GraphQL code and sync to a GitHub repository.

**Content-Type:** `application/json`

**Request Body:**
```json
{
  "schema": "type Query { hello: String! }",
  "config": {
    "package_name": "generated"
  },
  "github": {
    "owner": "your-username",
    "repo": "your-repo",
    "branch": "main",
    "commit_message": "Update generated GraphQL code",
    "create_repo": false,
    "private": false
  },
  "token": "ghp_your_personal_access_token"
}
```

**Request Fields:**

- `schema` (required): GraphQL schema content
- `config` (optional): Generation configuration
- `github` (required): GitHub sync configuration
  - `owner` (required): GitHub username or organization
  - `repo` (required): Repository name
  - `branch` (optional): Target branch (default: "main")
  - `commit_message` (optional): Commit message
  - `create_repo` (optional): Create repository if it doesn't exist
  - `private` (optional): Make repository private if creating
- `token` (optional): GitHub token (overrides GITHUB_TOKEN env var)

**Response:**
```json
{
  "message": "Code generated and synced to GitHub successfully",
  "data": {
    "repository": "https://github.com/your-username/your-repo",
    "branch": "main",
    "files": 3
  }
}
```

---

## Configuration Options

The `config` object in generation requests supports:

- `package_name`: Package name for generated code (default: "generated")
- `model_package`: Package name for models (default: same as package_name)
- `resolver_package`: Package name for resolvers (default: "main")
- `skip_validation`: Skip validation of generated code (default: false)
- `omit_slice_element_pointers`: Don't use pointers for slice elements (default: false)

---

## Error Responses

All endpoints return errors in this format:

```json
{
  "error": "Error message",
  "details": ["Additional detail 1", "Additional detail 2"]
}
```

Common HTTP status codes:
- `400 Bad Request`: Invalid request parameters
- `500 Internal Server Error`: Code generation or GitHub sync failed

---

## Examples

### Generate and Download Zip

```bash
# Using JSON
curl -X POST http://localhost:8080/api/generate/zip \
  -H "Content-Type: application/json" \
  -d '{
    "schema": "type Query { user(id: ID!): User } type User { id: ID! name: String! }"
  }' \
  -o generated.zip

# Using file upload
curl -X POST http://localhost:8080/api/generate/zip \
  -F "schema=@schema.graphqls" \
  -o generated.zip
```

### Sync to GitHub

```bash
curl -X POST http://localhost:8080/api/generate/github \
  -H "Content-Type: application/json" \
  -d '{
    "schema": "type Query { hello: String! }",
    "github": {
      "owner": "myusername",
      "repo": "my-graphql-api",
      "branch": "main",
      "create_repo": true,
      "private": false
    },
    "token": "ghp_your_token_here"
  }'
```

---

## GitHub Actions Integration

After syncing code to GitHub, you can set up automated Docker builds:

1. Copy `.github/workflows/docker-build.yml` to your repository
2. The workflow will automatically build and push Docker images on commits
3. Images are pushed to GitHub Container Registry (ghcr.io)

---

## Running the Server

```bash
# Using environment variables
export PORT=8080
export GITHUB_TOKEN=ghp_your_token
go run cmd/server/main.go

# Using .env file
cp .env.example .env
# Edit .env with your settings
go run cmd/server/main.go
```
