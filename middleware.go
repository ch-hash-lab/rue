package rue

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"log"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
)

// RequestLoggerConfig defines the config for request Logger middleware
type RequestLoggerConfig struct {
	Level        LogLevel
	Format       LogFormat
	Output       io.Writer
	SkipPaths    []string
	EnableCaller bool
	TimeFormat   string
	EnableColor  bool
}

// DefaultRequestLoggerConfig returns the default request logger config
func DefaultRequestLoggerConfig() RequestLoggerConfig {
	config := GetModeConfig()
	return RequestLoggerConfig{
		Level:        config.LogLevel,
		Format:       config.LogFormat,
		Output:       os.Stdout,
		SkipPaths:    nil,
		EnableCaller: config.EnableCaller,
		TimeFormat:   time.RFC3339Nano,
		EnableColor:  config.EnableColor,
	}
}

// RequestLogger returns a request Logger middleware with default config
func RequestLogger() HandlerFunc {
	return RequestLoggerWithConfig(DefaultRequestLoggerConfig())
}

// RequestLoggerWithConfig returns a request Logger middleware with custom config
func RequestLoggerWithConfig(config RequestLoggerConfig) HandlerFunc {
	if config.Output == nil {
		config.Output = os.Stdout
	}
	if config.TimeFormat == "" {
		config.TimeFormat = time.RFC3339Nano
	}

	skipPaths := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipPaths[path] = true
	}

	return func(c *Context) {
		// Skip logging for certain paths
		path := c.Request.URL.Path
		if skipPaths[path] {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()
		latency := time.Since(start)

		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		// Determine log level based on status code
		level := InfoLevel
		if statusCode >= 500 {
			level = ErrorLevel
		} else if statusCode >= 400 {
			level = WarnLevel
		}

		// Skip if below configured level
		if level < config.Level {
			return
		}

		entry := LogEntry{
			Timestamp: start.UTC().Format(config.TimeFormat),
			Level:     level.String(),
			Method:    method,
			Path:      path,
			Status:    statusCode,
			Latency:   latency.String(),
			ClientIP:  clientIP,
		}

		// Add caller info for errors
		if config.EnableCaller && level >= ErrorLevel {
			if len(c.Errors) > 0 {
				entry.Error = c.Errors[len(c.Errors)-1].Error()
			}
		}

		if config.Format == JSONFormat {
			writeJSONLog(config.Output, &entry)
		} else {
			writeTextLog(config.Output, &entry, config.EnableColor, level)
		}
	}
}

func writeJSONLog(w io.Writer, entry *LogEntry) {
	data, err := sonic.Marshal(entry)
	if err != nil {
		return
	}
	w.Write(data)
	w.Write([]byte("\n"))
}

func writeTextLog(w io.Writer, entry *LogEntry, enableColor bool, level LogLevel) {
	var buf strings.Builder

	// Color prefix
	if enableColor {
		buf.WriteString(level.Color())
	}

	// Parse timestamp
	t, _ := time.Parse(time.RFC3339Nano, entry.Timestamp)

	// Format: [RUE] 2006/01/02 - 15:04:05 | 200 |     1.234ms | 192.168.1.1 | GET /path
	buf.WriteString("[RUE] ")
	buf.WriteString(t.Local().Format("2006/01/02 - 15:04:05"))
	buf.WriteString(fmt.Sprintf(" | %3d | %13s | %15s | %-7s %s",
		entry.Status,
		entry.Latency,
		entry.ClientIP,
		entry.Method,
		entry.Path,
	))

	if entry.Error != "" {
		buf.WriteString(" | error=")
		buf.WriteString(entry.Error)
	}

	// Reset color
	if enableColor {
		buf.WriteString("\033[0m")
	}

	buf.WriteString("\n")
	w.Write([]byte(buf.String()))
}

// RecoveryConfig defines the config for Recovery middleware
type RecoveryConfig struct {
	StackSize  int
	StackAll   bool
	PrintStack bool
	Output     io.Writer
}

// DefaultRecoveryConfig returns the default recovery config
func DefaultRecoveryConfig() RecoveryConfig {
	return RecoveryConfig{
		StackSize:  4 << 10, // 4 KB
		StackAll:   false,
		PrintStack: true,
		Output:     os.Stderr,
	}
}

