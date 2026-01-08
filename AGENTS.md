# AGENTS.md

This file provides guidance for agentic coding assistants working on the "wo" HTTP web framework.

## Development Commands

### Testing
```bash
# Clear test cache
go clean -testcache

# Run all tests with race detector
go test -race ./...

# Run tests with verbose output
go test -race -v ./...

# Run specific package tests
go test -race -v ./middleware

# Run specific test function
go test -race -v ./middleware -run TestCORS_AllowOrigins_ExactMatch

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

# Clean dependencies
go mod tidy

# Run static analysis (if available)
golangci-lint run -v --timeout=5m --build-tags=race --output.code-climate.path gl-code-quality-report.json
```

## Code Style Guidelines

### Imports
- Group imports: standard library first, then third-party packages
- Use blank identifier `_` for compile-time interface checks
- Keep imports alphabetically sorted within groups

Example:
```go
import (
    "context"
    "net/http"

    "github.com/gowool/hook"
)
```

### Types and Generics
- Use generics with `[T Resolver]` constraint for type-safe routing
- Define type aliases for context keys:
```go
type (
    ctxEventKey struct{}
    ctxErrorKey struct{}
)
```
- Exported types use PascalCase
- Struct fields are PascalCase
- Use composition over inheritance where possible

### Naming Conventions
- Exported functions and methods: PascalCase (`NewRouter`, `GET`, `POST`)
- Private functions and methods: camelCase (`matchSubdomain`, `timestampFunc`)
- Constants: PascalCase (`ErrNotFound`, `HeaderOrigin`)
- Interfaces: PascalCase ending with type purpose (`Resolver`, `RateLimiterStorage`)
- Test functions: `Test<FunctionName>_<Scenario>`

### Error Handling
- Use `HTTPError` type for HTTP-specific errors with status codes
- Predefined errors exist in `error.go` (`ErrNotFound`, `ErrBadRequest`, etc.)
- Wrap errors with context: `fmt.Errorf("description: %w", err)`
- Use `errors.Is()` and `errors.As()` for error checking
- Chain errors with `WithInternal()` for internal error details
- Return errors from handlers; framework will handle them

Example:
```go
if err != nil {
    return ErrExtractorError.WithInternal(fmt.Errorf("rate_limiter: failed: %w", err))
}
```

### Middleware Pattern
- Middleware functions return `func(T Resolver) error`
- Accept config struct with `SetDefaults()` method
- Support optional Skipper functions for conditional execution
- Config structs use struct tags for env/json/yaml support

Example:
```go
func CORS[T wo.Resolver](cfg CORSConfig, skippers ...Skipper[T]) func(T) error {
    cfg.SetDefaults()
    return func(e T) error {
        if skip(e) { return e.Next() }
        // middleware logic
        return e.Next()
    }
}
```

### Configuration
- Define config structs with `SetDefaults()` method
- Use struct tags: `env:"KEY" json:"fieldName,omitempty" yaml:"fieldName,omitempty"`
- Document each field with comments explaining defaults and behavior
- Support functional options pattern for callbacks

### Testing
- Use `github.com/stretchr/testify` for assertions (`assert`, `require`)
- Use `httptest` for HTTP request/response testing
- Create helper functions for test setup (`newCORSTestEvent`)
- Use table-driven tests with struct slices:
```go
tests := []struct {
    name     string
    input    string
    expected string
}{
    {name: "case 1", input: "a", expected: "A"},
    {name: "case 2", input: "b", expected: "B"},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        // test logic
    })
}
```
- Always run tests with `-race` flag

### Router and Routing
- Router uses generics: `Router[T Resolver]`
- Routes are added via method shortcuts: `GET`, `POST`, `PUT`, `DELETE`, `PATCH`, etc.
- Use `Group()` for path prefixing and middleware inheritance
- Route paths follow `net/http.ServeMux` pattern format
- Middleware can be bound via `BindFunc` (anonymous) or `Bind` (named with priority)
- Named middleware can be removed with `Unbind()`

### Documentation
- Keep comments concise and focused
- Document exported functions and types
- Include security warnings where applicable (especially in CORS)
- Use examples in docstrings for complex patterns
- Reference external docs with URLs where helpful

### Performance Considerations
- Use `sync.Pool` for frequently allocated objects (e.g., Response)
- Avoid holding references to pooled objects beyond their lifecycle
- Use buffered I/O for large data
- Consider middleware execution order for performance
- Use content negotiation to minimize unnecessary serialization

### Constants
- Define HTTP-related constants in `consts.go`
- Header names follow `Header<Name>` pattern (e.g., `HeaderContentType`)
- Status codes use standard `net/http` constants

### Response Handling
- Response objects are pooled via `sync.Pool`
- Use `Before()` and `After()` hooks for response lifecycle management
- Supports buffering, status tracking, and size tracking
- Implements standard HTTP interfaces: `Flusher`, `Hijacker`, `Pusher`

### Session Management
- Session uses `context.Context` for storing session data
- Supports custom codec implementations for serialization
- Configurable cookie behavior (HttpOnly, Secure, SameSite, etc.)
- RememberMe functionality for persistent sessions

## Framework Architecture

### Core Components
- **Router**: Main routing engine with middleware support
- **Event**: Request context wrapper implementing `Resolver` interface
- **Response**: HTTP response wrapper with buffering and tracking
- **RouterGroup**: Hierarchical route organization with prefixing

### Key Packages
- `wo/`: Core framework (router, response, event, error handling)
- `wo/middleware/`: Middleware implementations (CORS, rate limiting, recovery, etc.)
- `wo/session/`: Session management with pluggable storage
- `wo/internal/`: Internal utilities (convert, encode, arr, must)
- `wo/adapter/`: Integration adapters for other frameworks

### Dependency Note
- Go version: 1.25 (development version)
- Key external dependencies: `github.com/gowool/hook`, `github.com/quic-go/quic-go`
- Uses `github.com/stretchr/testify` for testing
- Uses `github.com/tinylib/msgp` for MessagePack serialization (code generation)
