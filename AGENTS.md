# AGENTS.md

This file provides guidance for agentic coding assistants working on the "wo" HTTP web framework.

## Development Commands

### Testing
```bash
# Run all tests with race detector
go test -race ./...

# Run specific package tests
go test -race -v ./middleware

# Run specific test function
go test -race -v ./middleware -run TestCORS_AllowOrigins_ExactMatch

# Run tests with coverage
go test -race -cover ./...
```

### Building
```bash
go build ./...
go build -v ./...
```

### Code Quality
```bash
go fmt ./...
go vet ./...
go mod tidy
golangci-lint run -v --timeout=5m --build-tags=race
```

## Code Style Guidelines

### Imports
- Group imports: standard library → third-party → internal (blank lines between groups)
- Keep imports alphabetically sorted within groups
- Use blank identifier `_` for compile-time interface checks

Example:
```go
import (
    "context"
    "net/http"

    "github.com/gowool/hook"

    "github.com/gowool/wo/internal/convert"
)
```

### Types and Generics
- Use generics with `[T Resolver]` constraint for type-safe routing
- Define zero-sized struct types for context keys to prevent collisions
- Exported types: PascalCase, private fields: camelCase
- All config structs have `SetDefaults()` method with env/json/yaml struct tags

Example:
```go
type (
    ctxEventKey struct{}
    ctxErrorKey struct{}
)

type CORSConfig struct {
    AllowOrigins []string `env:"ALLOW_ORIGINS" json:"allowOrigins,omitempty"`
    AllowOriginFunc func(origin string) (bool, error) `json:"-"`
}

func (c *CORSConfig) SetDefaults() {
    if len(c.AllowOrigins) == 0 {
        c.AllowOrigins = []string{"*"}
    }
}
```

### Naming Conventions
- Exported: PascalCase (`NewRouter`, `GET`, `ErrNotFound`)
- Private: camelCase (`matchSubdomain`, `timestampFunc`)
- Interfaces: PascalCase ending with type purpose (`Resolver`, `RateLimiterStorage`)
- Tests: `Test<FunctionName>_<Scenario>`
- Constants: `Header<Name>` for headers, `MIME<Type>` for content types

### Error Handling
- Use `HTTPError` type with status codes and internal error wrapping
- Wrap errors with context: `ErrBadRequest.WithInternal(fmt.Errorf("context: %w", err))`
- Use `errors.Is()` and `errors.As()` for error checking
- Predefined errors exist in `error.go` (`ErrNotFound`, `ErrBadRequest`, etc.)
- Chain errors with `WithInternal()` for internal error details

### Middleware Pattern
- Middleware return `func(T Resolver) error`
- Accept config struct with `SetDefaults()` and optional Skipper functions

Example:
```go
func CORS[T wo.Resolver](cfg CORSConfig, skippers ...Skipper[T]) func(T) error {
    cfg.SetDefaults()
    skip := ChainSkipper[T](skippers...)
    return func(e T) error {
        if skip(e) { return e.Next() }
        // middleware logic
        return e.Next()
    }
}
```

### Testing
- Use `github.com/stretchr/testify` for assertions (`assert`, `require`)
- Use table-driven tests with struct slices
- Always run tests with `-race` flag
- Create helper functions for test setup (`newCORSTestEvent`)

Example:
```go
tests := []struct {
    name     string
    input    string
    expected string
}{
    {name: "case 1", input: "a", expected: "A"},
}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        assert.Equal(t, tt.expected, result)
    })
}
```

### Router and Routing
- Router uses generics: `Router[T Resolver]`
- Routes via method shortcuts: `GET`, `POST`, `PUT`, `DELETE`, `PATCH`, `HEAD`, `OPTIONS`
- Use `Group()` for path prefixing and middleware inheritance
- Middleware bound via `BindFunc` (anonymous) or `Bind` (named with priority)
- Named middleware can be removed with `Unbind()`
- Routes follow `net/http.ServeMux` pattern format

### Performance Considerations
- Use `sync.Pool` for frequently allocated objects (Response)
- Avoid holding references to pooled objects beyond their lifecycle
- Use buffered I/O for large data
- Consider middleware execution order

### Documentation
- Document exported functions and types with godoc comments
- Include security warnings where applicable (especially in CORS)
- Use examples in docstrings for complex patterns
- Reference external docs with URLs

## Framework Architecture

### Core Components
- **Router**: Generic routing engine with middleware support (`Router[T Resolver]`)
- **Event**: Request context implementing `Resolver` interface
- **Response**: HTTP response wrapper with buffering, tracking, and Before/After hooks
- **RouterGroup**: Hierarchical route organization with prefixing and fluent chaining

### Key Packages
- `wo/`: Core framework (router, response, event, error handling)
- `wo/middleware/`: Middleware implementations (CORS, rate limiting, recovery, etc.)
- `wo/session/`: Session management with pluggable storage and codecs
- `wo/internal/`: Internal utilities (convert, encode, arr, must)
- `wo/adapter/`: Integration adapters for other frameworks

### Dependencies
- Go 1.25+ (development version)
- Key: `github.com/gowool/hook`, `github.com/quic-go/quic-go`, `github.com/stretchr/testify`, `github.com/tinylib/msgp`
