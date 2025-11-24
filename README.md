# GOscade

[![Tests](https://github.com/ognick/goscade/actions/workflows/go.yml/badge.svg?style=flat-square&branch=main)](https://github.com/ognick/goscade/actions/workflows/go.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/ognick/goscade/v2.svg)](https://pkg.go.dev/github.com/ognick/goscade/v2)
[![Go Report Card](https://goreportcard.com/badge/github.com/ognick/goscade)](https://goreportcard.com/report/github.com/ognick/goscade)
[![codecov](https://codecov.io/gh/ognick/goscade/branch/main/graph/badge.svg)](https://codecov.io/gh/ognick/goscade)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](https://opensource.org/licenses/MIT)

**GOscade** is a lightweight Go library for managing the lifecycle and dependencies of concurrent components. Unlike other solutions, goscade focuses on simplicity, automatic dependency detection, and proper lifecycle management in a concurrent environment.

goscade is a thin wrapper at the application's top level that doesn't penetrate into domain core logic. While the library ensures uniform component startup, it doesn't enforce architecture or affect business logic.

## Features

- **Automatic dependency detection** - No manual dependency declaration needed
- **Explicit dependency declaration** - Support for implicit dependencies when automatic detection is not sufficient
- **Fluent API** - Chain component creation and registration with `Register()` helper
- **Concurrent execution** - Components run in parallel when possible
- **Graceful shutdown** - Proper cleanup with dependency awareness
- **Health checks** - Built-in readiness probe system
- **Visual graph representation** - See your component dependencies
- **Adapter pattern** - Wrap existing components with custom logic
- **Circular dependency handling** - Optional support for circular dependencies
- **Configurable timeouts** - Set custom timeouts for component startup
- **Nested lifecycle** - Lifecycle implements Component interface, can be used as a component in another lifecycle

## Installation

```bash
go get github.com/ognick/goscade/v2
```

## Quick Start

```go
package main

import (
    "context"
    "errors"

    "github.com/ognick/goscade/v2"
)

// Logger interface that your logger must implement
type Logger interface {
    Infof(format string, args ...interface{})
    Errorf(format string, args ...interface{})
}

// Example component that implements goscade.Component
type Server struct {
    addr string
}

func (s *Server) Run(ctx context.Context, readinessProbe func(error)) error {
    // Start your server here
    readinessProbe(nil) // Signal that server is ready
    <-ctx.Done()        // Wait for shutdown signal
    return nil
}

func main() {
    // Create a logger that implements Infof and Errorf methods
    log := &myLogger{} // Your logger implementation
    
    // Create lifecycle manager with signal handling
    lc := goscade.NewLifecycle(log, goscade.WithShutdownHook())

    // Register components using fluent API
    server := goscade.Register(lc, &Server{addr: ":8080"})
    _ = server // Use the registered component if needed
    
    // Option 1: Use helper function with callback
    err := goscade.Run(context.Background(), lc, func() {
        log.Infof("All components are ready")
    })
    
    // Option 2: Use lifecycle.Run() directly
    // err := lc.Run(context.Background(), func(err error) {
    //     if err == nil {
    //         log.Infof("All components are ready")
    //     } else {
    //         log.Errorf("Startup error: %v", err)
    //     }
    // })
    
    // Handle any errors
    if err != nil && !errors.Is(err, context.Canceled) {
        log.Errorf("%v", err)
    }
}
```

## Examples

<table>
<tr>
<td><img src="docs/basic_workflow.gif" width="400" alt="Basic Workflow"><br>Basic Workflow</td>
<td><img src="docs/startup_error.gif" width="400" alt="Startup Error"><br>Startup Error</td>
<td><img src="docs/unexpected_shutdown.gif" width="400" alt="Unexpected Shutdown"><br>Unexpected Shutdown</td>
</tr>
</table>

## Advanced Usage

### Adapter Pattern
Wrap existing components with custom logic:

```go
// Wrap an existing HTTP server with custom startup logic
server := &http.Server{Addr: ":8080"}
adapter := goscade.NewAdapter(server, func(ctx context.Context, srv *http.Server, probe func(cause error)) error {
    // Custom startup logic
    probe(nil) // Signal readiness
    return srv.ListenAndServe()
})

lc.Register(adapter)
```

### Fluent Registration API
Chain component creation and registration:

```go
// Using the fluent Register function
server := goscade.Register(lc, NewServer(handler))
db := goscade.Register(lc, NewDatabase(config))
cache := goscade.Register(lc, NewCache(db))

// Alternative: traditional registration
lc.Register(NewServer(handler))
lc.Register(NewDatabase(config))
lc.Register(NewCache(db))
```

### Explicit Dependencies
When automatic dependency detection is not sufficient, you can explicitly declare dependencies:

```go
// Register components with explicit dependencies
db := goscade.Register(lc, NewDatabase(config))
cache := goscade.Register(lc, NewCache(), db) // cache depends on db
service := goscade.Register(lc, NewService(), db, cache) // service depends on both db and cache
```

This is useful when:
- Dependencies are injected through interfaces
- Dependencies are passed as function parameters
- You need to ensure specific startup order

### Circular Dependency Support
Optional support for circular dependencies:

```go
lc := goscade.NewLifecycle(log, goscade.WithCircularDependency())
// Now components with circular dependencies won't cause panics
```

### Configurable Timeouts
Set custom timeouts for component startup:

```go
// Set 30 seconds timeout for component startup
lc := goscade.NewLifecycle(log, goscade.WithStartTimeout(30*time.Second))
// Components that do not become ready within 30 seconds will timeout
```

### Nested Lifecycle
Lifecycle implements the Component interface, so you can use it as a component in another lifecycle:

```go
// Create child lifecycle with components
childLC := goscade.NewLifecycle(log)
childLC.Register(&Database{})
childLC.Register(&Cache{})

// Register child lifecycle as component in parent (no casting needed)
parentLC := goscade.NewLifecycle(log)
parentLC.Register(childLC) // Lifecycle is a Component
parentLC.Register(&APIServer{})

// Run parent lifecycle - it will manage child lifecycle as a component
err := parentLC.Run(ctx, func(err error) {
    if err == nil {
        log.Infof("All lifecycles are ready")
    }
})
```

## Requirements

- Go 1.22 or higher

## Technical Details

### Core Features
- Component lifecycle management (idle → running → ready → stopping → stopped)
- Automatic dependency graph building through reflection
- Explicit dependency declaration for complex scenarios
- Fluent API for component registration with `goscade.Register()`
- Topological sorting and component startup
- Readiness signaling via `readinessProbe`
- Graceful shutdown on context cancellation
- Error propagation through dependency graph
- Optional observability: dependency and state inspection
- Blocking lifecycle execution with `goscade.Run()` helper function
- Lifecycle implements Component interface (can be nested)
- Optional signal handling for graceful shutdown with `WithShutdownHook()`

### Technical Limitations
- Components must be pointers or interfaces for proper dependency detection
- Circular dependencies are disabled by default (can be enabled with `WithCircularDependency()`)
- Graph is built using reflection on startup, which introduces overhead proportional to the number and complexity of components
- Explicit dependencies must be registered before the component that depends on them


## License

This project is licensed under the MIT License - see the LICENSE file for details.