package rue

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
)

// Context is the request context that carries request-scoped data
type Context struct {
	Request *http.Request
	Writer  ResponseWriter

	Params   Params
	handlers HandlersChain
	cursor   int
	aborted  bool
	fullPath string

	mu    sync.RWMutex
	store map[string]any

	engine *Engine

	// Errors
	Errors []error
}

// reset resets the context for reuse
func (c *Context) reset(w http.ResponseWriter, r *http.Request) {
	c.Writer = &responseWriter{ResponseWriter: w, status: http.StatusOK}
	c.Request = r
	c.Params = c.Params[:0]
	c.handlers = nil
	c.cursor = -1
	c.aborted = false
	c.fullPath = ""
	c.store = nil
	c.Errors = c.Errors[:0]
}

// ============== Flow Control ==============

// Next executes the pending handlers in the chain
func (c *Context) Next() {
	c.cursor++
	for c.cursor < len(c.handlers) {
		if c.aborted {
			return
		}
		c.handlers[c.cursor](c)
		c.cursor++
	}
}

// Abort prevents pending handlers from being called
func (c *Context) Abort() {
	c.aborted = true
}

// AbortWithStatus aborts with the specified status code
func (c *Context) AbortWithStatus(code int) {
	c.Status(code)
	c.Writer.WriteHeaderNow()
	c.Abort()
}

// AbortWithJSON aborts with a JSON response
func (c *Context) AbortWithJSON(code int, obj any) {
	c.Abort()
	c.JSON(code, obj)
}

// IsAborted returns true if the context was aborted
func (c *Context) IsAborted() bool {
	return c.aborted
}

// ============== Request Data ==============

// Param returns the value of the URL param
func (c *Context) Param(key string) string {
	return c.Params.ByName(key)
}

// Query returns the query string parameter value
func (c *Context) Query(key string) string {
	return c.Request.URL.Query().Get(key)
}

// QueryDefault returns the query string parameter value or a default
func (c *Context) QueryDefault(key, defaultValue string) string {
	if value := c.Query(key); value != "" {
		return value
	}
	return defaultValue
}

// DefaultQuery is an alias for QueryDefault for compatibility
func (c *Context) DefaultQuery(key, defaultValue string) string {
	return c.QueryDefault(key, defaultValue)
}

// QueryArray returns a slice of strings for a given query key
func (c *Context) QueryArray(key string) []string {
	return c.Request.URL.Query()[key]
}

// QueryMap returns a map for a given query key
func (c *Context) QueryMap(key string) map[string]string {
	return c.queryMap(c.Request.URL.Query(), key)
}

func (c *Context) queryMap(values url.Values, key string) map[string]string {
	dicts := make(map[string]string)
	for k, v := range values {
		if i := strings.IndexByte(k, '['); i >= 1 && k[0:i] == key {
			if j := strings.IndexByte(k[i+1:], ']'); j >= 1 {
				dicts[k[i+1:][:j]] = v[0]
			}
		}
	}
	return dicts
}

// PostForm returns the form data value
func (c *Context) PostForm(key string) string {
	return c.Request.PostFormValue(key)
}

// PostFormDefault returns the form data value or a default
func (c *Context) PostFormDefault(key, defaultValue string) string {
	if value := c.PostForm(key); value != "" {
		return value
	}
	return defaultValue
}

// PostFormArray returns a slice of strings for a given form key
func (c *Context) PostFormArray(key string) []string {
	c.Request.ParseMultipartForm(c.engine.MaxMultipartMemory)
	return c.Request.PostForm[key]
}

// FormFile returns the first file for the provided form key
func (c *Context) FormFile(name string) (*multipart.FileHeader, error) {
	if c.Request.MultipartForm == nil {
		if err := c.Request.ParseMultipartForm(c.engine.MaxMultipartMemory); err != nil {
			return nil, err
		}
	}
	f, fh, err := c.Request.FormFile(name)
	if err != nil {
		return nil, err
	}
	f.Close()
	return fh, nil
}

// MultipartForm returns the multipart form
func (c *Context) MultipartForm() (*multipart.Form, error) {
	err := c.Request.ParseMultipartForm(c.engine.MaxMultipartMemory)
	return c.Request.MultipartForm, err
}

