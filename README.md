# Rue

A high-performance, feature-rich web framework for Go with minimal dependencies.

## Features

- **High-Performance Router**: Segment-trie routing with ~6.5ns static route matching
- **Middleware Support**: Logger, Recovery, CORS, Rate Limiter, JWT, API Key
- **Compression**: Gzip and Brotli with automatic content negotiation
- **Data Binding**: JSON, XML, Form, Query, Header binding with validation
- **WebSocket**: Full RFC 6455 implementation with Hub for broadcasting
- **SSE**: Server-Sent Events support
- **GraphQL**: Built-in GraphQL handler with Playground
- **WebRTC**: Signaling server for peer-to-peer connections
- **QUIC/HTTP3**: HTTP/3 support via quic-go
- **Testing**: Built-in Agent for integration testing

## Installation

```bash
go get github.com/ch-hash-lab/rue
```

## Quick Start

```go
package main

import (
    "net/http"
    "github.com/ch-hash-lab/rue"
)

func main() {
    // Create engine with default middleware (Logger + Recovery)
    r := rue.Default()

    // Simple route
    r.GET("/", func(c *rue.Context) {
        c.JSON(http.StatusOK, rue.H{"message": "Hello, Rue!"})
    })

    // Path parameters
    r.GET("/users/:id", func(c *rue.Context) {
        id := c.Param("id")
        c.JSON(http.StatusOK, rue.H{"user_id": id})
    })

    // Query parameters
    r.GET("/search", func(c *rue.Context) {
        q := c.Query("q")
        page := c.DefaultQuery("page", "1")
        c.JSON(http.StatusOK, rue.H{"query": q, "page": page})
    })

    r.Run(":8080")
}
```

## Routing

### Basic Routes

```go
r.GET("/path", handler)
r.POST("/path", handler)
r.PUT("/path", handler)
r.DELETE("/path", handler)
r.PATCH("/path", handler)
r.HEAD("/path", handler)
r.OPTIONS("/path", handler)
r.Any("/path", handler)  // All methods
```

### Route Groups

```go
api := r.Group("/api")
{
    api.GET("/users", listUsers)

    v1 := api.Group("/v1")
    v1.Use(authMiddleware)
    {
        v1.GET("/posts", listPosts)
        v1.POST("/posts", createPost)
    }
}
```

### Path Parameters

```go
// Named parameter
r.GET("/users/:id", func(c *rue.Context) {
    id := c.Param("id")  // /users/123 -> id = "123"
})

// Wildcard parameter
r.GET("/files/*filepath", func(c *rue.Context) {
    path := c.Param("filepath")  // /files/docs/readme.md -> filepath = "docs/readme.md"
})
```

## Request Handling

### Data Binding

```go
type CreateUser struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"min=18,max=120"`
}

r.POST("/users", func(c *rue.Context) {
    var user CreateUser

    // Bind and validate
    if err := c.BindJSON(&user); err != nil {
        c.AbortWithStatus(http.StatusBadRequest)
        return
    }

    if err := c.Validate(&user); err != nil {
        c.JSON(http.StatusBadRequest, rue.H{"errors": err})
        return
    }

    c.JSON(http.StatusCreated, user)
})
```

### Validation Rules

- `required` - Field must not be empty
- `min=N` - Minimum value/length
- `max=N` - Maximum value/length
- `len=N` - Exact length
- `email` - Valid email format
- `url` - Valid URL format
- `regex=pattern` - Match regex pattern
- `oneof=a b c` - Value must be one of listed options

### Response Methods

```go
c.JSON(http.StatusOK, obj)           // JSON response
c.XML(http.StatusOK, obj)            // XML response
c.String(http.StatusOK, "fmt", args) // Formatted string (printf-style)
c.Text(http.StatusOK, "text")        // Plain text
c.Data(http.StatusOK, "text/html", []byte("<h1>Hi</h1>"))
c.File("path/to/file")
c.Stream(http.StatusOK, "application/octet-stream", reader)
c.Redirect(http.StatusFound, "/new-path")
```

## Middleware

### Built-in Middleware

```go
// Logger - logs requests
r.Use(rue.RequestLogger())
r.Use(rue.LoggerWithConfig(rue.LoggerConfig{
    SkipPaths: []string{"/health"},
}))

