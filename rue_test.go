package rue

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEngine_New(t *testing.T) {
	engine := New()
	if engine == nil {
		t.Fatal("New() returned nil")
	}
	if engine.router == nil {
		t.Error("router should not be nil")
	}
	if engine.Binder == nil {
		t.Error("Binder should not be nil")
	}
	if engine.Validator == nil {
		t.Error("Validator should not be nil")
	}
}

func TestEngine_Default(t *testing.T) {
	engine := Default()
	if engine == nil {
		t.Fatal("Default() returned nil")
	}
	// Default should have Logger and Recovery middleware
	if len(engine.middleware) != 2 {
		t.Errorf("Default middleware count = %d, want 2", len(engine.middleware))
	}
}

func TestEngine_ServeHTTP(t *testing.T) {
	engine := New()

	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, "hello")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "hello" {
		t.Errorf("Body = %q, want %q", rec.Body.String(), "hello")
	}
}

func TestEngine_NotFound(t *testing.T) {
	engine := New()

	engine.GET("/exists", func(c *Context) {
		c.Text(http.StatusOK, "ok")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestEngine_MiddlewareChain(t *testing.T) {
	engine := New()

	var order []int

	engine.Use(func(c *Context) {
		order = append(order, 1)
		c.Next()
		order = append(order, 5)
	})

	engine.Use(func(c *Context) {
		order = append(order, 2)
		c.Next()
		order = append(order, 4)
	})

	engine.GET("/test", func(c *Context) {
		order = append(order, 3)
		c.Text(http.StatusOK, "ok")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(rec, req)

	expected := []int{1, 2, 3, 4, 5}
	if len(order) != len(expected) {
		t.Errorf("Execution order = %v, want %v", order, expected)
	}
	for i, v := range expected {
		if i < len(order) && order[i] != v {
			t.Errorf("order[%d] = %d, want %d", i, order[i], v)
		}
	}
}

func TestEngine_JSONResponse(t *testing.T) {
	engine := New()

	engine.GET("/json", func(c *Context) {
		c.JSON(http.StatusOK, H{"message": "hello"})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/json", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}
}

func TestEngine_PathParams(t *testing.T) {
	engine := New()

	engine.GET("/users/:id", func(c *Context) {
		id := c.Param("id")
		c.Text(http.StatusOK, "user:"+id)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "user:123" {
		t.Errorf("Body = %q, want %q", rec.Body.String(), "user:123")
	}
}

func TestEngine_QueryParams(t *testing.T) {
	engine := New()

	engine.GET("/search", func(c *Context) {
		q := c.Query("q")
		c.Text(http.StatusOK, "search:"+q)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/search?q=hello", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "search:hello" {
		t.Errorf("Body = %q, want %q", rec.Body.String(), "search:hello")
	}
}

func TestEngine_PostJSON(t *testing.T) {
	engine := New()

	type User struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	engine.POST("/users", func(c *Context) {
		var user User
		if err := c.BindJSON(&user); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		c.JSON(http.StatusCreated, user)
	})

	body := bytes.NewBufferString(`{"name":"john","age":30}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/users", body)
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusCreated)
	}
}

func TestEngine_Abort(t *testing.T) {
	engine := New()

	var handlerCalled bool

	engine.Use(func(c *Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	})

	engine.GET("/protected", func(c *Context) {
		handlerCalled = true
		c.Text(http.StatusOK, "ok")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if handlerCalled {
		t.Error("Handler should not be called after Abort")
	}
}

func TestEngine_ContextPool(t *testing.T) {
	engine := New()

	var contexts []*Context

	engine.GET("/test", func(c *Context) {
		contexts = append(contexts, c)
		c.Text(http.StatusOK, "ok")
	})

	// Make multiple requests
	for i := 0; i < 10; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		engine.ServeHTTP(rec, req)
	}

	// Contexts should be reused from pool
	// We can't directly test pool reuse, but we can verify requests work
	if len(contexts) != 10 {
		t.Errorf("Expected 10 contexts, got %d", len(contexts))
	}
}

func TestEngine_LifecycleHooks(t *testing.T) {
	engine := New()

	var onRequestCalled, onResponseCalled bool

	engine.OnRequest(func(c *Context) {
		onRequestCalled = true
	})

	engine.OnResponse(func(c *Context) {
		onResponseCalled = true
	})

	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, "ok")
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	engine.ServeHTTP(rec, req)

	if !onRequestCalled {
		t.Error("OnRequest hook should be called")
	}
	if !onResponseCalled {
		t.Error("OnResponse hook should be called")
	}
}

func TestEngine_Groups(t *testing.T) {
	engine := New()

	api := engine.Group("/api")
	{
		api.GET("/users", func(c *Context) {
			c.Text(http.StatusOK, "users")
		})

		v1 := api.Group("/v1")
		{
			v1.GET("/posts", func(c *Context) {
				c.Text(http.StatusOK, "v1 posts")
			})
		}
	}

	tests := []struct {
		path     string
		expected string
	}{
		{"/api/users", "users"},
		{"/api/v1/posts", "v1 posts"},
	}

	for _, tt := range tests {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tt.path, nil)
		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%s: Status = %d, want %d", tt.path, rec.Code, http.StatusOK)
		}
		if rec.Body.String() != tt.expected {
			t.Errorf("%s: Body = %q, want %q", tt.path, rec.Body.String(), tt.expected)
		}
	}
}

func TestHandlersChain_Last(t *testing.T) {
	handler1 := func(c *Context) {}
	handler2 := func(c *Context) {}

	chain := HandlersChain{handler1, handler2}
	if chain.Last() == nil {
		t.Error("Last() should not return nil")
	}

	emptyChain := HandlersChain{}
	if emptyChain.Last() != nil {
		t.Error("Last() should return nil for empty chain")
	}
}

// Benchmark tests
func BenchmarkEngine_SimpleRoute(b *testing.B) {
	engine := New()
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
	}
}

func BenchmarkEngine_ParamRoute(b *testing.B) {
	engine := New()
	engine.GET("/users/:id", func(c *Context) {
		c.Param("id")
		c.Text(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
	}
}

func BenchmarkEngine_JSONResponse(b *testing.B) {
	engine := New()
	engine.GET("/json", func(c *Context) {
		c.JSON(http.StatusOK, H{"message": "hello", "count": 42})
	})

	req := httptest.NewRequest(http.MethodGet, "/json", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
	}
}
