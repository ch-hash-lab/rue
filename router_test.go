package rue

import (
	"net/http"
	"testing"
	"testing/quick"
)

// Property 1: Router Path Matching Correctness
// For any registered route with path parameters (:param) or wildcards (*path),
// and for any request path matching that pattern, the Router should correctly
// extract parameter values and match the most specific route.
// Validates: Requirements 1.3, 1.4, 1.5, 1.6

func TestRouter_StaticRoutes(t *testing.T) {
	router := newRouter()
	handler := HandlersChain{func(c *Context) {}}

	routes := []string{
		"/",
		"/users",
		"/users/list",
		"/api/v1/users",
		"/api/v1/posts",
		"/api/v2/users",
	}

	for _, route := range routes {
		router.addRoute(http.MethodGet, route, handler)
	}

	for _, route := range routes {
		var params Params
		handlers, fullPath, found := router.getValue(http.MethodGet, route, &params)
		if !found {
			t.Errorf("Route %s not found", route)
		}
		if fullPath != route {
			t.Errorf("FullPath = %s, want %s", fullPath, route)
		}
		if len(handlers) != 1 {
			t.Errorf("Handlers length = %d, want 1", len(handlers))
		}
	}
}

func TestRouter_ParamRoutes(t *testing.T) {
	router := newRouter()
	handler := HandlersChain{func(c *Context) {}}

	router.addRoute(http.MethodGet, "/users/:id", handler)
	router.addRoute(http.MethodGet, "/users/:id/posts", handler)
	router.addRoute(http.MethodGet, "/users/:id/posts/:postId", handler)

	tests := []struct {
		path       string
		wantParams Params
		wantFound  bool
	}{
		{"/users/123", Params{{Key: "id", Value: "123"}}, true},
		{"/users/abc", Params{{Key: "id", Value: "abc"}}, true},
		{"/users/123/posts", Params{{Key: "id", Value: "123"}}, true},
		{"/users/123/posts/456", Params{{Key: "id", Value: "123"}, {Key: "postId", Value: "456"}}, true},
		{"/users", Params{}, false},
		{"/users/", Params{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			var params Params
			_, _, found := router.getValue(http.MethodGet, tt.path, &params)
			if found != tt.wantFound {
				t.Errorf("Found = %v, want %v", found, tt.wantFound)
			}
			if found && len(params) != len(tt.wantParams) {
				t.Errorf("Params = %v, want %v", params, tt.wantParams)
			}
			for i, p := range tt.wantParams {
				if i < len(params) && (params[i].Key != p.Key || params[i].Value != p.Value) {
					t.Errorf("Param[%d] = %v, want %v", i, params[i], p)
				}
			}
		})
	}
}

func TestRouter_WildcardRoutes(t *testing.T) {
	router := newRouter()
	handler := HandlersChain{func(c *Context) {}}

	router.addRoute(http.MethodGet, "/static/*filepath", handler)
	router.addRoute(http.MethodGet, "/files/*path", handler)

	tests := []struct {
		path      string
		wantParam string
		wantValue string
		wantFound bool
	}{
		{"/static/css/style.css", "filepath", "/css/style.css", true},
		{"/static/js/app.js", "filepath", "/js/app.js", true},
		{"/static/", "filepath", "/", true},
		{"/files/documents/report.pdf", "path", "/documents/report.pdf", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			var params Params
			_, _, found := router.getValue(http.MethodGet, tt.path, &params)
			if found != tt.wantFound {
				t.Errorf("Found = %v, want %v", found, tt.wantFound)
			}
			if found {
				if len(params) != 1 {
					t.Errorf("Params length = %d, want 1", len(params))
				} else if params[0].Key != tt.wantParam || params[0].Value != tt.wantValue {
					t.Errorf("Param = %v, want {%s: %s}", params[0], tt.wantParam, tt.wantValue)
				}
			}
		})
	}
}

func TestRouter_MethodRouting(t *testing.T) {
	router := newRouter()

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	for _, method := range methods {
		router.addRoute(method, "/resource", HandlersChain{func(c *Context) {}})
	}

	for _, method := range methods {
		var params Params
		_, _, found := router.getValue(method, "/resource", &params)
		if !found {
			t.Errorf("Route not found for method %s", method)
		}
	}

	// Test non-registered method
	var params Params
	_, _, found := router.getValue(http.MethodOptions, "/resource", &params)
	if found {
		t.Error("Should not find route for non-registered method")
	}
}

func TestRouter_MostSpecificRoute(t *testing.T) {
	router := newRouter()

	staticHandler := HandlersChain{func(c *Context) {}}
	paramHandler := HandlersChain{func(c *Context) {}}

	// Register both static and param routes
	router.addRoute(http.MethodGet, "/users/new", staticHandler)
	router.addRoute(http.MethodGet, "/users/:id", paramHandler)

	// Static route should be matched for /users/new
	var params Params
	handlers, fullPath, found := router.getValue(http.MethodGet, "/users/new", &params)
	if !found {
		t.Error("Route not found")
	}
	if fullPath != "/users/new" {
		t.Errorf("FullPath = %s, want /users/new", fullPath)
	}
	if len(params) != 0 {
		t.Errorf("Should not have params for static route, got %v", params)
	}
	if len(handlers) != 1 {
		t.Error("Should have handler")
	}

	// Param route should be matched for /users/123
	params = nil
	handlers, fullPath, found = router.getValue(http.MethodGet, "/users/123", &params)
	if !found {
		t.Error("Route not found")
	}
	if fullPath != "/users/:id" {
		t.Errorf("FullPath = %s, want /users/:id", fullPath)
	}
	if len(params) != 1 || params[0].Value != "123" {
		t.Errorf("Params = %v, want [{id 123}]", params)
	}
}