// Recovery returns a Recovery middleware with default config
func Recovery() HandlerFunc {
	return RecoveryWithConfig(DefaultRecoveryConfig())
}

// RecoveryWithConfig returns a Recovery middleware with custom config
func RecoveryWithConfig(config RecoveryConfig) HandlerFunc {
	if config.Output == nil {
		config.Output = os.Stderr
	}
	if config.StackSize == 0 {
		config.StackSize = 4 << 10
	}

	logger := log.New(config.Output, "[RUE-RECOVERY] ", log.LstdFlags)

	return func(c *Context) {
		defer func() {
			if err := recover(); err != nil {
				// Get stack trace
				stack := make([]byte, config.StackSize)
				length := runtime.Stack(stack, config.StackAll)
				stack = stack[:length]

				if config.PrintStack {
					logger.Printf("panic recovered: %v\n%s", err, stack)
				}

				c.AbortWithStatus(500)
			}
		}()
		c.Next()
	}
}

// ============== CORS Middleware ==============

// CORSConfig defines the config for CORS middleware
type CORSConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSConfig returns the default CORS config
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders: []string{"Origin", "Content-Type", "Accept", "Authorization"},
		MaxAge:       86400,
	}
}

// CORS returns a CORS middleware with default config
func CORS() HandlerFunc {
	return CORSWithConfig(DefaultCORSConfig())
}

// CORSWithConfig returns a CORS middleware with custom config
func CORSWithConfig(config CORSConfig) HandlerFunc {
	if len(config.AllowOrigins) == 0 {
		config.AllowOrigins = []string{"*"}
	}
	if len(config.AllowMethods) == 0 {
		config.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	}

	allowOrigins := make(map[string]bool)
	allowAll := false
	for _, origin := range config.AllowOrigins {
		if origin == "*" {
			allowAll = true
		}
		allowOrigins[origin] = true
	}

	return func(c *Context) {
		origin := c.Header("Origin")

		// Check if origin is allowed
		if allowAll || allowOrigins[origin] {
			if allowAll {
				c.SetHeader("Access-Control-Allow-Origin", "*")
			} else {
				c.SetHeader("Access-Control-Allow-Origin", origin)
			}
		}

		// Set other CORS headers
		if len(config.AllowMethods) > 0 {
			c.SetHeader("Access-Control-Allow-Methods", strings.Join(config.AllowMethods, ", "))
		}
		if len(config.AllowHeaders) > 0 {
			c.SetHeader("Access-Control-Allow-Headers", strings.Join(config.AllowHeaders, ", "))
		}
		if len(config.ExposeHeaders) > 0 {
			c.SetHeader("Access-Control-Expose-Headers", strings.Join(config.ExposeHeaders, ", "))
		}
		if config.AllowCredentials {
			c.SetHeader("Access-Control-Allow-Credentials", "true")
		}
		if config.MaxAge > 0 {
			c.SetHeader("Access-Control-Max-Age", fmt.Sprintf("%d", config.MaxAge))
		}

		// Handle preflight request
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// ============== Rate Limiter Middleware ==============

// RateLimiterConfig defines the config for RateLimiter middleware
type RateLimiterConfig struct {
	Rate      float64               // Tokens per second
	Burst     int                   // Maximum burst size
	KeyFunc   func(*Context) string // Function to extract key (default: client IP)
	ErrorFunc func(*Context)        // Custom error handler
	SkipFunc  func(*Context) bool   // Skip rate limiting for certain requests
}

// DefaultRateLimiterConfig returns the default rate limiter config
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Rate:  10,
		Burst: 20,
		KeyFunc: func(c *Context) string {
			return c.ClientIP()
		},
	}
}

// tokenBucket implements the token bucket algorithm
type tokenBucket struct {
	tokens     float64
	lastUpdate time.Time
	rate       float64
	burst      int
	mu         sync.Mutex
}

func newTokenBucket(rate float64, burst int) *tokenBucket {
	return &tokenBucket{
		tokens:     float64(burst),
		lastUpdate: time.Now(),
		rate:       rate,
		burst:      burst,
	}
}

