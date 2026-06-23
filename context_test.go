package rue

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/quick"
)

// Property 2: Context Data Round-Trip
// For any key-value pair stored in Context via Set(), retrieving it via Get()
// should return the exact same value. For any request with query/form/path parameters,
// Context should correctly extract and return them.
// Validates: Requirements 2.2, 2.3

func createTestContext() (*Context, *httptest.ResponseRecorder) {
	engine := New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test?name=john&age=30", nil)
	c := &Context{engine: engine}
	c.reset(rec, req)
	return c, rec
}

func TestContext_SetGet(t *testing.T) {
	c, _ := createTestContext()

	// Test string value
	c.Set("key1", "value1")
	val, exists := c.Get("key1")
	if !exists {
		t.Error("key1 should exist")
	}
	if val != "value1" {
		t.Errorf("Get(key1) = %v, want value1", val)
	}

	// Test int value
	c.Set("key2", 42)
	val, exists = c.Get("key2")
	if !exists {
		t.Error("key2 should exist")
	}
	if val != 42 {
		t.Errorf("Get(key2) = %v, want 42", val)
	}

	// Test non-existent key
	_, exists = c.Get("nonexistent")
	if exists {
		t.Error("nonexistent key should not exist")
	}
}

func TestContext_MustGet(t *testing.T) {
	c, _ := createTestContext()

	c.Set("key", "value")
	val := c.MustGet("key")
	if val != "value" {
		t.Errorf("MustGet(key) = %v, want value", val)
	}

	// Test panic on non-existent key
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustGet should panic on non-existent key")
		}
	}()
	c.MustGet("nonexistent")
}

func TestContext_GetTyped(t *testing.T) {
	c, _ := createTestContext()

	c.Set("str", "hello")
	c.Set("int", 42)
	c.Set("bool", true)

	if c.GetString("str") != "hello" {
		t.Error("GetString failed")
	}
	if c.GetInt("int") != 42 {
		t.Error("GetInt failed")
	}
	if c.GetBool("bool") != true {
		t.Error("GetBool failed")
	}

	// Test default values for wrong types
	if c.GetString("int") != "" {
		t.Error("GetString should return empty for non-string")
	}
	if c.GetInt("str") != 0 {
		t.Error("GetInt should return 0 for non-int")
	}
}

func TestContext_Query(t *testing.T) {
	engine := New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test?name=john&age=30&tags=a&tags=b", nil)
	c := &Context{engine: engine}
	c.reset(rec, req)

	if c.Query("name") != "john" {
		t.Errorf("Query(name) = %s, want john", c.Query("name"))
	}
	if c.Query("age") != "30" {
		t.Errorf("Query(age) = %s, want 30", c.Query("age"))
	}
	if c.Query("nonexistent") != "" {
		t.Error("Query should return empty for non-existent key")
	}
}

func TestContext_QueryDefault(t *testing.T) {
	c, _ := createTestContext()

	if c.QueryDefault("name", "default") != "john" {
		t.Error("QueryDefault should return existing value")
	}
	if c.QueryDefault("nonexistent", "default") != "default" {
		t.Error("QueryDefault should return default for non-existent key")
	}
}

func TestContext_QueryArray(t *testing.T) {
	engine := New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test?tags=a&tags=b&tags=c", nil)
	c := &Context{engine: engine}
	c.reset(rec, req)

	tags := c.QueryArray("tags")
	if len(tags) != 3 {
		t.Errorf("QueryArray(tags) length = %d, want 3", len(tags))
	}
	if tags[0] != "a" || tags[1] != "b" || tags[2] != "c" {
		t.Errorf("QueryArray(tags) = %v, want [a b c]", tags)
	}
}

func TestContext_PostForm(t *testing.T) {
	engine := New()
	rec := httptest.NewRecorder()
	form := url.Values{}
	form.Set("username", "testuser")
	form.Set("password", "secret")
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c := &Context{engine: engine}
	c.reset(rec, req)

	if c.PostForm("username") != "testuser" {
		t.Errorf("PostForm(username) = %s, want testuser", c.PostForm("username"))
	}
	if c.PostForm("password") != "secret" {
		t.Errorf("PostForm(password) = %s, want secret", c.PostForm("password"))
	}
}

func TestContext_Header(t *testing.T) {
	engine := New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Header.Set("Content-Type", "application/json")
	c := &Context{engine: engine}
	c.reset(rec, req)

	if c.Header("X-Custom-Header") != "custom-value" {
		t.Error("Header should return custom header value")
	}
	if c.ContentType() != "application/json" {
		t.Errorf("ContentType() = %s, want application/json", c.ContentType())
	}
}

func TestContext_Param(t *testing.T) {
	c, _ := createTestContext()
	c.Params = Params{
		{Key: "id", Value: "123"},
		{Key: "name", Value: "john"},
	}

	if c.Param("id") != "123" {
		t.Errorf("Param(id) = %s, want 123", c.Param("id"))
	}
	if c.Param("name") != "john" {
		t.Errorf("Param(name) = %s, want john", c.Param("name"))
	}
	if c.Param("nonexistent") != "" {
		t.Error("Param should return empty for non-existent key")
	}
}

