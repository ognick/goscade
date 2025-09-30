# GOscade

[![Tests](https://github.com/ognick/goscade/actions/workflows/go.yml/badge.svg?style=flat-square&branch=main)](https://github.com/ognick/goscade/actions/workflows/go.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](https://opensource.org/licenses/MIT)

**GOscade** is a lightweight Go library for managing the lifecycle and dependencies of concurrent components. Unlike other solutions, goscade focuses on simplicity, automatic dependency detection, and proper lifecycle management in a concurrent environment.

goscade is a thin wrapper at the application's top level that doesn't penetrate into domain core logic. While the library ensures uniform component startup, it doesn't enforce architecture or affect business logic.

## Examples

<table>
<tr>
<td><img src="docs/basic_workflow.gif" width="400" alt="Basic Workflow"><br>Basic Workflow</td>
<td><img src="docs/startup_error.gif" width="400" alt="Startup Error"><br>Startup Error</td>
<td><img src="docs/unexpected_shutdown.gif" width="400" alt="Unexpected Shutdown"><br>Unexpected Shutdown</td>
</tr>
</table>

## Features

- **Automatic dependency detection** - No manual dependency declaration needed
- **Concurrent execution** - Components run in parallel when possible
- **Graceful shutdown** - Proper cleanup with dependency awareness
- **Health checks** - Built-in readiness probe system
- **Visual graph representation** - See your component dependencies
- **Adapter pattern** - Wrap existing components with custom logic
- **Queue utilities** - FIFO and LIFO queue implementations
- **Circular dependency handling** - Optional support for circular dependencies

## Installation

```bash
go get github.com/ognick/goscade
```

## Usage

```go
package main

import (
    "context"
    "errors"

    "github.com/ognick/goscade"
)

func main() {
    // Create lifecycle manager
    log := logger.NewLogger()
    lc := goscade.NewLifecycle(log)

    // Register components
    lc.Register(&Database{})
    lc.Register(&Cache{})
    lc.Register(&Service{})
    
    // Start all components with readiness probe
    waitGracefulShutdown := lc.Run(context.Background(), func(err error) {
        if err != nil {
            log.Errorf("readiness probe failed: %v", err)
        } else {
            log.Info("All components are ready")
        }
    })
    
    // Awaiting graceful shutdown
    if err := waitGracefulShutdown(); err != nil && !errors.Is(err, context.Canceled) {
        log.Errorf("%v", err)
    }
}
```

## New in v3.0.0

### Adapter Pattern
Wrap existing components with custom logic:

```go
// Wrap an existing HTTP server with custom startup logic
server := &http.Server{Addr: ":8080"}
adapter := goscade.NewAdapter(server, func(ctx context.Context, srv *http.Server, probe func(error)) error {
    // Custom startup logic
    probe(nil) // Signal readiness
    return srv.ListenAndServe()
})

lc.Register(adapter)
```

### Queue Utilities
Built-in FIFO and LIFO queue implementations:

```go
// FIFO Queue (First In, First Out)
fifoQueue := &goscade.FIFOQueue[string]{}
fifoQueue.Enqueue("first")
fifoQueue.Enqueue("second")
item, ok := fifoQueue.Dequeue() // Returns "first"

// LIFO Queue (Last In, First Out - Stack)
lifoQueue := &goscade.LIFOQueue[string]{}
lifoQueue.Enqueue("first")
lifoQueue.Enqueue("second")
item, ok := lifoQueue.Dequeue() // Returns "second"
```

### Fluent Registration API
Chain component creation and registration:

```go
// Using the fluent Register function
server := goscade.Register(lc, NewServer(handler))
db := goscade.Register(lc, NewDatabase(config))
cache := goscade.Register(lc, NewCache(db))
```

### Circular Dependency Support
Optional support for circular dependencies:

```go
lc := goscade.NewLifecycle(log, goscade.WithCircularDependency())
// Now components with circular dependencies won't cause panics
```

## Testing

```bash
make test
```

### Test Coverage
- **98.7% code coverage** - Comprehensive test suite
- **Race condition free** - All tests pass with `-race` flag
- **Concurrent safety** - Verified with multiple test runs
- **37 test cases** covering all major functionality

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Unique Features

- **Automatic dependency detection** through reflection without explicit declaration
- **Built-in concurrent execution** support
- **Graceful shutdown** with dependency graph awareness
- **Minimal API** - just one interface for components
- **No external dependencies** - pure library
- **Interface support** in dependencies
- **Idiomatic Go code** without architecture enforcement

## Core Features

- Component lifecycle management (idle → running → ready → stopping → stopped)
- Automatic dependency graph building
- Topological sorting and component startup
- Readiness signaling via `readinessProbe`
- Graceful shutdown on context cancellation
- Error propagation through dependency graph
- Optional observability: dependency and state inspection

## Technical Limitations

- Components must be pointers or interfaces for proper dependency detection
- Circular dependencies are disabled by default (can be enabled with `WithCircularDependency()`)
- No built-in support for optional dependencies
- Graph is built using reflection on startup, which introduces overhead proportional to the number and complexity of components
- Queue implementations are not thread-safe (use with proper synchronization if needed in concurrent scenarios)

## Requirements

- Go 1.22 or higher

## Usage Examples

### Basic Example with Adapter Pattern

```go
package main

import (
    "context"
    "errors"
    "net/http"
    "time"

    "github.com/ognick/goscade"
)

// Wrap an existing HTTP server
func main() {
    log := NewLogger()
    lc := goscade.NewLifecycle(log)
    
    // Create HTTP server
    server := &http.Server{
        Addr:    ":8080",
        Handler: http.DefaultServeMux,
    }
    
    // Wrap with adapter for custom startup logic
    serverAdapter := goscade.NewAdapter(server, func(ctx context.Context, srv *http.Server, probe func(error)) error {
        // Custom startup logic
        go func() {
            time.Sleep(100 * time.Millisecond) // Simulate startup time
            probe(nil) // Signal readiness
        }()
        
        // Start server
        if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
            return err
        }
        return nil
    })
    
    lc.Register(serverAdapter)
    
    // Start with readiness probe
    waitGracefulShutdown := lc.Run(context.Background(), func(err error) {
        if err != nil {
            log.Errorf("Server failed to start: %v", err)
        } else {
            log.Info("Server is ready on :8080")
        }
    })
    
    // Wait for shutdown
    if err := waitGracefulShutdown(); err != nil && !errors.Is(err, context.Canceled) {
        log.Fatalf("Server error: %v", err)
    }
}
```

### HTTP Server Example

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "net"
    "net/http"

    "github.com/ognick/goscade"
)

// Server component that implements the Component interface
type Server struct {
    handler http.Handler
    server  *http.Server
    health  func() bool
}

func NewServer(handler http.Handler, health func() bool) *Server {
    s := &Server{
        handler: handler,
        server: &http.Server{
            Addr:    ":8080",
            Handler: handler,
        },
        health: health,
    }

    // Register health check endpoint
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        if s.health() {
            w.WriteHeader(http.StatusOK)
            return
        }
        w.WriteHeader(http.StatusServiceUnavailable)
    })

    return s
}

