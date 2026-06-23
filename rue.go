// Package rue is a high-performance, extensible Go web framework.
// It provides REST API, WebSocket, gRPC, GraphQL, SSE, WebRTC, and QUIC/HTTP3 support.
package rue

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"
)

// HandlerFunc defines the handler function type
type HandlerFunc func(*Context)

// HandlersChain is a slice of HandlerFunc
type HandlersChain []HandlerFunc

// Last returns the final handler in the chain, or nil if empty
func (c HandlersChain) Last() HandlerFunc {
	if length := len(c); length > 0 {
		return c[length-1]
	}
	return nil
}

// H is a convenience alias for map[string]any, used for structured responses
type H map[string]any

// Engine is the core framework engine
type Engine struct {
	RouterGroup

	pool   sync.Pool
	router *Router

	// Configuration
	MaxMultipartMemory int64
	MaxRequestBodySize int64 // Maximum request body size (default: 4MB)
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

	// Stats reporter
	stats       *statsReporter
	statsConfig StatsConfig

	// Lifecycle hooks
	onStart    []func()
	onShutdown []func()
	onRequest  []func(*Context)
	onResponse []func(*Context)

	// Server
	server *http.Server

	// Security: Trusted proxies for X-Forwarded-For header
	trustedProxies    map[string]bool
	trustedProxyCIDRs []*net.IPNet

	// Router options
	RedirectTrailingSlash bool
}

// New creates a new Engine instance without any middleware
func New() *Engine {
	engine := &Engine{
		RouterGroup: RouterGroup{
			basePath: "/",
		},
		router:             newRouter(),
		MaxMultipartMemory: 32 << 20, // 32 MB
		MaxRequestBodySize: 4 << 20,  // 4 MB
		Mode:               GetMode(),
		statsConfig:        DefaultStatsConfig(),
	}
	engine.RouterGroup.engine = engine
	engine.pool.New = func() any {
		return engine.newContext()
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

// newContext creates a new Context bound to this engine
func (e *Engine) newContext() *Context {
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
		// Try trailing slash redirect if enabled
		if e.RedirectTrailingSlash && path != "/" {
			var redirectPath string
			if path[len(path)-1] == '/' {
				// Try without trailing slash
				redirectPath = path[:len(path)-1]
			} else {
				// Try with trailing slash
				redirectPath = path + "/"
			}

			if _, _, redirectFound := e.router.getValue(method, redirectPath, nil); redirectFound {
				// Redirect to the correct path
				code := http.StatusMovedPermanently // 301 for GET
				if method != http.MethodGet {
					code = http.StatusTemporaryRedirect // 307 for other methods
				}
				c.Redirect(code, redirectPath)
				return
			}
		}

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
	// Start stats reporter
	e.startStats()
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
	// Start stats reporter
	e.startStats()
	// Call onStart hooks
	for _, hook := range e.onStart {
		hook()
	}
	return e.server.ListenAndServeTLS(certFile, keyFile)
}

// Shutdown gracefully shuts down the server
func (e *Engine) Shutdown(ctx context.Context) error {
	// Stop stats reporter
	e.stopStats()
	// Call onShutdown hooks
	for _, hook := range e.onShutdown {
		hook()
	}
	if e.server != nil {
		return e.server.Shutdown(ctx)
	}
	return nil
}

// startStats starts the stats reporter
func (e *Engine) startStats() {
	if e.statsConfig.Enabled {
		e.stats = newStatsReporter(e.statsConfig, e.Logger)
		e.stats.start()
	}
}

// stopStats stops the stats reporter
func (e *Engine) stopStats() {
	if e.stats != nil {
		e.stats.stop()
	}
}

// SetStatsConfig sets the stats configuration
func (e *Engine) SetStatsConfig(config StatsConfig) *Engine {
	e.statsConfig = config
	return e
}

// DisableStats disables system statistics reporting
func (e *Engine) DisableStats() *Engine {
	e.statsConfig.Enabled = false
	return e
}

// SetStatsInterval sets the interval for stats reporting
func (e *Engine) SetStatsInterval(interval time.Duration) *Engine {
	e.statsConfig.Interval = interval
	return e
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

// SetTrustedProxies sets the list of trusted proxy IP addresses or CIDR ranges.
// Only requests from trusted proxies will have X-Forwarded-For and X-Real-IP headers trusted.
// Example: e.SetTrustedProxies([]string{"127.0.0.1", "10.0.0.0/8", "192.168.0.0/16"})
func (e *Engine) SetTrustedProxies(proxies []string) error {
	e.trustedProxies = make(map[string]bool)
	e.trustedProxyCIDRs = nil

	for _, proxy := range proxies {
		// Check if it's a CIDR range
		if _, cidr, err := net.ParseCIDR(proxy); err == nil {
			e.trustedProxyCIDRs = append(e.trustedProxyCIDRs, cidr)
		} else if ip := net.ParseIP(proxy); ip != nil {
			e.trustedProxies[ip.String()] = true
		} else {
			return &net.ParseError{Type: "IP address or CIDR", Text: proxy}
		}
	}
	return nil
}

// isTrustedProxy checks if the given IP is a trusted proxy
func (e *Engine) isTrustedProxy(ip string) bool {
	// If no trusted proxies configured, don't trust any
	if len(e.trustedProxies) == 0 && len(e.trustedProxyCIDRs) == 0 {
		return false
	}

	// Check exact IP match
	if e.trustedProxies[ip] {
		return true
	}

	// Check CIDR ranges
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	for _, cidr := range e.trustedProxyCIDRs {
		if cidr.Contains(parsedIP) {
			return true
		}
	}
	return false
}