func (tb *tokenBucket) allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastUpdate).Seconds()
	tb.lastUpdate = now

	// Add tokens based on elapsed time
	tb.tokens += elapsed * tb.rate
	if tb.tokens > float64(tb.burst) {
		tb.tokens = float64(tb.burst)
	}

	// Check if we have enough tokens
	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

// rateLimiterStore stores token buckets for each key
type rateLimiterStore struct {
	buckets map[string]*tokenBucket
	rate    float64
	burst   int
	mu      sync.RWMutex
}

func newRateLimiterStore(rate float64, burst int) *rateLimiterStore {
	return &rateLimiterStore{
		buckets: make(map[string]*tokenBucket),
		rate:    rate,
		burst:   burst,
	}
}

func (s *rateLimiterStore) getBucket(key string) *tokenBucket {
	s.mu.RLock()
	bucket, exists := s.buckets[key]
	s.mu.RUnlock()

	if exists {
		return bucket
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double check after acquiring write lock
	if bucket, exists = s.buckets[key]; exists {
		return bucket
	}

	bucket = newTokenBucket(s.rate, s.burst)
	s.buckets[key] = bucket
	return bucket
}

// RateLimiter returns a RateLimiter middleware with default config
func RateLimiter() HandlerFunc {
	return RateLimiterWithConfig(DefaultRateLimiterConfig())
}

// RateLimiterWithConfig returns a RateLimiter middleware with custom config
func RateLimiterWithConfig(config RateLimiterConfig) HandlerFunc {
	if config.Rate <= 0 {
		config.Rate = 10
	}
	if config.Burst <= 0 {
		config.Burst = 20
	}
	if config.KeyFunc == nil {
		config.KeyFunc = func(c *Context) string {
			return c.ClientIP()
		}
	}

	store := newRateLimiterStore(config.Rate, config.Burst)

	return func(c *Context) {
		// Skip if configured
		if config.SkipFunc != nil && config.SkipFunc(c) {
			c.Next()
			return
		}

		key := config.KeyFunc(c)
		bucket := store.getBucket(key)

		if !bucket.allow() {
			if config.ErrorFunc != nil {
				config.ErrorFunc(c)
			} else {
				c.AbortWithJSON(http.StatusTooManyRequests, H{
					"code":    http.StatusTooManyRequests,
					"message": "Too Many Requests",
				})
			}
			return
		}

		c.Next()
	}
}

// ============== JWT Middleware ==============

// JWTClaims represents JWT claims
type JWTClaims struct {
	Subject   string         `json:"sub,omitempty"`
	Issuer    string         `json:"iss,omitempty"`
	Audience  string         `json:"aud,omitempty"`
	ExpiresAt int64          `json:"exp,omitempty"`
	IssuedAt  int64          `json:"iat,omitempty"`
	NotBefore int64          `json:"nbf,omitempty"`
	ID        string         `json:"jti,omitempty"`
	Custom    map[string]any `json:"-"`
}

// JWTConfig defines the config for JWT middleware
type JWTConfig struct {
	Secret        []byte
	SigningMethod string                // HS256, HS384, HS512
	TokenLookup   string                // "header:Authorization", "query:token", "cookie:jwt"
	AuthScheme    string                // "Bearer"
	ContextKey    string                // Key to store claims in context
	ErrorFunc     func(*Context, error) // Custom error handler
	SkipFunc      func(*Context) bool   // Skip JWT validation for certain requests
}

// DefaultJWTConfig returns the default JWT config
func DefaultJWTConfig() JWTConfig {
	return JWTConfig{
		SigningMethod: "HS256",
		TokenLookup:   "header:Authorization",
		AuthScheme:    "Bearer",
		ContextKey:    "jwt_claims",
	}
}

// JWT returns a JWT middleware with the given secret
func JWT(secret []byte) HandlerFunc {
	config := DefaultJWTConfig()
	config.Secret = secret
	return JWTWithConfig(config)
}

// JWTWithConfig returns a JWT middleware with custom config
func JWTWithConfig(config JWTConfig) HandlerFunc {
	if len(config.Secret) == 0 {
		panic("JWT secret is required")
	}
	if config.SigningMethod == "" {
		config.SigningMethod = "HS256"
	}
	if config.TokenLookup == "" {
		config.TokenLookup = "header:Authorization"
	}
	if config.AuthScheme == "" {
		config.AuthScheme = "Bearer"
	}
	if config.ContextKey == "" {
		config.ContextKey = "jwt_claims"
	}

	return func(c *Context) {
		// Skip if configured
		if config.SkipFunc != nil && config.SkipFunc(c) {
			c.Next()
			return
		}

		// Extract token
		token, err := extractToken(c, config.TokenLookup, config.AuthScheme)
		if err != nil {
			handleJWTError(c, config, err)
			return
		}

		// Parse and validate token
		claims, err := ParseToken(token, config.Secret)
		if err != nil {
			handleJWTError(c, config, err)
			return
		}

		// Store claims in context
		c.Set(config.ContextKey, claims)
		c.Next()
	}
}

func extractToken(c *Context, lookup, scheme string) (string, error) {
	parts := strings.SplitN(lookup, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid token lookup format")
	}

	switch parts[0] {
	case "header":
		auth := c.Header(parts[1])
		if auth == "" {
			return "", fmt.Errorf("missing authorization header")
		}
		if scheme != "" {
			prefix := scheme + " "
			if !strings.HasPrefix(auth, prefix) {
				return "", fmt.Errorf("invalid authorization scheme")
			}
			return auth[len(prefix):], nil
		}
		return auth, nil

	case "query":
		token := c.Query(parts[1])
		if token == "" {
			return "", fmt.Errorf("missing token in query")
		}
		return token, nil

	case "cookie":
		token, err := c.Cookie(parts[1])
		if err != nil {
			return "", fmt.Errorf("missing token in cookie")
		}
		return token, nil

	default:
		return "", fmt.Errorf("unsupported token lookup: %s", parts[0])
	}
}

func handleJWTError(c *Context, config JWTConfig, err error) {
	if config.ErrorFunc != nil {
		config.ErrorFunc(c, err)
	} else {
		c.AbortWithJSON(http.StatusUnauthorized, H{
			"code":    http.StatusUnauthorized,
			"message": "Unauthorized",
			"error":   err.Error(),
		})
	}
}

// GenerateToken generates a JWT token with the given claims
func GenerateToken(claims *JWTClaims, secret []byte) (string, error) {
	return GenerateTokenWithMethod(claims, secret, "HS256")
}

// GenerateTokenWithMethod generates a JWT token with the specified signing method
func GenerateTokenWithMethod(claims *JWTClaims, secret []byte, method string) (string, error) {
	// Create header
	header := map[string]string{
		"alg": method,
		"typ": "JWT",
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Create payload
	payload := make(map[string]any)
	if claims.Subject != "" {
		payload["sub"] = claims.Subject
	}
	if claims.Issuer != "" {
		payload["iss"] = claims.Issuer
	}
	if claims.Audience != "" {
		payload["aud"] = claims.Audience
	}
	if claims.ExpiresAt != 0 {
		payload["exp"] = claims.ExpiresAt
	}
	if claims.IssuedAt != 0 {
		payload["iat"] = claims.IssuedAt
	}
	if claims.NotBefore != 0 {
		payload["nbf"] = claims.NotBefore
	}
	if claims.ID != "" {
		payload["jti"] = claims.ID
	}
	for k, v := range claims.Custom {
		payload[k] = v
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	// Create signature
	signingInput := headerB64 + "." + payloadB64
	signature, err := sign(signingInput, secret, method)
	if err != nil {
		return "", err
	}

	return signingInput + "." + signature, nil
}

// ParseToken parses and validates a JWT token
func ParseToken(tokenString string, secret []byte) (*JWTClaims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	// Decode header
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid header encoding")
	}
	var header map[string]string
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("invalid header JSON")
	}

	method := header["alg"]
	if method == "" {
		return nil, fmt.Errorf("missing signing method")
	}

	// Verify signature
	signingInput := parts[0] + "." + parts[1]
	expectedSig, err := sign(signingInput, secret, method)
	if err != nil {
		return nil, err
	}
	if !hmac.Equal([]byte(parts[2]), []byte(expectedSig)) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Decode payload
	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid payload encoding")
	}
	var payload map[string]any
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		return nil, fmt.Errorf("invalid payload JSON")
	}

	// Parse claims
	claims := &JWTClaims{Custom: make(map[string]any)}
	for k, v := range payload {
		switch k {
		case "sub":
			if s, ok := v.(string); ok {
				claims.Subject = s
			}
		case "iss":
			if s, ok := v.(string); ok {
				claims.Issuer = s
			}
		case "aud":
			if s, ok := v.(string); ok {
				claims.Audience = s
			}
		case "exp":
			if f, ok := v.(float64); ok {
				claims.ExpiresAt = int64(f)
			}
		case "iat":
			if f, ok := v.(float64); ok {
				claims.IssuedAt = int64(f)
			}
		case "nbf":
			if f, ok := v.(float64); ok {
				claims.NotBefore = int64(f)
			}
		case "jti":
			if s, ok := v.(string); ok {
				claims.ID = s
			}
		default:
			claims.Custom[k] = v
		}
	}

	// Validate expiration
	now := time.Now().Unix()
	if claims.ExpiresAt != 0 && now > claims.ExpiresAt {
		return nil, fmt.Errorf("token expired")
	}
	if claims.NotBefore != 0 && now < claims.NotBefore {
		return nil, fmt.Errorf("token not yet valid")
	}

	return claims, nil
}