func (s *Server) waitForReady(ctx context.Context) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-time.After(1 * time.Millisecond):
        conn, err := net.Dial("tcp", s.server.Addr)
        if err != nil {
            return err
        }
        return conn.Close()
    }
}

func (s *Server) Run(ctx context.Context, readinessProbe func(error)) error {
    done := make(chan error)
    go func() {
        if err := s.server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
            done <- fmt.Errorf("error occurred while running http server: %w", err)
        }
        close(done)
    }()

    go func() {
        err := s.waitForReady(ctx)
        readinessProbe(err)
    }()

    select {
    case err := <-done:
        if err != nil {
            return fmt.Errorf("failed to listen: %v", err)
        }
    case <-ctx.Done():
        if err := s.server.Shutdown(ctx); err != nil {
            return fmt.Errorf("failed to shutdown http server: %w", err)
        }
    }

    return nil
}

// Example components with dependencies

type Database interface {
    // Database methods
}

type PostgreSQL struct {
    // ...
}

func NewPostgreSQL() *PostgreSQL {
    return &PostgreSQL{}
}

func (d *PostgreSQL) Run(ctx context.Context, readinessProbe func(error)) error {
    readinessProbe(nil)
    <-ctx.Done()
    return ctx.Err()
}

type Cache interface {
    // Cache methods
}

type Redis struct {
    DB Database
}

func NewRedis(db Database) *Redis {
    return &Redis{DB: db}
}

func (c *Redis) Run(ctx context.Context, readinessProbe func(error)) error {
    readinessProbe(nil)
    <-ctx.Done()
    return ctx.Err()
}

type Service struct {
    Cache Cache
}

func NewService(cache Cache) *Service {
    return &Service{Cache: cache}
}

func (s *Service) Run(ctx context.Context, readinessProbe func(error)) error {
    readinessProbe(nil)
    <-ctx.Done()
    return ctx.Err()
}

func main() {
    // Create logger
    log := NewLogger()
    
    // Create lifecycle manager
    lc := goscade.NewLifecycle(log)
    
    // Create and register components using fluent API
    server := goscade.Register(lc, NewServer(http.DefaultServeMux, func() bool {
        return lc.Status() == goscade.LifecycleStatusReady
    }))

    db := goscade.Register(lc, NewPostgreSQL())
    cache := goscade.Register(lc, NewRedis(db))
    service := goscade.Register(lc, NewService(cache))
    
    // Start all components with readiness probe
    waitGracefulShutdown := lc.Run(context.Background(), func(err error) {
        if err != nil {
            log.Errorf("readiness probe failed: %v", err)
        } else {
            log.Info("Server started on http://localhost:8080")
        }
    })
    
    // Wait for graceful shutdown
    if err := waitGracefulShutdown(); err != nil && !errors.Is(err, context.Canceled) {
        log.Fatalf("%v", err)
    }
    
    log.Info("Application gracefully finished")
}