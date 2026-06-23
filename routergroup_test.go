package rue

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/quick"
)

// Property 5: Route Group Inheritance
// For any nested route group, the final route path should be the concatenation
// of all parent prefixes. For any group-specific middleware, it should only
// apply to routes within that group and its children.
// Validates: Requirements 4.1, 4.2, 4.3, 4.4

func TestRouterGroup_PathPrefix(t *testing.T) {
	engine := New()

	api := engine.Group("/api")
	v1 := api.Group("/v1")
	users := v1.Group("/users")

	var called bool
	users.GET("/:id", func(c *Context) {
		called = true
		c.Text(http.StatusOK, "user")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	engine.ServeHTTP(rec, req)

	if !called {
		t.Error("Handler should be called for nested group path")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestRouterGroup_Middleware(t *testing.T) {
	engine := New()

	var order []string

	// Global middleware
	engine.Use(func(c *Context) {
		order = append(order, "global")
		c.Next()
	})

	// Group middleware
	api := engine.Group("/api")
	api.Use(func(c *Context) {
		order = append(order, "api")
		c.Next()
	})

	// Nested group middleware
	v1 := api.Group("/v1")
	v1.Use(func(c *Context) {
		order = append(order, "v1")
		c.Next()
	})

	v1.GET("/test", func(c *Context) {
		order = append(order, "handler")
		c.Text(http.StatusOK, "ok")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	engine.ServeHTTP(rec, req)

	expected := []string{"global", "api", "v1", "handler"}
	if len(order) != len(expected) {
		t.Errorf("Execution order = %v, want %v", order, expected)
	}
	for i, v := range expected {
		if i < len(order) && order[i] != v {
			t.Errorf("order[%d] = %s, want %s", i, order[i], v)
		}
	}
}

func TestRouterGroup_MiddlewareIsolation(t *testing.T) {
	engine := New()

	var apiCalled, otherCalled bool

	// API group with middleware
	api := engine.Group("/api")
	api.Use(func(c *Context) {
		apiCalled = true
		c.Next()
	})
	api.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, "api")
	})

	// Other group without the api middleware
	other := engine.Group("/other")
	other.Use(func(c *Context) {
		otherCalled = true
		c.Next()
	})
	other.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, "other")
	})

	// Test API route
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	engine.ServeHTTP(rec, req)

	if !apiCalled {
		t.Error("API middleware should be called for /api/test")
	}
	if otherCalled {
		t.Error("Other middleware should NOT be called for /api/test")
	}

	// Reset and test other route
	apiCalled = false
	otherCalled = false

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/other/test", nil)
	engine.ServeHTTP(rec, req)

	if apiCalled {
		t.Error("API middleware should NOT be called for /other/test")
	}
	if !otherCalled {
		t.Error("Other middleware should be called for /other/test")
	}
}

func TestRouterGroup_Use(t *testing.T) {
	engine := New()

	var middlewareCalled bool
	middleware := func(c *Context) {
		middlewareCalled = true
		c.Next()
	}

	engine.Use(middleware)
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, "ok")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(rec, req)

	if !middlewareCalled {
		t.Error("Middleware should be called")
	}
}

func TestRouterGroup_Group(t *testing.T) {
	engine := New()

	api := engine.Group("/api")
	if api.basePath != "/api" {
		t.Errorf("basePath = %s, want /api", api.basePath)
	}

	v1 := api.Group("/v1")
	if v1.basePath != "/api/v1" {
		t.Errorf("basePath = %s, want /api/v1", v1.basePath)
	}

	users := v1.Group("/users")
	if users.basePath != "/api/v1/users" {
		t.Errorf("basePath = %s, want /api/v1/users", users.basePath)
	}
}

func TestRouterGroup_HTTPMethods(t *testing.T) {
	engine := New()

	methods := map[string]func(string, ...HandlerFunc) *RouterGroup{
		http.MethodGet:     engine.GET,
		http.MethodPost:    engine.POST,
		http.MethodPut:     engine.PUT,
		http.MethodDelete:  engine.DELETE,
		http.MethodPatch:   engine.PATCH,
		http.MethodHead:    engine.HEAD,
		http.MethodOptions: engine.OPTIONS,
	}

	for method, register := range methods {
		localMethod := method // capture for closure
		register("/"+method, func(c *Context) {
			c.Text(http.StatusOK, localMethod)
		})
	}

	for method := range methods {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/"+method, nil)
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%s: Status = %d, want %d", method, rec.Code, http.StatusOK)
		}
	}
}

func TestRouterGroup_Any(t *testing.T) {
	engine := New()

	engine.Any("/any", func(c *Context) {
		c.Text(http.StatusOK, c.Request.Method)
	})

	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
		http.MethodHead,
		http.MethodOptions,
	}

	for _, method := range methods {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, "/any", nil)
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%s: Status = %d, want %d", method, rec.Code, http.StatusOK)
		}
	}
}

func TestRouterGroup_Handle(t *testing.T) {
	engine := New()

	engine.Handle(http.MethodGet, "/custom", func(c *Context) {
		c.Text(http.StatusOK, "custom")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/custom", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// Property-based test: Nested group paths should be correctly concatenated
// Feature: rue-framework, Property 5: Route Group Inheritance
// Validates: Requirements 4.1, 4.2, 4.3, 4.4
func TestRouterGroup_Property_PathConcatenation(t *testing.T) {
	f := func(prefix1, prefix2 string) bool {
		// Only test valid path segments
		if prefix1 == "" || prefix2 == "" {
			return true
		}
		if len(prefix1) > 20 || len(prefix2) > 20 {
			return true
		}
		for _, c := range prefix1 + prefix2 {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
				return true
			}
		}

		engine := New()
		g1 := engine.Group("/" + prefix1)
		g2 := g1.Group("/" + prefix2)

		expectedPath := "/" + prefix1 + "/" + prefix2
		return g2.basePath == expectedPath
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Property-based test: Group middleware should be inherited by child groups
// Feature: rue-framework, Property 5: Route Group Inheritance
// Validates: Requirements 4.1, 4.2, 4.3, 4.4
func TestRouterGroup_Property_MiddlewareInheritance(t *testing.T) {
	f := func(numMiddleware uint8) bool {
		// Limit middleware count
		count := int(numMiddleware%5) + 1

		engine := New()
		var callCount int

		// Add middleware to parent group
		parent := engine.Group("/parent")
		for i := 0; i < count; i++ {
			parent.Use(func(c *Context) {
				callCount++
				c.Next()
			})
		}

		// Create child group
		child := parent.Group("/child")
		child.GET("/test", func(c *Context) {
			c.Text(http.StatusOK, "ok")
		})

		// Make request
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/parent/child/test", nil)
		engine.ServeHTTP(rec, req)

		// Property: all parent middleware should be called
		return callCount == count
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

func TestJoinPaths(t *testing.T) {
	tests := []struct {
		absolute string
		relative string
		expected string
	}{
		{"/", "", "/"},
		{"/", "/", "/"},
		{"/api", "/v1", "/api/v1"},
		{"/api", "v1", "/api/v1"},
		{"/api/", "/v1", "/api/v1"},
		{"/api", "/v1/", "/api/v1/"},
	}

	for _, tt := range tests {
		result := resolvePath(tt.absolute, tt.relative)
		if result != tt.expected {
			t.Errorf("resolvePath(%q, %q) = %q, want %q", tt.absolute, tt.relative, result, tt.expected)
		}
	}
}