// Recovery - recovers from panics
r.Use(rue.Recovery())

// CORS
r.Use(rue.CORS())
r.Use(rue.CORSWithConfig(rue.CORSConfig{
    AllowOrigins:     []string{"https://example.com"},
    AllowMethods:     []string{"GET", "POST"},
    AllowCredentials: true,
}))

// Rate Limiter
r.Use(rue.RateLimiter(10, 20))  // 10 req/sec, burst 20
r.Use(rue.RateLimiterWithConfig(rue.RateLimiterConfig{
    Rate:  100,
    Burst: 200,
    KeyFunc: func(c *rue.Context) string {
        return c.ClientIP()
    },
}))

// Compression
r.Use(rue.Gzip())      // Gzip only
r.Use(rue.Brotli())    // Brotli only
r.Use(rue.Compress())  // Auto-select (prefers Brotli)
```

### JWT Authentication

```go
secret := []byte("your-secret-key")

// Generate token
claims := &rue.JWTClaims{
    Subject:   "user123",
    ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
    Custom: map[string]any{
        "role": "admin",
    },
}
token, _ := rue.GenerateToken(claims, secret)

// Protect routes
r.Use(rue.JWT(secret))

r.GET("/protected", func(c *rue.Context) {
    claims, _ := c.Get("jwt_claims")
    c.JSON(http.StatusOK, claims)
})
```

### API Key Authentication

```go
r.Use(rue.APIKey(func(key string) bool {
    return key == "valid-api-key"
}))

// Or with config
r.Use(rue.APIKeyWithConfig(rue.APIKeyConfig{
    KeyLookup: "query:api_key",  // or "header:X-API-Key"
    Validator: func(key string) bool {
        return validateKey(key)
    },
}))
```

### Custom Middleware

```go
func MyMiddleware() rue.HandlerFunc {
    return func(c *rue.Context) {
        // Before request
        start := time.Now()

        c.Next()  // Process request

        // After request
        latency := time.Since(start)
        log.Printf("Request took %v", latency)
    }
}
```

## WebSocket

```go
r.GET("/ws", func(c *rue.Context) {
    handler := &rue.WebSocketHandler{
        OnConnect: func(conn *rue.WebSocketConn) {
            log.Println("Client connected")
        },
        OnMessage: func(conn *rue.WebSocketConn, msgType int, data []byte) {
            conn.Send(msgType, data)  // Echo
        },
        OnClose: func(conn *rue.WebSocketConn) {
            log.Println("Client disconnected")
        },
    }
    handler.Handle(c)
})

// With Hub for broadcasting
hub := rue.NewWebSocketHub()
go hub.Run()

r.GET("/ws", func(c *rue.Context) {
    handler := &rue.WebSocketHandler{
        Hub: hub,
        OnMessage: func(conn *rue.WebSocketConn, msgType int, data []byte) {
            hub.Broadcast(msgType, data)  // Send to all
        },
    }
    handler.Handle(c)
})
```

## Server-Sent Events (SSE)

```go
r.GET("/events", func(c *rue.Context) {
    client := rue.NewSSEClient(c)
    defer client.Close()

    for i := 0; i < 10; i++ {
        client.SendEvent("update", fmt.Sprintf("Event %d", i))
        time.Sleep(time.Second)
    }
})
```

## GraphQL

```go
schema := &rue.GraphQLSchema{
    Query: &rue.ObjectType{
        Name: "Query",
        Fields: map[string]*rue.Field{
            "hello": {
                Type: rue.StringType,
                Resolve: func(ctx *rue.ResolveContext) (any, error) {
                    return "Hello, World!", nil
                },
            },
            "user": {
                Type: userType,
                Args: map[string]*rue.Argument{
                    "id": {Type: rue.StringType},
                },
                Resolve: func(ctx *rue.ResolveContext) (any, error) {
                    id := ctx.Args["id"].(string)
                    return getUser(id), nil
                },
            },
        },
    },
}

