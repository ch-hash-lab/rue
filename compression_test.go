package rue

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
)

// Property 9: Compression Round-Trip
// For any compressible content, compressing and decompressing should yield the original content.
// Property 10: Compression Content Negotiation
// The middleware should select the appropriate compression based on Accept-Encoding header.
// Validates: Requirements 7.1-7.6

func TestGzip(t *testing.T) {
	engine := New()
	engine.Use(Gzip())
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, strings.Repeat("Hello, World! ", 100))
	})

	// Request with gzip support
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Error("Content-Encoding should be gzip")
	}

	// Decompress and verify
	reader, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("Failed to create gzip reader: %v", err)
	}
	defer reader.Close()

	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	expected := strings.Repeat("Hello, World! ", 100)
	if string(decompressed) != expected {
		t.Errorf("Decompressed content mismatch")
	}
}

func TestGzip_NoAcceptEncoding(t *testing.T) {
	engine := New()
	engine.Use(Gzip())
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, "Hello")
	})

	// Request without gzip support
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Should not compress without Accept-Encoding: gzip")
	}
}

func TestGzip_Skip(t *testing.T) {
	config := GzipConfig{
		SkipFunc: func(c *Context) bool {
			return c.Request.URL.Path == "/skip"
		},
	}

	engine := New()
	engine.Use(GzipWithConfig(config))
	engine.GET("/skip", func(c *Context) {
		c.Text(http.StatusOK, strings.Repeat("Hello", 100))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/skip", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	engine.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "gzip" {
		t.Error("Should skip compression for /skip path")
	}
}

func TestBrotli(t *testing.T) {
	engine := New()
	engine.Use(Brotli())
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, strings.Repeat("Hello, World! ", 100))
	})

	// Request with brotli support
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "br")
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}

	if w.Header().Get("Content-Encoding") != "br" {
		t.Error("Content-Encoding should be br")
	}

	// Decompress and verify
	reader := brotli.NewReader(w.Body)
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("Failed to decompress: %v", err)
	}

	expected := strings.Repeat("Hello, World! ", 100)
	if string(decompressed) != expected {
		t.Errorf("Decompressed content mismatch")
	}
}

func TestBrotli_NoAcceptEncoding(t *testing.T) {
	engine := New()
	engine.Use(Brotli())
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, "Hello")
	})

	// Request without brotli support
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") == "br" {
		t.Error("Should not compress without Accept-Encoding: br")
	}
}

func TestCompress_PrefersBrotli(t *testing.T) {
	engine := New()
	engine.Use(Compress())
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, strings.Repeat("Hello, World! ", 100))
	})

	// Request with both gzip and brotli support
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip, br")
	engine.ServeHTTP(w, req)

	// Should prefer brotli
	if w.Header().Get("Content-Encoding") != "br" {
		t.Errorf("Content-Encoding = %s, want br", w.Header().Get("Content-Encoding"))
	}
}

func TestCompress_FallbackToGzip(t *testing.T) {
	engine := New()
	engine.Use(Compress())
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, strings.Repeat("Hello, World! ", 100))
	})

	// Request with only gzip support
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	engine.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "gzip" {
		t.Errorf("Content-Encoding = %s, want gzip", w.Header().Get("Content-Encoding"))
	}
}

func TestCompress_NoCompression(t *testing.T) {
	engine := New()
	engine.Use(Compress())
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, "Hello")
	})

	// Request without compression support
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	if w.Header().Get("Content-Encoding") != "" {
		t.Error("Should not compress without Accept-Encoding")
	}

	if w.Body.String() != "Hello" {
		t.Errorf("Body = %s, want Hello", w.Body.String())
	}
}

func TestCompress_VaryHeader(t *testing.T) {
	engine := New()
	engine.Use(Compress())
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, strings.Repeat("Hello", 100))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	engine.ServeHTTP(w, req)

	if w.Header().Get("Vary") != "Accept-Encoding" {
		t.Error("Vary header should be set to Accept-Encoding")
	}
}

