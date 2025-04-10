---
sidebar_position: 3
---

import Tabs from '@theme/Tabs';
import TabItem from '@theme/TabItem';

# OpenTelemetry for Go

Instrumenting a Golang application with OpenTelemetry can provide valuable insights into the application's performance and behavior. 
OpenTelemetry is an open-source observability framework that enables the collection and exporting of telemetry data. 
This document covers the steps required to instrument a Golang application with OpenTelemetry.

## HTTP server
HTTP server instrumentation involves generating detailed spans that describe the handling of inbound HTTP requests. 
These spans provide insight into the entire lifecycle of each request, from the moment it arrives at the server to the moment 
it is sent back to the client.

<Tabs queryString="http-server">
  <TabItem value="without_routers" label="Without Request Routers" default>

**Step 1: add OpenTelemetry dependencies**

```bash
$ go get \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp \
  go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp
```

**Step 2: initialize OpenTelemetry and instrument HTTP handlers**

The following example demonstrates how to instrument an HTTP handler with OpenTelemetry and export traces to an OpenTelemetry Collector through HTTP. The collector's endpoint and coroot API key should be provided.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type TracerShutdown func(ctx context.Context) error

// Initialize OpenTelemetry tracer
func initTracer(ctx context.Context, otlpEndpoint, corootApiKey string) (TracerShutdown, error) {
	// create new exporter
	traceExporter, err := otlptrace.New(
		ctx,
		otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(otlpEndpoint),
			otlptracehttp.WithHeaders(map[string]string{"X-API-Key": corootApiKey}),
		),
	)
	if err != nil {
		return nil, err
	}

	// create new resource
	resources, err := resource.New(
		ctx,
		resource.WithAttributes(
			attribute.String("service.name", "hello_server"),
			attribute.String("library.language", "go"),
		),
	)
	if err != nil {
		return nil, err
	}

	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(resources),
		),
	)

	return traceExporter.Shutdown, nil
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, World!")
}

func main() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

  // ctrl+c handling
	go func() {
		select {
		case <-ctx.Done():
			return
		case s := <-sig:
			log.Printf("signal %s received: \n", s.String())
			cancel()
		}
	}()

	// If your coroot has only one project, leave corootApiKey empty
	tracerShutdownFunc, err := initTracer(ctx, "coroot.coroot:8080", "")
	if err != nil {
		log.Fatalf("failed to initialize tracer: %v", err)
	}

	// Initialize the HTTP server with instrumentation
	router := http.NewServeMux()
	instrumentedHandler := otelhttp.NewHandler(http.HandlerFunc(helloHandler), "GET /hello-world")
	router.Handle("/hello-world", instrumentedHandler)

	server := &http.Server{Addr: ":8082", Handler: router}

	go func() {
		log.Println("Starting server on :8082")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for termination signal
	<-ctx.Done()
	log.Println("Shutting down gracefully...")

	// Shutdown tracing, flush all remaining traces
	shutdownCtx := context.Background()
	if err := tracerShutdownFunc(shutdownCtx); err != nil {
		log.Printf("tracer shutdown error: %v", err)
	}

	// Shutdown HTTP server gracefully
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Bye bye!")
}
```

**Step 3: start your application**

```bash
go run main.go
```

**Step 4: validating**

As a result, our app reports traces containing only a server span:

<img alt="Go HTTP Server Span" src="/img/docs/go_server_span_http.png" class="card w-1200"/>

Span attributes:

<img alt="Go HTTP Server Span Attributes" src="/img/docs/go_server_span_attributes_http.png" class="card w-1200"/>

  </TabItem>
  <TabItem value="gorilla" label="Gorilla Mux">

**Step 1: add OpenTelemetry dependencies**

```bash
$ go get \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp \
  go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux github.com/gorilla/mux 