func TestContext_ClientIP(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		xri        string
		remoteAddr string
		expected   string
	}{
		{"X-Forwarded-For single", "192.168.1.1", "", "10.0.0.1:1234", "192.168.1.1"},
		{"X-Forwarded-For multiple", "192.168.1.1, 10.0.0.2", "", "10.0.0.1:1234", "192.168.1.1"},
		{"X-Real-IP", "", "192.168.1.2", "10.0.0.1:1234", "192.168.1.2"},
		{"RemoteAddr", "", "", "10.0.0.1:1234", "10.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := New()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			if tt.xri != "" {
				req.Header.Set("X-Real-IP", tt.xri)
			}
			req.RemoteAddr = tt.remoteAddr
			c := &Context{engine: engine}
			c.reset(rec, req)

			if c.ClientIP() != tt.expected {
				t.Errorf("ClientIP() = %s, want %s", c.ClientIP(), tt.expected)
			}
		})
	}
}

func TestContext_FlowControl(t *testing.T) {
	engine := New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	c := &Context{engine: engine}
	c.reset(rec, req)

	// Test initial state
	if c.IsAborted() {
		t.Error("Context should not be aborted initially")
	}

	// Test Abort
	c.Abort()
	if !c.IsAborted() {
		t.Error("Context should be aborted after Abort()")
	}
}

func TestContext_AbortWithStatus(t *testing.T) {
	engine := New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	c := &Context{engine: engine}
	c.reset(rec, req)

	c.AbortWithStatus(http.StatusForbidden)

	if !c.IsAborted() {
		t.Error("Context should be aborted")
	}
	if c.Writer.Status() != http.StatusForbidden {
		t.Errorf("Status = %d, want %d", c.Writer.Status(), http.StatusForbidden)
	}
}

func TestContext_Next(t *testing.T) {
	engine := New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	c := &Context{engine: engine}
	c.reset(rec, req)

	var order []int
	c.handlers = HandlersChain{
		func(c *Context) { order = append(order, 1); c.Next() },
		func(c *Context) { order = append(order, 2); c.Next() },
		func(c *Context) { order = append(order, 3) },
	}
	c.cursor = -1

	c.Next()

	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Errorf("Handler execution order = %v, want [1 2 3]", order)
	}
}

func TestContext_NextAbort(t *testing.T) {
	engine := New()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	c := &Context{engine: engine}
	c.reset(rec, req)

	var order []int
	c.handlers = HandlersChain{
		func(c *Context) { order = append(order, 1); c.Next() },
		func(c *Context) { order = append(order, 2); c.Abort() },
		func(c *Context) { order = append(order, 3) }, // Should not execute
	}
	c.cursor = -1

	c.Next()

	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Errorf("Handler execution order = %v, want [1 2]", order)
	}
}

// Property-based test: For any key-value pair, Set then Get should return the same value
// Feature: rue-framework, Property 2: Context Data Round-Trip
// Validates: Requirements 2.2, 2.3
func TestContext_Property_SetGetRoundTrip(t *testing.T) {
	f := func(key string, value string) bool {
		if key == "" {
			return true // Skip empty keys
		}
		c, _ := createTestContext()
		c.Set(key, value)
		got, exists := c.Get(key)
		return exists && got == value
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Property-based test: For any int value, Set then Get should return the same value
// Feature: rue-framework, Property 2: Context Data Round-Trip
// Validates: Requirements 2.2, 2.3
func TestContext_Property_SetGetIntRoundTrip(t *testing.T) {
	f := func(key string, value int) bool {
		if key == "" {
			return true
		}
		c, _ := createTestContext()
		c.Set(key, value)
		got, exists := c.Get(key)
		return exists && got == value
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Property-based test: Query parameters should be correctly extracted
// Feature: rue-framework, Property 2: Context Data Round-Trip
// Validates: Requirements 2.2, 2.3
func TestContext_Property_QueryExtraction(t *testing.T) {
	f := func(key, value string) bool {
		if key == "" || strings.ContainsAny(key, "=&?#") || strings.ContainsAny(value, "=&?#") {
			return true // Skip invalid query parameter characters
		}

		engine := New()
		rec := httptest.NewRecorder()
		queryStr := url.Values{key: []string{value}}.Encode()
		req := httptest.NewRequest(http.MethodGet, "/test?"+queryStr, nil)
		c := &Context{engine: engine}
		c.reset(rec, req)

		return c.Query(key) == value
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}

// Property-based test: Path parameters should be correctly extracted
// Feature: rue-framework, Property 2: Context Data Round-Trip
// Validates: Requirements 2.2, 2.3
func TestContext_Property_ParamExtraction(t *testing.T) {
	f := func(key, value string) bool {
		if key == "" {
			return true
		}
		c, _ := createTestContext()
		c.Params = Params{{Key: key, Value: value}}
		return c.Param(key) == value
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Error(err)
	}
}
