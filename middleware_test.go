package rue

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Property 4: Middleware Chain Execution Order
// Middleware should execute in the order they are registered, and Next() should
// properly pass control to the next handler.
// Validates: Requirements 3.1, 3.3, 3.4, 3.10

func TestLogger(t *testing.T) {
	var buf bytes.Buffer
	config := RequestLoggerConfig{
		Output:      &buf,
		Format:      TextFormat,
		EnableColor: false,
	}

	engine := New()
	engine.Use(RequestLoggerWithConfig(config))
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	output := buf.String()
	if !strings.Contains(output, "[RUE]") {
		t.Errorf("Logger output should contain prefix, got: %s", output)
	}
	if !strings.Contains(output, "200") {
		t.Errorf("Logger output should contain status code, got: %s", output)
	}
	if !strings.Contains(output, "GET") {
		t.Errorf("Logger output should contain method, got: %s", output)
	}
	if !strings.Contains(output, "/test") {
		t.Errorf("Logger output should contain path, got: %s", output)
	}
}

func TestLogger_SkipPaths(t *testing.T) {
	var buf bytes.Buffer
	config := RequestLoggerConfig{
		Output:    &buf,
		SkipPaths: []string{"/health"},
	}

	engine := New()
	engine.Use(RequestLoggerWithConfig(config))
	engine.GET("/health", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})
	engine.GET("/api", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})

	// Health endpoint should be skipped
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	engine.ServeHTTP(w, req)

	if strings.Contains(buf.String(), "/health") {
		t.Error("Logger should skip /health path")
	}

	// API endpoint should be logged
	buf.Reset()
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api", nil)
	engine.ServeHTTP(w, req)

	if !strings.Contains(buf.String(), "/api") {
		t.Error("Logger should log /api path")
	}
}

func TestRecovery(t *testing.T) {
	var buf bytes.Buffer
	config := RecoveryConfig{
		Output:     &buf,
		PrintStack: true,
	}

	engine := New()
	engine.Use(RecoveryWithConfig(config))
	engine.GET("/panic", func(c *Context) {
		panic("test panic")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/panic", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	if !strings.Contains(buf.String(), "test panic") {
		t.Errorf("Recovery should log panic message, got: %s", buf.String())
	}
}

func TestCORS(t *testing.T) {
	engine := New()
	engine.Use(CORS())
	engine.GET("/api", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})

	// Regular request
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Origin", "http://example.com")
	engine.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Error("CORS should set Access-Control-Allow-Origin header")
	}

	// Preflight request
	w = httptest.NewRecorder()
	req = httptest.NewRequest("OPTIONS", "/api", nil)
	req.Header.Set("Origin", "http://example.com")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Preflight status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

func TestCORS_CustomConfig(t *testing.T) {
	config := CORSConfig{
		AllowOrigins:     []string{"http://example.com"},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Content-Type"},
		AllowCredentials: true,
		MaxAge:           3600,
	}

	engine := New()
	engine.Use(CORSWithConfig(config))
	engine.GET("/api", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})

	// Allowed origin
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("Origin", "http://example.com")
	engine.ServeHTTP(w, req)

	if w.Header().Get("Access-Control-Allow-Origin") != "http://example.com" {
		t.Error("CORS should set specific origin")
	}
	if w.Header().Get("Access-Control-Allow-Credentials") != "true" {
		t.Error("CORS should set credentials header")
	}
}

func TestRateLimiter(t *testing.T) {
	config := RateLimiterConfig{
		Rate:  1,
		Burst: 2,
		KeyFunc: func(c *Context) string {
			return "test"
		},
	}

	engine := New()
	engine.Use(RateLimiterWithConfig(config))
	engine.GET("/api", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})

	// First two requests should succeed (burst = 2)
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api", nil)
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Request %d: Status = %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}

	// Third request should be rate limited
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Rate limited request: Status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}
}

func TestRateLimiter_Skip(t *testing.T) {
	config := RateLimiterConfig{
		Rate:  1,
		Burst: 1,
		SkipFunc: func(c *Context) bool {
			return c.Request.URL.Path == "/health"
		},
	}

	engine := New()
	engine.Use(RateLimiterWithConfig(config))
	engine.GET("/health", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})

	// Health endpoint should not be rate limited
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/health", nil)
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Health request %d should not be rate limited", i+1)
		}
	}
}

