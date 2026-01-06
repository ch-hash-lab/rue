// Package rue is a high-performance, extensible Go web framework.
// It provides REST API, WebSocket, gRPC, GraphQL, SSE, WebRTC, and QUIC/HTTP3 support.
package rue

import (
	"context"
	"net/http"
	"sync"
)

// HandlerFunc defines the handler function type
type HandlerFunc func(*Context)

// HandlersChain is a slice of HandlerFunc
type HandlersChain []HandlerFunc

// Last returns the last handler in the chain
func (c HandlersChain) Last() HandlerFunc {
	if length := len(c); length > 0 {
		return c[length-1]
	}
	return nil
}

// H is a shortcut for map[string]any, commonly used for JSON responses
type H map[string]any

// Engine is the core framework engine
type Engine struct {
	RouterGroup

	pool   sync.Pool
	router *Router

	// Configuration
	MaxMultipartMemory int64
	Mode               Mode

	// Extension points
	Binder       Binder
	Validator    Validator
	Renderer     Renderer
	ErrorHandler ErrorHandler

	// HTML template renderer
	htmlRenderer *HTMLRenderer

	// Logger
	Logger *Logger

	// Lifecycle hooks
	onStart    []func()
	onShutdown []func()
	onRequest  []func(*Context)
	onResponse []func(*Context)

	// Server
	server *http.Server
}

// New creates a new Engine instance without any middleware
func New() *Engine {
	engine := &Engine{
		RouterGroup: RouterGroup{
			basePath: "/",
		},
		router:             newRouter(),
		MaxMultipartMemory: 32 << 20, // 32 MB
		Mode:               GetMode(),
	}
	engine.RouterGroup.engine = engine
	engine.pool.New = func() any {
		return engine.allocateContext()
	}
	// Set default components
	engine.Binder = &DefaultBinder{}
	engine.Validator = NewValidator()
	engine.ErrorHandler = DefaultErrorHandler
	engine.Logger = NewLogger()
	return engine
}

// Default creates an Engine with Logger and Recovery middleware
func Default() *Engine {
	engine := New()
	engine.Use(RequestLogger(), Recovery())
	return engine
}

// allocateContext creates a new Context
func (e *Engine) allocateContext() *Context {
	return &Context{engine: e}
}

// ServeHTTP implements the http.Handler interface
func (e *Engine) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := e.pool.Get().(*Context)
	c.reset(w, r)

	// Call onRequest hooks
	for _, hook := range e.onRequest {
		hook(c)
	}

	e.handleRequest(c)

	// Call onResponse hooks
	for _, hook := range e.onResponse {
		hook(c)
	}

	e.pool.Put(c)
}

// handleRequest processes the HTTP request
func (e *Engine) handleRequest(c *Context) {
	method := c.Request.Method
	path := c.Request.URL.Path

	handlers, fullPath, found := e.router.getValue(method, path, &c.Params)
	if found {
		c.handlers = handlers
		c.fullPath = fullPath
		c.Next()
	} else {
		c.handlers = e.RouterGroup.middleware
		c.Next()
		if !c.IsAborted() {
			e.handleNotFound(c)
		}
	}
}

// handleNotFound handles 404 responses
func (e *Engine) handleNotFound(c *Context) {
	c.AbortWithStatus(http.StatusNotFound)
}

// Run starts the HTTP server
func (e *Engine) Run(addr string) error {
	e.server = &http.Server{
		Addr:    addr,
		Handler: e,
	}
	// Call onStart hooks
	for _, hook := range e.onStart {
		hook()
	}
	return e.server.ListenAndServe()
}

// RunTLS starts the HTTPS server
func (e *Engine) RunTLS(addr, certFile, keyFile string) error {
	e.server = &http.Server{
		Addr:    addr,
		Handler: e,
	}
	// Call onStart hooks
	for _, hook := range e.onStart {
		hook()
	}
	return e.server.ListenAndServeTLS(certFile, keyFile)
}

// Shutdown gracefully shuts down the server
func (e *Engine) Shutdown(ctx context.Context) error {
	// Call onShutdown hooks
	for _, hook := range e.onShutdown {
		hook()
	}
	if e.server != nil {
		return e.server.Shutdown(ctx)
	}
	return nil
}

// OnStart registers a hook to be called when the server starts
func (e *Engine) OnStart(fn func()) {
	e.onStart = append(e.onStart, fn)
}

// OnShutdown registers a hook to be called when the server shuts down
func (e *Engine) OnShutdown(fn func()) {
	e.onShutdown = append(e.onShutdown, fn)
}

// OnRequest registers a hook to be called for each request
func (e *Engine) OnRequest(fn func(*Context)) {
	e.onRequest = append(e.onRequest, fn)
}

// OnResponse registers a hook to be called after each response
func (e *Engine) OnResponse(fn func(*Context)) {
	e.onResponse = append(e.onResponse, fn)
}
