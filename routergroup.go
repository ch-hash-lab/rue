package rue

import (
	"net/http"
	"path"
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
		basePath:   g.calculateAbsolutePath(relativePath),
		middleware: g.combineHandlers(handlers),
		engine:     g.engine,
		parent:     g,
	}
}

// Handle registers a new request handler with the given path and method
func (g *RouterGroup) Handle(method, relativePath string, handlers ...HandlerFunc) *RouterGroup {
	absolutePath := g.calculateAbsolutePath(relativePath)
	handlers = g.combineHandlers(handlers)
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

// Any registers a handler for all HTTP methods
func (g *RouterGroup) Any(relativePath string, handlers ...HandlerFunc) *RouterGroup {
	g.GET(relativePath, handlers...)
	g.POST(relativePath, handlers...)
	g.PUT(relativePath, handlers...)
	g.DELETE(relativePath, handlers...)
	g.PATCH(relativePath, handlers...)
	g.HEAD(relativePath, handlers...)
	g.OPTIONS(relativePath, handlers...)
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
	absolutePath := g.calculateAbsolutePath(relativePath)
	fileServer := http.StripPrefix(absolutePath, http.FileServer(fs))

	handler := func(c *Context) {
		file := c.Param("filepath")
		// Check if file exists
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

// calculateAbsolutePath returns the absolute path for the given relative path
func (g *RouterGroup) calculateAbsolutePath(relativePath string) string {
	return joinPaths(g.basePath, relativePath)
}

// combineHandlers combines the group's middleware with the given handlers
func (g *RouterGroup) combineHandlers(handlers HandlersChain) HandlersChain {
	finalSize := len(g.middleware) + len(handlers)
	mergedHandlers := make(HandlersChain, finalSize)
	copy(mergedHandlers, g.middleware)
	copy(mergedHandlers[len(g.middleware):], handlers)
	return mergedHandlers
}

// joinPaths joins two paths
func joinPaths(absolutePath, relativePath string) string {
	if relativePath == "" {
		return absolutePath
	}

	finalPath := path.Join(absolutePath, relativePath)
	if lastChar(relativePath) == '/' && lastChar(finalPath) != '/' {
		return finalPath + "/"
	}
	return finalPath
}

// lastChar returns the last character of a string
func lastChar(str string) uint8 {
	if str == "" {
		return 0
	}
	return str[len(str)-1]
}
