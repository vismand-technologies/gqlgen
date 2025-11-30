# gqlgen REST API Service

A REST API service for GraphQL code generation, built on top of [gqlgen](https://github.com/99designs/gqlgen).

## About

This project extends the excellent [gqlgen](https://github.com/99designs/gqlgen) library by [99designs](https://99designs.com) to provide a REST API interface for GraphQL code generation. It enables generating Go GraphQL server code without requiring a local Go development environment.

### Credits

This project is built on top of **gqlgen** - a Go library for building GraphQL servers. All core code generation logic comes from the original gqlgen project:

- **Original Project**: [github.com/99designs/gqlgen](https://github.com/99designs/gqlgen)
- **Documentation**: [gqlgen.com](https://gqlgen.com)
- **License**: MIT

## Features

- **REST API for Code Generation** - Generate GraphQL server code via HTTP endpoints
- **In-Memory Generation** - No filesystem dependencies for code generation
- **GitHub Integration** - Fetch user types from GitHub for AutoBind and custom models
- **Zip Download** - Download generated code as a zip archive
- **GitHub Sync** - Push generated code directly to GitHub repositories
- **Custom Module Names** - Specify your own Go module path

## Project Structure

```
├── cmd/
│   └── server/          # REST API server entry point
├── handlers/            # HTTP request handlers
├── service/
│   ├── memory/          # In-memory code generation
│   └── github/          # GitHub package fetching
├── codegen/             # Core gqlgen code generation (from upstream)
├── plugin/              # gqlgen plugins (modelgen, resolvergen, etc.)
├── graphql/             # GraphQL runtime and introspection
└── docs/                # API documentation
```

## Quick Start

### 1. Start the REST Server

```shell
go run cmd/server/main.go
```

The server starts on `http://localhost:8080` by default.

### 2. Generate Code (Basic)

```shell
curl -X POST http://localhost:8080/api/generate/zip \
  -F "schema=@schema.graphqls" \
  -F 'config={"module_name": "github.com/myorg/myproject"}' \
  -o generated.zip
```

### 3. Generate with GitHub AutoBind

Fetch types from your existing GitHub repository:

```shell
curl -X POST http://localhost:8080/api/generate/zip \
  -F "schema=@schema.graphqls" \
  -F 'config={
    "module_name": "github.com/myorg/myproject",
    "autobind": ["github.com/myorg/myproject/models"],
    "github_token": "ghp_your_token"
  }' \
  -o generated.zip
```

### 4. Generate with Custom Model Mappings

Map GraphQL types to existing Go types:

```shell
curl -X POST http://localhost:8080/api/generate/zip \
  -F "schema=@schema.graphqls" \
  -F 'config={
    "module_name": "github.com/myorg/myproject",
    "models": {
      "User": "github.com/myorg/myproject/models.User",
      "Post": "github.com/myorg/myproject/models.Post"
    },
    "github_token": "ghp_your_token"
  }' \
  -o generated.zip
```

### 5. Sync to GitHub Repository

```shell
curl -X POST http://localhost:8080/api/generate/github \
  -H "Content-Type: application/json" \
  -d '{
    "schema": "type Query { hello: String! }",
    "github": {
      "owner": "your-username",
      "repo": "your-repo",
      "branch": "main",
      "path": "graph"
    },
    "token": "ghp_your_github_token"
  }'
```

## API Configuration Options

| Option | Type | Description |
|--------|------|-------------|
| `module_name` | string | Go module name for generated code |
| `package_name` | string | Package name for exec code |
| `model_package` | string | Package name for models |
| `resolver_package` | string | Package name for resolvers |
| `autobind` | []string | GitHub package paths to scan for types |
| `models` | map | GraphQL type to Go type mappings |
| `github_token` | string | GitHub token for private repos |
| `github_ref` | string | Git ref (branch/tag/commit) |
| `omit_slice_element_pointers` | bool | Omit pointers in slice elements |
| `omit_getters` | bool | Omit interface getters |

See [REST API Documentation](docs/rest-api.md) for complete endpoint details.

## Using gqlgen CLI (Traditional)

This project also supports the traditional gqlgen CLI workflow:

```shell
# Initialize a new project
go tool gqlgen init

# Generate code from existing config
go tool gqlgen generate
```

## Documentation

- [REST API Reference](docs/rest-api.md)
- [gqlgen Documentation](https://gqlgen.com)
- [Getting Started Tutorial](https://gqlgen.com/getting-started/)
- [Examples](https://github.com/99designs/gqlgen/tree/master/_examples)

## Contributing

Contributions are welcome. For major changes, please open an issue first to discuss what you would like to change.

## License

This project is licensed under the MIT License - see the original [gqlgen license](https://github.com/99designs/gqlgen/blob/master/LICENSE).

## Acknowledgments

- [99designs](https://99designs.com) for creating and maintaining gqlgen
- The gqlgen community for their contributions and feedback

## Resources

- [gqlgen Documentation](https://gqlgen.com)
- [GraphQL Specification](https://spec.graphql.org/)
- [Go GraphQL Tutorial](https://gqlgen.com/getting-started/)