```

**Step 2: initialize OpenTelemetry and instrument the request router**

The following example demonstrates how to instrument the [Gorilla HTTP router](https://github.com/gorilla/mux) with OpenTelemetry and export traces to an 
OpenTelemetry Collector through HTTP. The collector's endpoint and coroot API key should be provided.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type TracerShutdown func(ctx context.Context) error

// Initialize OpenTelemetry tracer
func initTracer(ctx context.Context, otlpEndpoint, corootApiKey string) (TracerShutdown, error) {
	// Create new exporter
	traceExporter, err := otlptrace.New(
		ctx,
		otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(otlpEndpoint),
			otlptracehttp.WithHeaders(map[string]string{"X-API-Key": corootApiKey}),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create new resource
	resources, err := resource.New(
		ctx,
		resource.WithAttributes(
			attribute.String("service.name", "hello_server"),
			attribute.String("library.language", "go"),
		),
	)
	if err != nil {
		return nil, err
	}

	// Set the global trace provider
	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(resources),
		),
	)

	return traceExporter.Shutdown, nil
}

// HTTP handler for /hello-world route
func helloWorldHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, World!")
}

func main() {
	// Handle OS signals for graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ctrl+C handling
	go func() {
		select {
		case <-ctx.Done():
			return
		case s := <-sig:
			log.Printf("signal %s received: \n", s.String())
			cancel()
		}
	}()

	// If your coroot has only one project, leave corootApiKey empty
	tracerShutdownFunc, err := initTracer(ctx, "coroot.coroot:8080", "")
	if err != nil {
		log.Fatalf("failed to initialize tracer: %v", err)
	}

	// Initialize the HTTP server with instrumentation
	router := mux.NewRouter()
	router.Use(otelhttp.NewMiddleware("hello_http_server"))
	router.Handle("/hello-world", http.HandlerFunc(helloWorldHandler))

	server := &http.Server{Addr: ":8082", Handler: router}

	go func() {
		log.Println("Starting server on :8082")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for termination signal
	<-ctx.Done()
	log.Println("Shutting down gracefully...")

	// Shutdown tracing, flush all remaining traces
	shutdownCtx := context.Background()
	if err := tracerShutdownFunc(shutdownCtx); err != nil {
		log.Printf("tracer shutdown error: %v", err)
	}

	// Shutdown HTTP server gracefully
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Bye bye!")
}
```

**Step 3: start your application**

```bash
go run main.go
```

**Step 4: validating**

As a result, our app reports traces containing only a server span:

<img alt="Go HTTP Server Span" src="/img/docs/go_server_span_gorilla.png" class="card w-1200"/>

Span attributes:

<img alt="Go HTTP Server Span Attributes" src="/img/docs/go_server_span_attributes_gorilla.png" class="card w-1200"/>

  </TabItem>
  <TabItem value="echo" label="Echo Web Framework">

**Step 1: add OpenTelemetry dependencies**

```bash 
$ go get \
  go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp \
  go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho
```

**Step 2: initialize OpenTelemetry and instrument the request router**