func TestRouter_NotFound(t *testing.T) {
	router := newRouter()
	router.addRoute(http.MethodGet, "/users", HandlersChain{func(c *Context) {}})

	tests := []string{
		"/",
		"/user",
		"/users/123",
		"/api",
	}

	for _, path := range tests {
		var params Params
		_, _, found := router.getValue(http.MethodGet, path, &params)
		if found {
			t.Errorf("Should not find route for %s", path)
		}
	}
}

func TestParams_Get(t *testing.T) {
	params := Params{
		{Key: "id", Value: "123"},
		{Key: "name", Value: "john"},
	}

	val, ok := params.Get("id")
	if !ok || val != "123" {
		t.Errorf("Get(id) = %s, %v, want 123, true", val, ok)
	}

	val, ok = params.Get("name")
	if !ok || val != "john" {
		t.Errorf("Get(name) = %s, %v, want john, true", val, ok)
	}

	val, ok = params.Get("nonexistent")
	if ok || val != "" {
		t.Errorf("Get(nonexistent) = %s, %v, want '', false", val, ok)
	}
}

func TestParams_ByName(t *testing.T) {
	params := Params{
		{Key: "id", Value: "123"},
	}

	if params.ByName("id") != "123" {
		t.Error("ByName(id) should return 123")
	}
	if params.ByName("nonexistent") != "" {
		t.Error("ByName(nonexistent) should return empty string")
	}
}

// Property-based test: For any registered static route, getValue should find it
// Feature: rue-framework, Property 1: Router Path Matching Correctness
// Validates: Requirements 1.3, 1.4, 1.5, 1.6
func TestRouter_Property_StaticRouteMatching(t *testing.T) {
	f := func(segments []string) bool {
		if len(segments) == 0 || len(segments) > 5 {
			return true // Skip edge cases
		}

		// Build a valid path from segments
		path := "/"
		for _, seg := range segments {
			if seg == "" || len(seg) > 20 {
				return true // Skip invalid segments
			}
			// Only allow alphanumeric segments
			valid := true
			for _, c := range seg {
				if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
					valid = false
					break
				}
			}
			if !valid {
				return true
			}
			path += seg + "/"
		}
		path = path[:len(path)-1] // Remove trailing slash

		router := newRouter()
		handler := HandlersChain{func(c *Context) {}}
		router.addRoute(http.MethodGet, path, handler)

		var params Params
		_, fullPath, found := router.getValue(http.MethodGet, path, &params)

		// Property: registered route should be found with correct path
		return found && fullPath == path
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Property-based test: For any param route, parameter values should be correctly extracted
// Feature: rue-framework, Property 1: Router Path Matching Correctness
// Validates: Requirements 1.3, 1.4, 1.5, 1.6
func TestRouter_Property_ParamExtraction(t *testing.T) {
	f := func(paramValue string) bool {
		if paramValue == "" || len(paramValue) > 50 {
			return true
		}
		// Only allow alphanumeric values
		for _, c := range paramValue {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
				return true
			}
		}

		router := newRouter()
		handler := HandlersChain{func(c *Context) {}}
		router.addRoute(http.MethodGet, "/users/:id", handler)

		var params Params
		_, _, found := router.getValue(http.MethodGet, "/users/"+paramValue, &params)

		// Property: param value should be correctly extracted
		return found && len(params) == 1 && params[0].Key == "id" && params[0].Value == paramValue
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Property-based test: Non-registered routes should not be found
// Feature: rue-framework, Property 1: Router Path Matching Correctness
// Validates: Requirements 1.3, 1.4, 1.5, 1.6
func TestRouter_Property_NonRegisteredNotFound(t *testing.T) {
	f := func(path string) bool {
		if path == "" || path[0] != '/' || len(path) > 100 {
			return true
		}

		router := newRouter()
		// Register a different route
		router.addRoute(http.MethodGet, "/registered/route", HandlersChain{func(c *Context) {}})

		// Skip if path happens to match
		if path == "/registered/route" {
			return true
		}

		var params Params
		_, _, found := router.getValue(http.MethodGet, path, &params)

		// Property: non-registered route should not be found
		return !found
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Benchmark tests
func BenchmarkRouter_StaticRoute(b *testing.B) {
	router := newRouter()
	router.addRoute(http.MethodGet, "/api/v1/users/list", HandlersChain{func(c *Context) {}})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var params Params
		router.getValue(http.MethodGet, "/api/v1/users/list", &params)
	}
}

func BenchmarkRouter_ParamRoute(b *testing.B) {
	router := newRouter()
	router.addRoute(http.MethodGet, "/api/v1/users/:id", HandlersChain{func(c *Context) {}})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var params Params
		router.getValue(http.MethodGet, "/api/v1/users/123", &params)
	}
}

func BenchmarkRouter_ManyRoutes(b *testing.B) {
	router := newRouter()
	routes := []string{
		"/",
		"/users",
		"/users/:id",
		"/users/:id/posts",
		"/users/:id/posts/:postId",
		"/api/v1/users",
		"/api/v1/posts",
		"/api/v2/users",
		"/static/*filepath",
	}
	for _, route := range routes {
		router.addRoute(http.MethodGet, route, HandlersChain{func(c *Context) {}})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var params Params
		router.getValue(http.MethodGet, "/users/123/posts/456", &params)
	}
}
