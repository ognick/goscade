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

- Automatic dependency detection
- Concurrent execution
- Graceful shutdown
- Health checks
- Visual graph representation

## Installation

```bash
go get github.com/ognick/goscade
```

## Usage

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/ognick/goscade"
)

func main() {
    ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer stop()

    runner := goscade.NewRunner(ctx)

    // Register components
    goscade.RegisterComponent(runner, &Database{})
    goscade.RegisterComponent(runner, &Cache{})
    goscade.RegisterComponent(runner, &Service{})

    // Start the system
    if err := runner.Run(); err != nil {
        log.Fatal(err)
    }
}
```

## Testing

```bash
make test
```

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

- Components must be pointers or interfaces.
- No built-in support for circular dependencies.
- No built-in support for optional dependencies.
- Graph is built using reflection on startup, which introduces overhead proportional to the number and complexity of components.

## Requirements

- Go 1.22 or higher

## Usage Examples

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
    // Create runner and context for graceful shutdown
    runner, ctx := goscade.CreateRunnerWithGracefulContext()
    log := NewLogger()
    
    // Create lifecycle manager
    lc := goscade.NewLifecycle(log)
    
    // Create and register components
    server := goscade.RegisterComponent(lc, NewServer(http.DefaultServeMux, func() bool {
        return lc.Status() == goscade.LifecycleStatusReady
    }))

    db := goscade.RegisterComponent(lc, NewPostgreSQL())
    cache := goscade.RegisterComponent(lc, NewRedis(db))
    service := goscade.RegisterComponent(lc, NewService(cache))
    
    // Start all components
    lc.RunAllComponents(runner, ctx)
    log.Info("Server started on http://localhost:8080")
    
    // Wait for graceful shutdown
    if err := runner.Wait(); err != nil && !errors.Is(err, context.Canceled) {
        log.Fatalf("%v", err)
    }
    
    log.Info("Application gracefully finished")
}