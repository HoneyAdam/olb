# OpenLoadBalancer Plugin Development Guide

This guide explains how to build, install, and run plugins for OpenLoadBalancer (OLB). Plugins extend OLB at runtime with custom middleware, load-balancing algorithms, health checks, and service discovery providers.

## Table of Contents

- [Overview](#overview)
- [Plugin Interface](#plugin-interface)
- [Plugin Lifecycle](#plugin-lifecycle)
- [PluginAPI Reference](#pluginapi-reference)
- [Extension Points](#extension-points)
- [Event System](#event-system)
- [Metrics](#metrics)
- [Build and Install](#build-and-install)
- [Configuration](#configuration)
- [Example Plugin](#example-plugin)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

OLB uses Go's `plugin` package to load shared object (`.so`) files at startup. Each plugin is a standalone Go module that compiles to a `.so` file and is placed in OLB's plugin directory. The plugin manager discovers, loads, initializes, and starts plugins automatically.

**Requirements:**

- Go 1.23 or later
- Linux or macOS (Go plugins are not supported on Windows)
- Plugin must be built with the same Go version as the OLB binary
- Zero external dependencies (use only Go stdlib and OLB's exported packages)

## Plugin Interface

Every plugin must implement the `plugin.Plugin` interface defined in `internal/plugin/plugin.go`:

```go
type Plugin interface {
    // Name returns the plugin name (unique identifier).
    Name() string
    // Version returns the plugin version string.
    Version() string
    // Init initializes the plugin with access to the host API.
    Init(api PluginAPI) error
    // Start begins plugin operation after initialization.
    Start() error
    // Stop gracefully shuts down the plugin.
    Stop() error
}
```

Additionally, the plugin's `main` package must export a `NewPlugin` function:

```go
// NewPlugin is the required entry point. The plugin manager looks up this
// symbol after loading the .so file. The signature must be exactly:
//   func() plugin.Plugin
func NewPlugin() plugin.Plugin {
    return &MyPlugin{}
}
```

The plugin manager loads the `.so` file, looks up the `NewPlugin` symbol, calls it to get a `Plugin` instance, and then manages the lifecycle.

## Plugin Lifecycle

Plugins go through a well-defined lifecycle:

```
Load (.so) --> NewPlugin() --> RegisterPlugin --> Init(api) --> Start() --> ... --> Stop()
```

1. **Load**: The plugin manager opens the `.so` file and looks up `NewPlugin`.
2. **NewPlugin()**: Called to create a fresh, uninitialized plugin instance.
3. **RegisterPlugin**: The instance is registered in the plugin manager's registry.
4. **Init(api)**: Called with the `PluginAPI`. Register middleware, subscribe to events, create metrics.
5. **Start()**: Called after Init succeeds. Launch background goroutines, open connections.
6. **Stop()**: Called during shutdown (reverse load order). Clean up resources, stop goroutines.

If `Init` returns an error, `Start` is never called and the error is reported to the operator.

If `Stop` returns an error, it is logged but shutdown continues for other plugins.

## PluginAPI Reference

The `PluginAPI` interface is provided to plugins during `Init`. It is the plugin's gateway to the OLB host system.

```go
type PluginAPI interface {
    // RegisterMiddleware registers a middleware factory under the given name.
    RegisterMiddleware(name string, factory MiddlewareFactory) error

    // RegisterBalancer registers a balancer factory under the given name.
    RegisterBalancer(name string, factory BalancerFactory) error

    // RegisterHealthCheck registers a health check factory under the given name.
    RegisterHealthCheck(name string, factory HealthCheckFactory) error

    // RegisterDiscovery registers a discovery factory under the given name.
    RegisterDiscovery(name string, factory DiscoveryFactory) error

    // Logger returns a logger instance scoped to the plugin.
    Logger() *logging.Logger

    // Metrics returns the shared metrics registry.
    Metrics() *metrics.Registry

    // Config returns the current configuration snapshot.
    Config() *config.Config

    // Subscribe subscribes to an event topic. Returns a subscription ID.
    Subscribe(event string, handler EventHandler) string

    // Publish publishes an event to a topic.
    Publish(event string, data interface{})
}
```

### Logger

The logger returned by `api.Logger()` is pre-configured with the plugin's name. Use structured fields for machine-parseable output:

```go
logger := api.Logger()

logger.Info("request processed",
    logging.String("path", "/api/users"),
    logging.Int("status", 200),
    logging.Duration("latency", elapsed),
)

logger.Warn("rate limit exceeded",
    logging.String("client_ip", "10.0.0.1"),
)

logger.Error("connection failed",
    logging.Error(err),
)
```

Available field constructors:

| Function                        | Description            |
|---------------------------------|------------------------|
| `logging.String(key, value)`    | String field           |
| `logging.Int(key, value)`       | Integer field          |
| `logging.Bool(key, value)`      | Boolean field          |
| `logging.Error(err)`            | Error field            |
| `logging.Duration(key, value)`  | Duration field         |

### Metrics

Plugins create and register metrics with the shared registry. These appear alongside built-in OLB metrics in Prometheus and JSON export endpoints.

```go
// Counter: monotonically increasing value (e.g., request counts)
counter := metrics.NewCounter("plugin_myplugin_requests_total", "Total requests")
api.Metrics().RegisterCounter(counter)
counter.Inc()
counter.Add(5)

// CounterVec: counter with labels (e.g., per-path counts)
counterVec := metrics.NewCounterVec(
    "plugin_myplugin_errors_total",
    "Errors by type",
    []string{"error_type"},
)
api.Metrics().RegisterCounterVec(counterVec)
counterVec.With("timeout").Inc()

// Gauge: value that can go up and down (e.g., active connections)
gauge := metrics.NewGauge("plugin_myplugin_active_conns", "Active connections")
api.Metrics().RegisterGauge(gauge)
gauge.Inc()
gauge.Dec()
gauge.Set(42.0)
gauge.Add(5.0)

// Histogram: distribution of values (e.g., latencies)
histogram := metrics.NewHistogram("plugin_myplugin_latency_seconds", "Request latency")
api.Metrics().RegisterHistogram(histogram)
histogram.Observe(0.025)
```

**Naming convention:** Prefix all plugin metrics with `plugin_<name>_` to avoid collisions with built-in metrics.

### Configuration

Access the current OLB configuration with `api.Config()`. The configuration is a snapshot; subscribe to `config.reload` events to react to changes.

```go
cfg := api.Config()
// Use cfg to read any configuration values your plugin needs.
```

## Extension Points

Plugins can extend OLB through four factory-based registries. Each factory receives a `map[string]interface{}` configuration from the YAML config file.

### Custom Middleware

```go
type MiddlewareFactory func(config map[string]interface{}) (Middleware, error)

type Middleware interface {
    Name() string
    Handle(next http.Handler) http.Handler
}
```

Register in Init:

```go
api.RegisterMiddleware("my-middleware", func(cfg map[string]interface{}) (plugin.Middleware, error) {
    return &myMiddleware{}, nil
})
```

Operators enable the middleware in their config:

```yaml
routes:
  - path: /api
    middleware:
      - name: my-middleware
        config:
          option_a: true
```

### Custom Balancer

```go
type BalancerFactory func(config map[string]interface{}) (Balancer, error)

type Balancer interface {
    Name() string
    Next(backends []string) int  // returns index of selected backend, or -1
}
```

Register in Init:

```go
api.RegisterBalancer("my-algorithm", func(cfg map[string]interface{}) (plugin.Balancer, error) {
    return &myBalancer{}, nil
})
```

### Custom Health Check

```go
type HealthCheckFactory func(config map[string]interface{}) (HealthChecker, error)

type HealthChecker interface {
    Name() string
    Check(address string) error  // nil = healthy, error = unhealthy
}
```

Register in Init:

```go
api.RegisterHealthCheck("my-check", func(cfg map[string]interface{}) (plugin.HealthChecker, error) {
    return &myChecker{}, nil
})
```

### Custom Service Discovery

```go
type DiscoveryFactory func(config map[string]interface{}) (DiscoveryProvider, error)

type DiscoveryProvider interface {
    Name() string
    Discover(service string) ([]string, error)
    Watch(service string) (<-chan []string, error)
    Stop() error
}
```

Register in Init:

```go
api.RegisterDiscovery("my-discovery", func(cfg map[string]interface{}) (plugin.DiscoveryProvider, error) {
    return &myProvider{}, nil
})
```

## Event System

The event bus enables loose coupling between OLB components and plugins. Plugins can subscribe to built-in events and publish custom events.

### Built-in Events

| Constant                        | Topic                   | Description                              |
|---------------------------------|-------------------------|------------------------------------------|
| `EventConfigReload`             | `config.reload`         | Configuration file was reloaded          |
| `EventBackendAdded`             | `backend.added`         | A new backend was added                  |
| `EventBackendRemoved`           | `backend.removed`       | A backend was removed                    |
| `EventBackendStateChange`       | `backend.state_change`  | Backend health state changed             |
| `EventRouteAdded`               | `route.added`           | A new route was added                    |
| `EventRouteRemoved`             | `route.removed`         | A route was removed                      |
| `EventHealthCheckResult`        | `health.check_result`   | A health check completed                 |

### Subscribing to Events

```go
func (p *MyPlugin) Init(api plugin.PluginAPI) error {
    // Subscribe returns a subscription ID for later cleanup.
    subID := api.Subscribe(plugin.EventBackendStateChange, func(event plugin.Event) {
        // event.Topic  - the topic name
        // event.Data   - the payload (type depends on the event)
        // event.Timestamp - when the event was published
        fmt.Printf("Backend changed at %s\n", event.Timestamp)
    })

    // Store the ID so you can unsubscribe during Stop.
    p.subscriptionIDs = append(p.subscriptionIDs, subID)
    return nil
}
```

### Publishing Custom Events

Plugins can publish their own events for other plugins to consume. Use a namespaced topic to avoid collisions:

```go
api.Publish("plugin.myplugin.custom_event", map[string]string{
    "key": "value",
})
```

Other plugins (or OLB core) can subscribe to `plugin.myplugin.custom_event` to react to your plugin's events.

### Event Data Types

Event `Data` is `interface{}`, so always use type assertions with a fallback:

```go
func (p *MyPlugin) onEvent(event plugin.Event) {
    switch data := event.Data.(type) {
    case map[string]interface{}:
        // handle map data
    case string:
        // handle string data
    default:
        // unknown type, log and continue
    }
}
```

## Build and Install

### Building a Plugin

```bash
# Navigate to your plugin directory
cd examples/plugins/ratelimit-logger

# Build as a Go plugin (produces a .so file)
go build -buildmode=plugin -o ratelimit-logger.so .
```

**Important:** The plugin must be built with the exact same Go version and module dependencies as the OLB binary. Mismatched versions will cause load failures.

### Installing a Plugin

Copy the `.so` file to OLB's plugin directory:

```bash
# Default plugin directory
cp ratelimit-logger.so /etc/olb/plugins/

# Or wherever your config specifies
cp ratelimit-logger.so /opt/olb/plugins/
```

OLB scans the plugin directory on startup and loads all `.so` files automatically (when `auto_load` is enabled).

### Creating a New Plugin

1. Create a new directory under `examples/plugins/` (or anywhere):

```bash
mkdir my-plugin && cd my-plugin
```

2. Initialize a Go module:

```bash
go mod init github.com/yourorg/olb-plugin-myplugin
```

3. Add the OLB dependency:

```bash
go mod edit -require github.com/openloadbalancer/olb@latest
```

4. Create `main.go` with the required structure:

```go
package main

import "github.com/openloadbalancer/olb/internal/plugin"

func NewPlugin() plugin.Plugin {
    return &MyPlugin{}
}

type MyPlugin struct {
    api plugin.PluginAPI
}

func (p *MyPlugin) Name() string    { return "my-plugin" }
func (p *MyPlugin) Version() string { return "0.1.0" }

func (p *MyPlugin) Init(api plugin.PluginAPI) error {
    p.api = api
    // Register extensions, subscribe to events, create metrics
    return nil
}

func (p *MyPlugin) Start() error {
    // Launch background work
    return nil
}

func (p *MyPlugin) Stop() error {
    // Clean up
    return nil
}
```

5. Build:

```bash
go build -buildmode=plugin -o my-plugin.so .
```

## Configuration

### Plugin Manager Settings

Configure the plugin manager in your OLB YAML config:

```yaml
plugins:
  # Directory to scan for .so plugin files
  directory: /etc/olb/plugins

  # Automatically load all .so files from the directory
  auto_load: true

  # Whitelist of allowed plugin names (empty = allow all)
  allowed:
    - ratelimit-logger
    - my-custom-plugin
```

### Per-Plugin Configuration

Plugin-specific configuration is passed through the factory functions. For middleware, this comes from the route's middleware config:

```yaml
routes:
  - path: /api
    middleware:
      - name: ratelimit-logger
        config:
          log_headers: true
          log_body: false
```

The `config` map is passed as `map[string]interface{}` to the `MiddlewareFactory` (and similarly for other factory types).

## Example Plugin

The `ratelimit-logger` plugin in `examples/plugins/ratelimit-logger/` is a complete, well-commented example that demonstrates:

- Implementing the `Plugin` interface with all lifecycle methods
- Registering a custom middleware that intercepts HTTP 429 responses
- Subscribing to `backend.state_change`, `backend.added`, `backend.removed`, and `config.reload` events
- Creating and updating Counter, CounterVec, and Gauge metrics
- Publishing custom events for plugin-to-plugin communication
- Running a background goroutine with clean shutdown
- Parsing middleware configuration from the factory config map
- Wrapping `http.ResponseWriter` to capture response status codes

To build and install it:

```bash
cd examples/plugins/ratelimit-logger
go build -buildmode=plugin -o ratelimit-logger.so .
cp ratelimit-logger.so /etc/olb/plugins/
```

## Best Practices

### Naming

- **Plugin names**: Use lowercase with hyphens (`my-plugin`).
- **Metrics**: Prefix with `plugin_<name>_` (`plugin_myplugin_requests_total`).
- **Events**: Prefix custom events with `plugin.<name>.` (`plugin.myplugin.alert`).

### Resource Management

- Always clean up in `Stop()`: close files, stop goroutines, release connections.
- Use `sync.WaitGroup` to track background goroutines.
- Use `atomic.Bool` or channels to signal goroutines to exit.

### Error Handling

- Return descriptive errors from `Init` so operators can diagnose issues.
- Log errors in event handlers instead of panicking.
- Use `fmt.Errorf` with `%w` for error wrapping.

### Thread Safety

- Event handlers may be called from any goroutine.
- Use `sync.Mutex` or `sync.RWMutex` to protect shared state.
- Metrics types (Counter, Gauge, Histogram) are safe for concurrent use.

### Configuration

- Provide sensible defaults for all configuration options.
- Use type assertions with fallbacks when reading config maps.
- Subscribe to `config.reload` to pick up configuration changes at runtime.

### Dependencies

- OLB follows a zero-external-dependency philosophy. Plugins should too.
- Only import `github.com/openloadbalancer/olb/internal/...` packages and Go stdlib.
- Do not import third-party modules; they may conflict with OLB's dependency-free design.

## Troubleshooting

### Plugin fails to load

```
failed to open plugin: plugin.Open("my-plugin.so"): ...
```

- Ensure the plugin was built with the same Go version as OLB.
- Ensure the plugin imports the same version of OLB's internal packages.
- Rebuild both OLB and the plugin from the same source tree.

### NewPlugin symbol not found

```
plugin missing NewPlugin symbol
```

- The plugin's `main` package must export `NewPlugin` at the package level.
- The function signature must be `func() plugin.Plugin` (no arguments, returns `plugin.Plugin`).

### NewPlugin has wrong signature

```
NewPlugin has wrong signature, expected func() Plugin
```

- Double-check the return type. It must return `plugin.Plugin`, not a concrete type.
- Ensure you are importing `github.com/openloadbalancer/olb/internal/plugin`.

### Plugin not in allowed list

```
plugin "my-plugin" is not in the allowed list
```

- Add the plugin name to the `allowed` list in the plugins config, or remove the `allowed` key to allow all plugins.

### Plugin already registered

```
plugin "my-plugin" is already registered
```

- Each plugin name must be unique. Check for duplicate `.so` files in the plugin directory.