func TestJWT(t *testing.T) {
	secret := []byte("test-secret")

	engine := New()
	engine.Use(JWT(secret))
	engine.GET("/protected", func(c *Context) {
		claims, _ := c.Get("jwt_claims")
		c.JSON(http.StatusOK, H{"claims": claims})
	})

	// Generate valid token
	claims := &JWTClaims{
		Subject:   "user123",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	token, err := GenerateToken(claims, secret)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Request with valid token
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Valid token: Status = %d, want %d", w.Code, http.StatusOK)
	}

	// Request without token
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/protected", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("No token: Status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	// Request with invalid token
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Invalid token: Status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestJWT_ExpiredToken(t *testing.T) {
	secret := []byte("test-secret")

	engine := New()
	engine.Use(JWT(secret))
	engine.GET("/protected", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})

	// Generate expired token
	claims := &JWTClaims{
		Subject:   "user123",
		ExpiresAt: time.Now().Add(-time.Hour).Unix(),
	}
	token, _ := GenerateToken(claims, secret)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expired token: Status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestJWT_TokenLookup(t *testing.T) {
	secret := []byte("test-secret")

	// Test query parameter lookup
	config := JWTConfig{
		Secret:      secret,
		TokenLookup: "query:token",
	}

	engine := New()
	engine.Use(JWTWithConfig(config))
	engine.GET("/protected", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})

	claims := &JWTClaims{Subject: "user123", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	token, _ := GenerateToken(claims, secret)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/protected?token="+token, nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Query token: Status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestGenerateToken_ParseToken(t *testing.T) {
	secret := []byte("test-secret")

	claims := &JWTClaims{
		Subject:   "user123",
		Issuer:    "test-issuer",
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		IssuedAt:  time.Now().Unix(),
		Custom: map[string]any{
			"role": "admin",
		},
	}

	token, err := GenerateToken(claims, secret)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	parsed, err := ParseToken(token, secret)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}

	if parsed.Subject != claims.Subject {
		t.Errorf("Subject = %s, want %s", parsed.Subject, claims.Subject)
	}
	if parsed.Issuer != claims.Issuer {
		t.Errorf("Issuer = %s, want %s", parsed.Issuer, claims.Issuer)
	}
	if parsed.Custom["role"] != "admin" {
		t.Errorf("Custom role = %v, want admin", parsed.Custom["role"])
	}
}

func TestAPIKey(t *testing.T) {
	validKeys := map[string]bool{
		"valid-key-1": true,
		"valid-key-2": true,
	}

	engine := New()
	engine.Use(APIKey(func(key string) bool {
		return validKeys[key]
	}))
	engine.GET("/api", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})

	// Valid API key
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("X-API-Key", "valid-key-1")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Valid key: Status = %d, want %d", w.Code, http.StatusOK)
	}

	// Invalid API key
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api", nil)
	req.Header.Set("X-API-Key", "invalid-key")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Invalid key: Status = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	// Missing API key
	w = httptest.NewRecorder()
	req = httptest.NewRequest("GET", "/api", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Missing key: Status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestAPIKey_QueryLookup(t *testing.T) {
	config := APIKeyConfig{
		KeyLookup: "query:api_key",
		Validator: func(key string) bool {
			return key == "valid-key"
		},
	}

	engine := New()
	engine.Use(APIKeyWithConfig(config))
	engine.GET("/api", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api?api_key=valid-key", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Query key: Status = %d, want %d", w.Code, http.StatusOK)
	}
}

// Property-based test: Middleware chain should execute in order
// Feature: rue-framework, Property 4: Middleware Chain Execution Order
// Validates: Requirements 3.1, 3.3, 3.4, 3.10
func TestMiddleware_Property_ExecutionOrder(t *testing.T) {
	var order []int

	engine := New()
	engine.Use(func(c *Context) {
		order = append(order, 1)
		c.Next()
		order = append(order, 4)
	})
	engine.Use(func(c *Context) {
		order = append(order, 2)
		c.Next()
		order = append(order, 3)
	})
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	expected := []int{1, 2, 3, 4}
	if len(order) != len(expected) {
		t.Errorf("Order length = %d, want %d", len(order), len(expected))
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("Order[%d] = %d, want %d", i, order[i], v)
		}
	}
}

// Property-based test: Abort should stop middleware chain
// Feature: rue-framework, Property 4: Middleware Chain Execution Order
// Validates: Requirements 3.1, 3.3, 3.4, 3.10
func TestMiddleware_Property_AbortStopsChain(t *testing.T) {
	handlerCalled := false

	engine := New()
	engine.Use(func(c *Context) {
		c.AbortWithStatus(http.StatusForbidden)
	})
	engine.GET("/test", func(c *Context) {
		handlerCalled = true
		c.Text(http.StatusOK, "OK")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	if handlerCalled {
		t.Error("Handler should not be called after Abort")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

// Property-based test: Recovery should catch all panics
// Feature: rue-framework, Property 4: Middleware Chain Execution Order
// Validates: Requirements 3.1, 3.3, 3.4, 3.10
func TestMiddleware_Property_RecoveryCatchesPanic(t *testing.T) {
	var buf bytes.Buffer
	config := RecoveryConfig{
		Output:     &buf,
		PrintStack: false,
	}

	engine := New()
	engine.Use(RecoveryWithConfig(config))

	panicMessages := []string{"panic1", "panic2", "panic3"}

	for _, msg := range panicMessages {
		localMsg := msg // Capture for closure
		engine.GET("/panic/"+msg, func(c *Context) {
			panic(localMsg)
		})
	}

	for _, msg := range panicMessages {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/panic/"+msg, nil)
		engine.ServeHTTP(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("Panic '%s': Status = %d, want %d", msg, w.Code, http.StatusInternalServerError)
		}
	}
}