// Cookie returns the named cookie
func (c *Context) Cookie(name string) (string, error) {
	cookie, err := c.Request.Cookie(name)
	if err != nil {
		return "", err
	}
	return url.QueryUnescape(cookie.Value)
}

// Header returns the request header value
func (c *Context) Header(key string) string {
	return c.Request.Header.Get(key)
}

// ContentType returns the Content-Type header
func (c *Context) ContentType() string {
	return filterFlags(c.Header("Content-Type"))
}

// ClientIP returns the client IP address
// Only trusts X-Forwarded-For and X-Real-IP headers if the request comes from a trusted proxy
func (c *Context) ClientIP() string {
	// Get remote IP
	remoteIP := c.Request.RemoteAddr
	if ip, _, err := net.SplitHostPort(remoteIP); err == nil {
		remoteIP = ip
	}

	// Only trust forwarding headers if request is from a trusted proxy
	if c.engine.isTrustedProxy(remoteIP) {
		// Check X-Forwarded-For
		if xff := c.Header("X-Forwarded-For"); xff != "" {
			// Get the first (client) IP from the chain
			if i := strings.Index(xff, ","); i > 0 {
				return strings.TrimSpace(xff[:i])
			}
			return strings.TrimSpace(xff)
		}
		// Check X-Real-IP
		if xri := c.Header("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}

	// Fall back to RemoteAddr
	return remoteIP
}

// FullPath returns the matched route full path
func (c *Context) FullPath() string {
	return c.fullPath
}

// ============== Binding ==============

// Bind binds the request body to obj based on Content-Type
func (c *Context) Bind(obj any) error {
	return c.engine.Binder.Bind(c, obj)
}

// BindJSON binds the JSON request body to obj
func (c *Context) BindJSON(obj any) error {
	return sonic.ConfigDefault.NewDecoder(c.Request.Body).Decode(obj)
}

// BindQuery binds the query string to obj
func (c *Context) BindQuery(obj any) error {
	return bindQuery(c.Request.URL.Query(), obj)
}

// ShouldBind binds the request body to obj, returns error without aborting
func (c *Context) ShouldBind(obj any) error {
	return c.Bind(obj)
}

// ShouldBindJSON binds the JSON request body to obj
func (c *Context) ShouldBindJSON(obj any) error {
	return c.BindJSON(obj)
}

// Validate validates the given struct using the engine's validator
func (c *Context) Validate(obj any) error {
	if c.engine.Validator != nil {
		return c.engine.Validator.Validate(obj)
	}
	return nil
}

// ============== Response ==============

// Status sets the HTTP response status code
func (c *Context) Status(code int) *Context {
	c.Writer.WriteHeader(code)
	return c
}

// SetHeader sets a response header
func (c *Context) SetHeader(key, value string) {
	c.Writer.Header().Set(key, value)
}

// SetCookie sets a cookie
func (c *Context) SetCookie(name, value string, maxAge int, path, domain string, secure, httpOnly bool) {
	if path == "" {
		path = "/"
	}
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		MaxAge:   maxAge,
		Path:     path,
		Domain:   domain,
		Secure:   secure,
		HttpOnly: httpOnly,
	})
}

// JSON renders a JSON response
func (c *Context) JSON(code int, obj any) error {
	c.SetHeader("Content-Type", "application/json; charset=utf-8")
	c.Status(code)
	return sonic.ConfigDefault.NewEncoder(c.Writer).Encode(obj)
}