handler := rue.NewGraphQLHandler(schema)
r.POST("/graphql", handler.Handle)
r.GET("/graphql", handler.Playground)  // GraphQL Playground UI
```

## WebRTC Signaling

```go
signaling := rue.NewSignalingServer()

r.GET("/webrtc", func(c *rue.Context) {
    signaling.Handle(c)
})

// Client sends JSON messages:
// {"type": "join", "room": "room1"}
// {"type": "offer", "sdp": "..."}
// {"type": "answer", "sdp": "..."}
// {"type": "candidate", "candidate": "..."}
```

## QUIC/HTTP3

```go
// Run HTTP/3 server
r.RunQUIC(":443", "cert.pem", "key.pem")

// With custom config
config := &rue.QUICConfig{
    MaxIncomingStreams: 100,
    IdleTimeout:        30 * time.Second,
    Enable0RTT:         true,
}
r.RunQUICWithConfig(":443", "cert.pem", "key.pem", config)

// Alt-Svc middleware (advertise HTTP/3 on HTTP/1.1 or HTTP/2)
r.Use(rue.AltSvc(":443"))
```

## gRPC Integration

Rue provides seamless gRPC integration with middleware support.

```go
import (
    "google.golang.org/grpc"
    "github.com/ch-hash-lab/rue"
)

// Create gRPC server with Rue engine
engine := rue.New()
gs := rue.NewGRPCServer(engine)

// Add Rue middleware as gRPC interceptors
gs.Use(func(c *rue.Context) {
    // This middleware runs for every gRPC call
    c.Set("request_id", generateID())
})

// Add native gRPC interceptors
gs.UseUnary(rue.GRPCLogger())
gs.UseUnary(rue.GRPCRecovery())

// Build and register services
server := gs.Build()
pb.RegisterMyServiceServer(server, &myService{})

// Run server
gs.Run(":50051")
```

### gRPC Authentication

```go
// JWT-style authentication for gRPC
authFunc := func(ctx context.Context) (context.Context, error) {
    md, ok := metadata.FromIncomingContext(ctx)
    if !ok {
        return nil, status.Error(codes.Unauthenticated, "no metadata")
    }

    tokens := md.Get("authorization")
    if len(tokens) == 0 {
        return nil, status.Error(codes.Unauthenticated, "no token")
    }

    // Validate token and add user to context
    user, err := validateToken(tokens[0])
    if err != nil {
        return nil, status.Error(codes.Unauthenticated, "invalid token")
    }

    return context.WithValue(ctx, "user", user), nil
}

gs.UseUnary(rue.GRPCAuth(authFunc))
gs.UseStream(rue.GRPCStreamAuth(authFunc))
```

### Accessing Context in gRPC Handlers

```go
func (s *myService) MyMethod(ctx context.Context, req *pb.Request) (*pb.Response, error) {
    // Get Rue's GRPCContext
    gc := rue.GetGRPCContext(ctx)
    if gc != nil {
        // Access values set by middleware
        requestID, _ := gc.Get("request_id")

        // Access metadata
        auth := gc.GetMetadata("authorization")
    }

    return &pb.Response{}, nil
}
```

## Testing with Agent

```go
func TestAPI(t *testing.T) {
    r := rue.New()
    r.GET("/users/:id", getUser)

    agent := rue.NewAgent(r)

    // Simple GET
    resp := agent.Get("/users/123")
    if resp.StatusCode != http.StatusOK {
        t.Errorf("Expected 200, got %d", resp.StatusCode)
    }

    // POST with JSON
    resp = agent.PostJSON("/users", map[string]string{
        "name": "John",
    })

    // With headers
    agent.SetHeader("Authorization", "Bearer token")
    resp = agent.Get("/protected")

    // Request builder
    resp = agent.NewRequest("POST", "/search").
        Query("q", "test").
        Header("X-Custom", "value").
        JSON(map[string]string{"filter": "active"}).
        Send()

    // Parse response
    var result map[string]any
    resp.JSON(&result)
}
```

## Error Handling

```go
// Predefined errors
rue.ErrBadRequest          // 400
rue.ErrUnauthorized        // 401
rue.ErrForbidden           // 403
rue.ErrNotFound            // 404
rue.ErrInternalServerError // 500

