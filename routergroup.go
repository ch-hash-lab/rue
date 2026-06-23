package rue

import (
	"net/http"
	"path"
	"slices"
	"strings"
)

// RouterGroup is used to configure routes with common prefix and middleware
type RouterGroup struct {
	basePath   string
	middleware HandlersChain
	engine     *Engine
	parent     *RouterGroup
}

// Use adds middleware to the group
func (g *RouterGroup) Use(middleware ...HandlerFunc) *RouterGroup {
	g.middleware = append(g.middleware, middleware...)
	return g
}

// Group creates a new router group with the given path prefix
func (g *RouterGroup) Group(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	return &RouterGroup{
		basePath:   g.fullPath(relativePath),
		middleware: g.mergeChain(handlers),
		engine:     g.engine,
		parent:     g,
	}
}

// Handle registers a new request handler with the given path and method
func (g *RouterGroup) Handle(method, relativePath string, handlers ...HandlerFunc) *RouterGroup {
	absolutePath := g.fullPath(relativePath)
	handlers = g.mergeChain(handlers)
	g.engine.router.addRoute(method, absolutePath, handlers)
	return g
}

// GET registers a GET handler
func (g *RouterGroup) GET(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	return g.Handle(http.MethodGet, relativePath, handlers...)
}

// POST registers a POST handler
func (g *RouterGroup) POST(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	return g.Handle(http.MethodPost, relativePath, handlers...)
}

// PUT registers a PUT handler
func (g *RouterGroup) PUT(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	return g.Handle(http.MethodPut, relativePath, handlers...)
}

// DELETE registers a DELETE handler
func (g *RouterGroup) DELETE(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	return g.Handle(http.MethodDelete, relativePath, handlers...)
}

// PATCH registers a PATCH handler
func (g *RouterGroup) PATCH(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	return g.Handle(http.MethodPatch, relativePath, handlers...)
}

// HEAD registers a HEAD handler
func (g *RouterGroup) HEAD(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	return g.Handle(http.MethodHead, relativePath, handlers...)
}

// OPTIONS registers an OPTIONS handler
func (g *RouterGroup) OPTIONS(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	return g.Handle(http.MethodOptions, relativePath, handlers...)
}

// CONNECT registers a CONNECT handler
func (g *RouterGroup) CONNECT(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	return g.Handle(http.MethodConnect, relativePath, handlers...)
}

// TRACE registers a TRACE handler
func (g *RouterGroup) TRACE(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	return g.Handle(http.MethodTrace, relativePath, handlers...)
}

// Any registers a handler for all HTTP methods
func (g *RouterGroup) Any(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	g.GET(relativePath, handlers...)
	g.POST(relativePath, handlers...)
	g.PUT(relativePath, handlers...)
	g.DELETE(relativePath, handlers...)
	g.PATCH(relativePath, handlers...)
	g.HEAD(relativePath, handlers...)
	g.OPTIONS(relativePath, handlers...)
	g.CONNECT(relativePath, handlers...)
	g.TRACE(relativePath, handlers...)
	return g
}

// Static serves files from the given file system root
func (g *RouterGroup) Static(relativePath, root string) *RouterGroup {
	return g.StaticFS(relativePath, http.Dir(root))
}

// StaticFile registers a single route to serve a single file
func (g *RouterGroup) StaticFile(relativePath, filepath string) *RouterGroup {
	handler := func(c *Context) {
		c.File(filepath)
	}
	g.GET(relativePath, handler)
	g.HEAD(relativePath, handler)
	return g
}

// StaticFS serves files from the given file system
func (g *RouterGroup) StaticFS(relativePath string, fs http.FileSystem) *RouterGroup {
	absolutePath := g.fullPath(relativePath)
	fileServer := http.StripPrefix(absolutePath, http.FileServer(fs))

	handler := func(c *Context) {
		file := c.Param("filepath")
		f, err := fs.Open(file)
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		f.Close()
		fileServer.ServeHTTP(c.Writer, c.Request)
	}

	urlPattern := path.Join(relativePath, "/*filepath")
	g.GET(urlPattern, handler)
	g.HEAD(urlPattern, handler)
	return g
}

// fullPath returns the absolute path for the given relative path
func (g *RouterGroup) fullPath(relativePath string) string {
	return resolvePath(g.basePath, relativePath)
}

// mergeChain produces a new handler chain combining group middleware with route handlers
func (g *RouterGroup) mergeChain(handlers HandlersChain) HandlersChain {
	return slices.Concat(g.middleware, handlers)
}

// resolvePath joins a base and relative path, preserving a trailing slash on rel
func resolvePath(base, rel string) string {
	if rel == "" {
		return base
	}
	joined := path.Join(base, rel)
	if strings.HasSuffix(rel, "/") && !strings.HasSuffix(joined, "/") {
		joined += "/"
	}
	return joined
}
