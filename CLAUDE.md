# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

### Testing
```bash
# Clear test cache
go clean -testcache

# Run all tests
go test -race ./...

# Run tests with verbose output
go test -race -v ./...

# Run specific test
go test -race -v ./middleware

# Run tests with coverage
go test -race -cover ./...
```

### Building
```bash
# Build the package
go build ./...

# Build with verbose output
go build -v ./...
```

### Code Quality
```bash
# Format code
go fmt ./...

# Run go vet
go vet ./...

# Run go mod tidy to clean dependencies
go mod tidy

# Run static analysis with golangci-lint
golangci-lint run -v --timeout=5m --build-tags=race --output.code-climate.path gl-code-quality-report.json
```

### Dependencies
- Uses Go 1.25 (development version)
- Key dependencies: `github.com/gowool/hook`, `github.com/quic-go/quic-go`, `github.com/invopop/validation`
- Uses `github.com/tinylib/msgp` for MessagePack serialization with code generation tool

## Architecture Overview

"wo" is a modern HTTP web framework built with Go 1.25+ generics, featuring a type-safe routing system with comprehensive middleware support.

### Core Architecture Patterns

#### Generic-Based Type System
The framework is built around `Resolver[T]` interface where `T` is a generic type parameter that extends the base `Resolver`. This provides compile-time type safety while maintaining flexibility:

```go
type Resolver interface {
    hook.Resolver
    SetRequest(r *http.Request)
    Request() *http.Request
    SetResponse(w http.ResponseWriter)
    Response() http.ResponseWriter
}
```

#### Event-Driven Request Processing
- `Event` struct wraps HTTP request/response and provides context throughout request lifecycle
- `EventFactoryFunc[T]` creates route-specific events with optional cleanup functions
- Response pooling via `sync.Pool` for performance optimization

#### Middleware Hook System
Built on `github.com/gowool/hook` with sophisticated middleware management:
- Named middleware with priority ordering
- Middleware exclusion/removal capabilities
- Nested middleware inheritance from router groups
- Pre-hook system for request preprocessing

### Key Components

#### Core Router (`router.go`)
- `Router[T]` extends `RouterGroup[T]` and builds `http.ServeMux` handlers
- Supports hierarchical routing with group-based middleware inheritance
- Pattern-based route registration with conflict detection

#### Response Handling (`response.go`, `event.go`)
- `Response` wrapper around `http.ResponseWriter` with buffering and status tracking
- `Event` implements `Resolver` interface and provides request context
- Content negotiation support for JSON, XML, HTML, and MessagePack

#### Router Groups (`group.go`)
- Hierarchical route organization with path prefixing
- Middleware inheritance from parent groups
- Route method shortcuts (GET, POST, PUT, DELETE, etc.)

#### Error Handling (`error.go`)
- `HTTPError[T]` and `RedirectError[T]` types for typed error handling
- `HTTPErrorHandler[T]` function type for centralized error processing
- Internal error chaining with context preservation

### Middleware Ecosystem (`middleware/`)

#### CORS (`cors.go`)
Full CORS support with configurable origins, methods, headers, and security options including credential support.

#### Rate Limiting (`rate_limiter*.go`)
Token bucket rate limiter with pluggable storage backends:
- Memory-based storage implementation
- MessagePack-based storage integration
- Configurable rate limits and burst sizes

#### Session Management (`session.go`)
Session handling with configurable storage and codec support for distributed applications.

#### Body Handling (`body*.go`)
- `body_limit.go`: Request body size limiting
- `body_rereadable.go`: Body buffering for multiple reads

#### Security (`secure.go`)
Security headers and HTTP-only protections for production deployments.

### Supporting Packages

#### Internal Utilities (`internal/`)
- `convert/`: Type conversion utilities
- `encode/`: Serialization helpers
- `arr/`: Array and slice utilities
- `must/`: Panic-on-error helpers for initialization

#### Integration (`adapter/`)
- `huma/`: Integration adapter for Huma (OpenAPI) framework compatibility

#### Server Utilities (`server/`)
Server configuration and utility functions for production deployments.

## Testing Patterns

- Uses `github.com/stretchr/testify` for assertions and test helpers
- Focuses on critical path testing over exhaustive coverage
- Comprehensive middleware testing with mock implementations
- Integration tests for content negotiation and header handling

## Development Guidelines

### When Adding New Routes
1. Use appropriate HTTP method shortcuts on `RouterGroup`
2. Consider middleware inheritance from parent groups
3. Leverage generics for type-safe handler signatures
4. Use `EventFactoryFunc` for custom event creation when needed

### When Creating Middleware
1. Implement `hook.HookFunc[T]` interface
2. Follow the established naming pattern in `middleware/`
3. Include comprehensive tests with mock scenarios
4. Document configuration options and behavior

### Error Handling Patterns
- Use `HTTPError[T]` for HTTP-specific errors with status codes
- Leverage `RedirectError[T]` for HTTP redirects
- Implement custom error handlers via `HTTPErrorHandler[T]`
- Chain errors with context preservation

### Performance Considerations
- Response objects are pooled via `sync.Pool` - avoid holding references
- Middleware execution order matters for performance
- Use content negotiation to minimize unnecessary serialization