// IndentedJSON renders an indented JSON response
func (c *Context) IndentedJSON(code int, obj any) error {
	c.SetHeader("Content-Type", "application/json; charset=utf-8")
	c.Status(code)
	encoder := sonic.ConfigDefault.NewEncoder(c.Writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(obj)
}

// XML renders an XML response
func (c *Context) XML(code int, obj any) error {
	c.SetHeader("Content-Type", "application/xml; charset=utf-8")
	c.Status(code)
	return xml.NewEncoder(c.Writer).Encode(obj)
}

// HTML renders an HTML response using the engine's renderer
func (c *Context) HTML(code int, name string, data any) error {
	c.SetHeader("Content-Type", "text/html; charset=utf-8")
	c.Status(code)
	if c.engine.Renderer != nil {
		return c.engine.Renderer.Render(c.Writer, name, data)
	}
	return fmt.Errorf("no renderer configured")
}

// String renders a string response with optional format arguments
func (c *Context) String(code int, format string, values ...any) error {
	c.SetHeader("Content-Type", "text/plain; charset=utf-8")
	c.Status(code)
	if len(values) > 0 {
		_, err := c.Writer.Write([]byte(sprintf(format, values...)))
		return err
	}
	_, err := c.Writer.Write([]byte(format))
	return err
}

// Text renders a plain text response (no formatting)
func (c *Context) Text(code int, text string) error {
	c.SetHeader("Content-Type", "text/plain; charset=utf-8")
	c.Status(code)
	_, err := c.Writer.Write([]byte(text))
	return err
}

// Data writes raw bytes to the response
func (c *Context) Data(code int, contentType string, data []byte) error {
	c.SetHeader("Content-Type", contentType)
	c.Status(code)
	_, err := c.Writer.Write(data)
	return err
}

// File serves a file
func (c *Context) File(filepath string) {
	http.ServeFile(c.Writer, c.Request, filepath)
}

// Stream sends a streaming response
func (c *Context) Stream(code int, contentType string, reader io.Reader) error {
	c.SetHeader("Content-Type", contentType)
	c.Status(code)
	_, err := io.Copy(c.Writer, reader)
	return err
}

// Redirect redirects the request
func (c *Context) Redirect(code int, location string) {
	http.Redirect(c.Writer, c.Request, location, code)
}

// ============== Storage ==============

// Set stores a key-value pair in the context
func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.store == nil {
		c.store = make(map[string]any)
	}
	c.store[key] = value
}

// Get retrieves a value from the context
func (c *Context) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.store == nil {
		return nil, false
	}
	value, exists := c.store[key]
	return value, exists
}

// MustGet retrieves a value from the context, panics if not found
func (c *Context) MustGet(key string) any {
	value, exists := c.Get(key)
	if !exists {
		panic("Key \"" + key + "\" does not exist")
	}
	return value
}

// GetString retrieves a string value from the context
func (c *Context) GetString(key string) string {
	if val, ok := c.Get(key); ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// GetInt retrieves an int value from the context
func (c *Context) GetInt(key string) int {
	if val, ok := c.Get(key); ok {
		if i, ok := val.(int); ok {
			return i
		}
	}
	return 0
}

// GetBool retrieves a bool value from the context
func (c *Context) GetBool(key string) bool {
	if val, ok := c.Get(key); ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// ============== context.Context Interface ==============

// Deadline returns the time when work done on behalf of this context should be canceled
func (c *Context) Deadline() (deadline time.Time, ok bool) {
	if c.Request == nil || c.Request.Context() == nil {
		return
	}
	return c.Request.Context().Deadline()
}

// Done returns a channel that's closed when work done on behalf of this context should be canceled
func (c *Context) Done() <-chan struct{} {
	if c.Request == nil || c.Request.Context() == nil {
		return nil
	}
	return c.Request.Context().Done()
}

// Err returns the error value
func (c *Context) Err() error {
	if c.Request == nil || c.Request.Context() == nil {
		return nil
	}
	return c.Request.Context().Err()
}

// Value returns the value associated with this context for key
func (c *Context) Value(key any) any {
	if keyStr, ok := key.(string); ok {
		if val, exists := c.Get(keyStr); exists {
			return val
		}
	}
	if c.Request == nil || c.Request.Context() == nil {
		return nil
	}
	return c.Request.Context().Value(key)
}

// Context returns the request's context
func (c *Context) Context() context.Context {
	return c.Request.Context()
}

// ============== Helpers ==============

func filterFlags(content string) string {
	for i, char := range content {
		if char == ' ' || char == ';' {
			return content[:i]
		}
	}
	return content
}

func sprintf(format string, values ...any) string {
	if len(values) == 0 {
		return format
	}
	// Use fmt.Sprintf for proper formatting
	return fmt.Sprintf(format, values...)
}

// bindQuery binds query values to a struct
func bindQuery(values url.Values, obj any) error {
	return mapFormByTag(obj, values, "form")
}