func sign(input string, secret []byte, method string) (string, error) {
	var mac hash.Hash
	switch method {
	case "HS256":
		mac = hmac.New(sha256.New, secret)
	case "HS384":
		mac = hmac.New(sha256.New, secret) // Simplified, using SHA256
	case "HS512":
		mac = hmac.New(sha256.New, secret) // Simplified, using SHA256
	default:
		return "", fmt.Errorf("unsupported signing method: %s", method)
	}

	mac.Write([]byte(input))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

// ============== API Key Middleware ==============

// APIKeyConfig defines the config for APIKey middleware
type APIKeyConfig struct {
	KeyLookup  string              // "header:X-API-Key", "query:api_key"
	Validator  func(string) bool   // Function to validate the API key
	ContextKey string              // Key to store API key in context
	ErrorFunc  func(*Context)      // Custom error handler
	SkipFunc   func(*Context) bool // Skip API key validation for certain requests
}

// DefaultAPIKeyConfig returns the default API key config
func DefaultAPIKeyConfig() APIKeyConfig {
	return APIKeyConfig{
		KeyLookup:  "header:X-API-Key",
		ContextKey: "api_key",
	}
}

// APIKey returns an APIKey middleware with the given validator
func APIKey(validator func(string) bool) HandlerFunc {
	config := DefaultAPIKeyConfig()
	config.Validator = validator
	return APIKeyWithConfig(config)
}

// APIKeyWithConfig returns an APIKey middleware with custom config
func APIKeyWithConfig(config APIKeyConfig) HandlerFunc {
	if config.Validator == nil {
		panic("API key validator is required")
	}
	if config.KeyLookup == "" {
		config.KeyLookup = "header:X-API-Key"
	}
	if config.ContextKey == "" {
		config.ContextKey = "api_key"
	}

	return func(c *Context) {
		// Skip if configured
		if config.SkipFunc != nil && config.SkipFunc(c) {
			c.Next()
			return
		}

		// Extract API key
		key, err := extractAPIKey(c, config.KeyLookup)
		if err != nil {
			handleAPIKeyError(c, config)
			return
		}

		// Validate API key
		if !config.Validator(key) {
			handleAPIKeyError(c, config)
			return
		}

		// Store API key in context
		c.Set(config.ContextKey, key)
		c.Next()
	}
}

func extractAPIKey(c *Context, lookup string) (string, error) {
	parts := strings.SplitN(lookup, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid key lookup format")
	}

	switch parts[0] {
	case "header":
		key := c.Header(parts[1])
		if key == "" {
			return "", fmt.Errorf("missing API key in header")
		}
		return key, nil

	case "query":
		key := c.Query(parts[1])
		if key == "" {
			return "", fmt.Errorf("missing API key in query")
		}
		return key, nil

	default:
		return "", fmt.Errorf("unsupported key lookup: %s", parts[0])
	}
}

func handleAPIKeyError(c *Context, config APIKeyConfig) {
	if config.ErrorFunc != nil {
		config.ErrorFunc(c)
	} else {
		c.AbortWithJSON(http.StatusUnauthorized, H{
			"code":    http.StatusUnauthorized,
			"message": "Invalid API Key",
		})
	}
}