// Property-based test: Gzip compression round-trip
// Feature: rue-framework, Property 9: Compression Round-Trip
// Validates: Requirements 7.1-7.6
func TestCompression_Property_GzipRoundTrip(t *testing.T) {
	testData := []string{
		strings.Repeat("Hello, World! ", 100),
		strings.Repeat("AAAAAAAAAA", 500),
		strings.Repeat("Test data with special chars: !@#$%^&*() ", 50),
	}

	for _, data := range testData {
		localData := data // capture for closure
		engine := New()
		engine.Use(Gzip())
		engine.GET("/test", func(c *Context) {
			c.Text(http.StatusOK, localData)
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept-Encoding", "gzip")
		engine.ServeHTTP(w, req)

		reader, err := gzip.NewReader(w.Body)
		if err != nil {
			t.Fatalf("Failed to create gzip reader: %v", err)
		}

		decompressed, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatalf("Failed to decompress: %v", err)
		}

		if string(decompressed) != localData {
			t.Errorf("Round-trip failed for data length %d", len(localData))
		}
	}
}

// Property-based test: Brotli compression round-trip
// Feature: rue-framework, Property 9: Compression Round-Trip
// Validates: Requirements 7.1-7.6
func TestCompression_Property_BrotliRoundTrip(t *testing.T) {
	testData := []string{
		strings.Repeat("Hello, World! ", 100),
		strings.Repeat("BBBBBBBBBB", 500),
		strings.Repeat("JSON data: {\"key\": \"value\"} ", 50),
	}

	for _, data := range testData {
		localData := data // capture for closure
		engine := New()
		engine.Use(Brotli())
		engine.GET("/test", func(c *Context) {
			c.Text(http.StatusOK, localData)
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Accept-Encoding", "br")
		engine.ServeHTTP(w, req)

		reader := brotli.NewReader(w.Body)
		decompressed, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("Failed to decompress: %v", err)
		}

		if string(decompressed) != localData {
			t.Errorf("Round-trip failed for data length %d", len(localData))
		}
	}
}

// Property-based test: Content negotiation selects correct encoding
// Feature: rue-framework, Property 10: Compression Content Negotiation
// Validates: Requirements 7.1-7.6
func TestCompression_Property_ContentNegotiation(t *testing.T) {
	testCases := []struct {
		acceptEncoding   string
		expectedEncoding string
	}{
		{"gzip", "gzip"},
		{"br", "br"},
		{"gzip, br", "br"}, // Prefer brotli
		{"br, gzip", "br"}, // Prefer brotli
		{"deflate", ""},    // Not supported
		{"identity", ""},   // No compression
		{"", ""},           // No compression
	}

	for _, tc := range testCases {
		engine := New()
		engine.Use(Compress())
		engine.GET("/test", func(c *Context) {
			c.Text(http.StatusOK, strings.Repeat("Test", 100))
		})

		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		if tc.acceptEncoding != "" {
			req.Header.Set("Accept-Encoding", tc.acceptEncoding)
		}
		engine.ServeHTTP(w, req)

		got := w.Header().Get("Content-Encoding")
		if got != tc.expectedEncoding {
			t.Errorf("Accept-Encoding=%q: Content-Encoding=%q, want %q",
				tc.acceptEncoding, got, tc.expectedEncoding)
		}
	}
}

// Property-based test: Compressed content is smaller than original
// Feature: rue-framework, Property 9: Compression Round-Trip
// Validates: Requirements 7.1-7.6
func TestCompression_Property_SizeReduction(t *testing.T) {
	// Highly compressible data
	data := strings.Repeat("AAAAAAAAAA", 1000)

	// Test gzip
	engine := New()
	engine.Use(Gzip())
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, data)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	engine.ServeHTTP(w, req)

	compressedSize := w.Body.Len()
	originalSize := len(data)

	if compressedSize >= originalSize {
		t.Errorf("Gzip: Compressed size (%d) should be smaller than original (%d)",
			compressedSize, originalSize)
	}

	// Test brotli
	engine2 := New()
	engine2.Use(Brotli())
	engine2.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, data)
	})

	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("Accept-Encoding", "br")
	engine2.ServeHTTP(w2, req2)

	compressedSize2 := w2.Body.Len()
	if compressedSize2 >= originalSize {
		t.Errorf("Brotli: Compressed size (%d) should be smaller than original (%d)",
			compressedSize2, originalSize)
	}
}

// Benchmark gzip compression
func BenchmarkGzip(b *testing.B) {
	data := strings.Repeat("Hello, World! ", 1000)

	engine := New()
	engine.Use(Gzip())
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, data)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
	}
}

// Benchmark brotli compression
func BenchmarkBrotli(b *testing.B) {
	data := strings.Repeat("Hello, World! ", 1000)

	engine := New()
	engine.Use(Brotli())
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, data)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "br")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
	}
}

// Test that compression writers implement necessary interfaces
func TestCompressionWriters_Interfaces(t *testing.T) {
	// Test gzipResponseWriter
	var gzw interface{} = &gzipResponseWriter{}
	if _, ok := gzw.(io.Writer); !ok {
		t.Error("gzipResponseWriter should implement io.Writer")
	}

	// Test brotliResponseWriter
	var brw interface{} = &brotliResponseWriter{}
	if _, ok := brw.(io.Writer); !ok {
		t.Error("brotliResponseWriter should implement io.Writer")
	}
}

// Test WriteString method
func TestGzipResponseWriter_WriteString(t *testing.T) {
	w := httptest.NewRecorder()
	gzw := gzip.NewWriter(w)

	rw := &gzipResponseWriter{
		ResponseWriter: &responseWriter{ResponseWriter: w, status: http.StatusOK},
		gzipWriter:     gzw,
	}

	n, err := rw.WriteString("Hello")
	if err != nil {
		t.Errorf("WriteString error: %v", err)
	}
	if n != 5 {
		t.Errorf("WriteString returned %d, want 5", n)
	}
}