The following example demonstrates how to instrument the [Echo web framework](https://echo.labstack.com/) with OpenTelemetry and export traces to an OpenTelemetry Collector through HTTP. The collector's endpoint and coroot API key should be provided.

```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/labstack/echo/v4"

	"go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type TracerShutdown func(ctx context.Context) error

// Initialize OpenTelemetry tracer
func initTracer(ctx context.Context, otlpEndpoint, corootApiKey string) (TracerShutdown, error) {
	// Create new exporter
	traceExporter, err := otlptrace.New(
		ctx,
		otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(otlpEndpoint),
			otlptracehttp.WithHeaders(map[string]string{"X-API-Key": corootApiKey}),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create new resource
	resources, err := resource.New(
		ctx,
		resource.WithAttributes(
			attribute.String("service.name", "hello_server"),
			attribute.String("library.language", "go"),
		),
	)
	if err != nil {
		return nil, err
	}

	// Set the global trace provider
	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(resources),
		),
	)

	return traceExporter.Shutdown, nil
}

// HTTP handler for /hello-world route
func helloWorldHandler(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}

func main() {
	// Handle OS signals for graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ctrl+C handling
	go func() {
		select {
		case <-ctx.Done():
			return
		case s := <-sig:
			log.Printf("signal %s received: \n", s.String())
			cancel()
		}
	}()

	// If your coroot has only one project, leave corootApiKey empty
	tracerShutdownFunc, err := initTracer(ctx, "coroot.coroot:8080", "")
	if err != nil {
		log.Fatalf("failed to initialize tracer: %v", err)
	}

	// Initialize the Echo server with instrumentation
	e := echo.New()
	e.Use(otelecho.Middleware("hello_server"))
	e.GET("/hello-world", helloWorldHandler)

	server := &http.Server{Addr: ":8082", Handler: e}

	go func() {
		log.Println("Starting server on :8082")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for termination signal
	<-ctx.Done()
	log.Println("Shutting down gracefully...")

	// Shutdown tracing, flush all remaining traces
	shutdownCtx := context.Background()
	if err := tracerShutdownFunc(shutdownCtx); err != nil {
		log.Printf("tracer shutdown error: %v", err)
	}

	// Shutdown HTTP server gracefully
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Bye bye!")
}
```

**Step 3: start your application**

```bash
go run main.go
```

**Step 4: validating**

As a result, our app reports traces containing only a server span:

<img alt="Go HTTP Server Span" src="/img/docs/go_server_span_echo.png" class="card w-1200"/>

Span attributes:

<img alt="Go HTTP Server Span Attributes" src="/img/docs/go_server_span_attributes_echo.png" class="card w-1200"/>

</TabItem>
  <TabItem value="gin" label="GIN Web Framework">

**Step 1: add OpenTelemetry dependencies**
```bash 
$ go get \
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp \
go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin
```

**Step 2: initialize OpenTelemetry and instrument the request router**

The following example demonstrates how to instrument the [Gin web framework](https://github.com/gin-gonic/gin) with OpenTelemetry and export traces to an 
OpenTelemetry Collector through HTTP. The collector's endpoint and coroot API key should be provided.

```go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type TracerShutdown func(ctx context.Context) error

// Initialize OpenTelemetry tracer
func initTracer(ctx context.Context, otlpEndpoint, corootApiKey string) (TracerShutdown, error) {
	// Create new exporter
	traceExporter, err := otlptrace.New(
		ctx,
		otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(otlpEndpoint),
			otlptracehttp.WithHeaders(map[string]string{"X-API-Key": corootApiKey}),
		),
	)
	if err != nil {
		return nil, err
	}

	// Create new resource
	resources, err := resource.New(
		ctx,
		resource.WithAttributes(
			attribute.String("service.name", "hello_server"),
			attribute.String("library.language", "go"),
		),
	)
	if err != nil {
		return nil, err
	}

	// Set the global trace provider
	otel.SetTracerProvider(
		sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sdktrace.AlwaysSample()),
			sdktrace.WithBatcher(traceExporter),
			sdktrace.WithResource(resources),
		),
	)

	return traceExporter.Shutdown, nil
}

// HTTP handler for /hello-world endpoint
func helloWorldHandler(c *gin.Context) {
	c.String(http.StatusOK, "Hello, World!")
}

func main() {
	// Handle OS signals for graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Ctrl+C handling
	go func() {
		select {
		case <-ctx.Done():
			return
		case s := <-sig:
			log.Printf("signal %s received: \n", s.String())
			cancel()
		}
	}()

	// If your coroot has only one project, leave corootApiKey empty
	tracerShutdownFunc, err := initTracer(ctx, "coroot.coroot:8080", "")
	if err != nil {
		log.Fatalf("failed to initialize tracer: %v", err)
	}

	// Initialize the Gin server with instrumentation
	r := gin.Default()
	r.Use(otelgin.Middleware("hello_server"))
	r.GET("/hello-world", helloWorldHandler)

	server := &http.Server{Addr: ":8082", Handler: r}

	go func() {
		log.Println("Starting server on :8082")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Wait for termination signal
	<-ctx.Done()
	log.Println("Shutting down gracefully...")

	// Shutdown tracing, flush all remaining traces
	shutdownCtx := context.Background()
	if err := tracerShutdownFunc(shutdownCtx); err != nil {
		log.Printf("tracer shutdown error: %v", err)
	}

	// Shutdown HTTP server gracefully
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Bye bye!")
}
```

**Step 3: start your application**

```bash
go run main.go
```

**Step 4: validating**

As a result, our app reports traces containing only a server span:

<img alt="Go HTTP Server Span" src="/img/docs/go_server_span_echo.png" class="card w-1200"/>

Span attributes:

<img alt="Go HTTP Server Span Attributes" src="/img/docs/go_server_span_attributes_echo.png" class="card w-1200"/>

  </TabItem>
</Tabs>

## Adding custom attributes to spans

To add custom attributes to a span while processing a particular request, you can retrieve the span from the request context.
This allows you to add contextual information that can help with understanding and debugging the request.

<Tabs queryString="http-server">
  <TabItem value="http_router" label="http.Handler" default>

```go
func helloHandler(w http.ResponseWriter, r *http.Request) {
  vars := mux.Vars(r)
  name := vars["name"]
  
  // get the current span by the request context
  currentSpan := trace.SpanFromContext(r.Context())
  currentSpan.SetAttributes(attribute.String("hello.name", name))
  
  fmt.Fprintf(w, "Hello, %s!", name)
}
```

  </TabItem>
  <TabItem value="echo" label="Echo Web Framework">

```go
e.GET("/hello/:name", func(c echo.Context) error {
  name := c.Param("name")
  
  // get the current span by the request context
  currentSpan := trace.SpanFromContext(c.Request().Context())
  currentSpan.SetAttributes(attribute.String("hello.name", name))
  
  return c.String(http.StatusOK, fmt.Sprintf("Hello, %s!", name))
})
```

  </TabItem>
  <TabItem value="gin" label="Gin Web Framework">

```go
r.GET("/hello/:name", func(c *gin.Context) {
  name := c.Param("name")
  
  // get the current span by the request context
  currentSpan := trace.SpanFromContext(c.Request.Context())
  currentSpan.SetAttributes(attribute.String("hello.name", name))
  
  c.String(http.StatusOK, "Hello, %s!", name)
})
```
  </TabItem>
</Tabs>

## Instrumenting http.Client

If your application makes calls to other services or public HTTP APIs, you can instrument those calls using the `otelhttp` HTTP client wrapper.

<Tabs queryString="request">
  <TabItem value="get" label="http.Get" default>

```go
func handler(w http.ResponseWriter, r *http.Request) {
  // using the initial request context to propagate the relevant trace context to nested spans
  resp, err := otelhttp.Get(r.Context(), "https://www.google.com")
  ...
}
```
  </TabItem>
  <TabItem value="manual" label="Manual Request Creation">

```go
func handler(w http.ResponseWriter, r *http.Request) {
  // using the initial request context to propagate the relevant trace context to nested spans
  req, err := http.NewRequestWithContext(r.Context(), "GET", "https://www.google.com", nil)
  if err != nil {
    ...
  }
  req.Header.Set("X-Header", "value")
  resp, err := otelhttp.DefaultClient.Do(req)
  ...
}
```
  </TabItem>
</Tabs>

Traces now include not only a server span but also a client span that provides details of the outbound HTTP call.

<img alt="Go HTTP client Span" src="/img/docs/go_http_client_span.png" class="card w-1200"/>

Client span attributes:

<img alt="Go HTTP client Span Attributes" src="/img/docs/go_http_client_span_attributes.png" class="card w-1200"/>

## Instrumenting SQL queries

To instrument SQL queries with OpenTelemetry in Golang, you can use the `otelsql` package. Here's an example of how to use it:

```go
import (
  "database/sql"
  _ "github.com/lib/pq"
  "github.com/uptrace/opentelemetry-go-extra/otelsql"
  ...
)

var db *sql.DB

func main() {
  ...
  // Instrument `sql.DB` with the `otelsql` wrapper
  db, err = otelsql.Open("postgres", connStr, otelsql.WithAttributes(semconv.DBSystemPostgreSQL))
  ...
}

func handler(w http.ResponseWriter, r *http.Request) {
  ...
  // context propagation using r.Context()
  rows, err := db.QueryContext(r.Context(), "SELECT * FROM products WHERE brand=$1", brand)
  ...
}
```

Client span attributes:

<img alt="Go SQL client Span Attributes" src="/img/docs/go_sql_span_attributes.png" class="card w-1200"/>

## GORM

GORM is a popular open-source Object-Relational Mapping (ORM) library for Go. 
It provides a convenient way to interact with databases by abstracting away the low-level details of SQL queries and 
allowing developers to work with higher-level objects and methods.

To instrument the underlying SQL queries with OpenTelemetry, you can use the `otelgorm` package:

```go
import (
  "github.com/uptrace/opentelemetry-go-extra/otelgorm"
  "gorm.io/gorm"
  ...
)

var db *gorm.DB

func main() {
  ...
  db, err := gorm.Open(postgres.Open(connStr))
  ...
  // Add the instrumentation
  err = db.Use(otelgorm.NewPlugin())
  ...
}

func handler(w http.ResponseWriter, r *http.Request) {
  ...
  // context propagation using r.Context()
  err := db.WithContext(r.Context()).Where("brand = ?", brand).Find(&products).Error
  ...
}
```

Client span attributes:

<img alt="Go GORM client Span Attributes" src="/img/docs/go_gorm_span_attributes.png" class="card w-1200"/>

## Redis client

[go-redis](https://github.com/redis/go-redis) provides a hook that instruments Redis calls with OpenTelemetry.

```go
import (
  "github.com/go-redis/redis/extra/redisotel/v8"
  ...
)

var db *redis.Client

func main() {
  db = redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})
  
  // Initialize the instrumentation hook
  db.AddHook(redisotel.NewTracingHook())
  ...
}

func handler(w http.ResponseWriter, r *http.Request) {
  // context propagation using r.Context()
  cmd := db.SAdd(r.Context(), cartId, productId)
  if cmd.Err() != nil {
    ...
  }
}
```

Client span attributes:

<img alt="Go Redis client Span Attributes" src="/img/docs/go_redis_client_span_attributes.png" class="card w-1200"/>

## Memcached client

You can instrument Memcached calls by creating an instrumented Memcached client that wraps a regular Memcached client with `otelmemcache`.

```go
import (
  "go.opentelemetry.io/contrib/instrumentation/github.com/bradfitz/gomemcache/memcache/otelmemcache"
  ...
)

var cache *otelmemcache.Client

func main() {
  // Initialize the instrumented client
  cache = otelmemcache.NewClientWithTracing(memcache.New("127.0.0.1:11211"))  
  ...
}

func handler(w http.ResponseWriter, r *http.Request) {
  // context propagation using r.Context()
  item, err := cache.WithContext(r.Context()).Get(sessionId)
  ...
}
```

Client span attributes:

<img alt="Go Memcached client Span Attributes" src="/img/docs/go_memcached_client_span_attributes.png" class="card w-1200"/>

## MongoDB client

```go 
import (
  "go.mongodb.org/mongo-driver/mongo"
  "go.mongodb.org/mongo-driver/mongo/options"
  "go.opentelemetry.io/contrib/instrumentation/go.mongodb.org/mongo-driver/mongo/otelmongo"
  ...
)

var db *mongo.Client

func main() {
  ...
  opts := options.Client()
  opts.ApplyURI("mongodb://127.0.0.1:27017")
  
  // Add the instrumentation to the client
  opts.Monitor = otelmongo.NewMonitor()
  
  db, err = mongo.Connect(ctx, opts)
  ...
}

func handler(w http.ResponseWriter, r *http.Request) {
  ...
  collection := db.Database("orders").Collection("orders")
  
  // context propagation using r.Context()
  _, err = collection.InsertOne(r.Context(), bson.D{
    {"user_id", userId},
    {"products", productIds},
    {"total", total},
    {"Address", user.Address},
  })
  ...
}
```

Client span attributes:

<img alt="Go Mongodb client Span Attributes" src="/img/docs/go_mongodb_client_span_attributes.png" class="card w-1200"/>

## Cassandra client

```go
import (
  "github.com/gocql/gocql"
  "go.opentelemetry.io/contrib/instrumentation/github.com/gocql/gocql/otelgocql"
  ...
)

var db *gocql.Session

func main() {
  ...
  cluster := gocql.NewCluster("127.0.0.1:9042")
  db, err = cluster.CreateSession()
  ...
  // Add the instrumentation to the client
  db, err = otelgocql.NewSessionWithTracing(context.Background(), cluster)
  ...
}

func handler(w http.ResponseWriter, r *http.Request) {
  ...
  // context propagation using r.Context()
  err := db.
    Query(`INSERT INTO log (email, ts, event) VALUES (?, ?, ?)`, email, now, "login").
    WithContext(r.Context()).
    Exec()
  ...
}
```

Client span attributes:

<img alt="Go Cassandra client Span Attributes" src="/img/docs/go_cassandra_client_span_attributes.png" class="card w-1200"/>

## Whats next?

- Visit [OpenTelemetry Go Docs](https://opentelemetry.io/docs/languages/go/)

- Check instumented packages in [OpenTelemetry Registry](https://opentelemetry.io/ecosystem/registry/?language=go&component=instrumentation)