// Custom error
err := rue.NewError(http.StatusConflict, "Resource already exists")
err = err.WithDetails(map[string]string{"field": "email"})

// In handler
r.GET("/resource", func(c *rue.Context) {
    if notFound {
        c.AbortWithError(http.StatusNotFound, rue.ErrNotFound)
        return
    }
})

// Custom error handler
r.ErrorHandler = func(c *rue.Context, err error) {
    // Custom error handling logic
}
```

## Lifecycle Hooks

```go
r.OnRequest(func(c *rue.Context) {
    // Called before each request
})

r.OnResponse(func(c *rue.Context) {
    // Called after each request
})

r.OnStart(func() {
    log.Println("Server starting...")
})

r.OnShutdown(func() {
    log.Println("Server shutting down...")
})
```

## Graceful Shutdown

```go
r := rue.Default()
// ... setup routes

go func() {
    if err := r.Run(":8080"); err != nil {
        log.Fatal(err)
    }
}()

// Wait for interrupt signal
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

// Graceful shutdown with timeout
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
r.Shutdown(ctx)
```

## Security Configuration

### Trusted Proxies

By default, Rue does not trust any proxy headers (X-Forwarded-For, X-Real-IP). If your application is behind a reverse proxy, configure trusted proxies:

```go
r := rue.New()

// Trust localhost and private networks
r.SetTrustedProxies([]string{
    "127.0.0.1",
    "10.0.0.0/8",
    "172.16.0.0/12",
    "192.168.0.0/16",
})
```

### Request Body Size Limit

Rue limits request body size to 4MB by default to prevent DoS attacks:

```go
r := rue.New()
r.MaxRequestBodySize = 10 << 20  // 10MB
```

### Trailing Slash Redirect

Enable automatic redirect between `/path` and `/path/`:

```go
r := rue.New()
r.RedirectTrailingSlash = true
```

## HTML Templates

```go
r := rue.New()

// Set custom template functions
r.SetFuncMap(template.FuncMap{
    "formatDate": func(t time.Time) string {
        return t.Format("2006-01-02")
    },
})

// Load templates from glob pattern
r.LoadHTMLGlob("templates/*.html")

// Or load from multiple levels with **
r.LoadHTMLGlob("templates/**/*.html")

// Or load specific files
r.LoadHTMLFiles("templates/index.html", "templates/about.html")

// Render template
r.GET("/", func(c *rue.Context) {
    c.HTML(http.StatusOK, "index.html", rue.H{
        "title": "Home",
        "user":  user,
    })
})
```

## Benchmarks

```
BenchmarkRouter_StaticRoute-8       181342215    6.640 ns/op     0 B/op    0 allocs/op
BenchmarkRouter_ParamRoute-8         38045265   31.88 ns/op     32 B/op    1 allocs/op
BenchmarkRouter_ManyRoutes-8         24370306   48.98 ns/op     64 B/op    1 allocs/op
BenchmarkEngine_SimpleRoute-8         2905870  420.9 ns/op    1064 B/op   11 allocs/op
BenchmarkEngine_JSONResponse-8        1240557  982.0 ns/op    1632 B/op   16 allocs/op
BenchmarkGzip-8                         27502   42767 ns/op   30131 B/op   16 allocs/op
BenchmarkBrotli-8                       60792   19511 ns/op   30360 B/op   15 allocs/op
```

## Dependencies

- [github.com/bytedance/sonic](https://github.com/bytedance/sonic) - High-performance JSON
- [github.com/andybalholm/brotli](https://github.com/andybalholm/brotli) - Brotli compression
- [github.com/quic-go/quic-go](https://github.com/quic-go/quic-go) - QUIC/HTTP3 support
- [google.golang.org/grpc](https://google.golang.org/grpc) - gRPC support

## Originality

All code in this repository (v0.0.7+) is original work. The routing engine
uses a segment-trie design with run-compressed literal chains, iterative
backtracking, and method-switch dispatch — an independent architecture
developed from first principles. Historical versions (v0.0.1–v0.0.6) contained
code derived from gin and httprouter; see NOTICE for attribution.

## License

MIT License
