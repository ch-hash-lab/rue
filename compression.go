package rue

import (
	"compress/gzip"
	"io"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
)

// ============== Gzip Middleware ==============

// GzipConfig defines the config for Gzip middleware
type GzipConfig struct {
	Level        int                 // Compression level (1-9, default: 6)
	MinLength    int                 // Minimum content length to compress (default: 1024)
	ContentTypes []string            // Content types to compress (default: text/*, application/json, application/xml)
	SkipFunc     func(*Context) bool // Skip compression for certain requests
}

// DefaultGzipConfig returns the default Gzip config
func DefaultGzipConfig() GzipConfig {
	return GzipConfig{
		Level:     gzip.DefaultCompression,
		MinLength: 1024,
		ContentTypes: []string{
			"text/html",
			"text/css",
			"text/plain",
			"text/javascript",
			"application/json",
			"application/xml",
			"application/javascript",
		},
	}
}

var gzipWriterPool = sync.Pool{
	New: func() any {
		w, _ := gzip.NewWriterLevel(io.Discard, gzip.DefaultCompression)
		return w
	},
}

// gzipResponseWriter wraps ResponseWriter with gzip compression
type gzipResponseWriter struct {
	ResponseWriter
	gzipWriter *gzip.Writer
	minLength  int
	written    bool
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.written = true
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")
	}
	return w.gzipWriter.Write(data)
}

func (w *gzipResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *gzipResponseWriter) Close() error {
	return w.gzipWriter.Close()
}

// Gzip returns a Gzip middleware with default config
func Gzip() HandlerFunc {
	return GzipWithConfig(DefaultGzipConfig())
}

// GzipWithConfig returns a Gzip middleware with custom config
func GzipWithConfig(config GzipConfig) HandlerFunc {
	if config.Level == 0 {
		config.Level = gzip.DefaultCompression
	}
	if config.MinLength == 0 {
		config.MinLength = 1024
	}

	contentTypes := make(map[string]bool)
	for _, ct := range config.ContentTypes {
		contentTypes[ct] = true
	}

	return func(c *Context) {
		// Skip if configured
		if config.SkipFunc != nil && config.SkipFunc(c) {
			c.Next()
			return
		}

		// Check Accept-Encoding
		if !strings.Contains(c.Header("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}

		// Get gzip writer from pool
		gzipWriter := gzipWriterPool.Get().(*gzip.Writer)
		gzipWriter.Reset(c.Writer)

		gzw := &gzipResponseWriter{
			ResponseWriter: c.Writer,
			gzipWriter:     gzipWriter,
			minLength:      config.MinLength,
		}

		c.Writer = gzw
		defer func() {
			gzw.Close()
			gzipWriterPool.Put(gzipWriter)
		}()

		c.SetHeader("Vary", "Accept-Encoding")
		c.Next()
	}
}

// ============== Brotli Middleware ==============

// BrotliConfig defines the config for Brotli middleware
type BrotliConfig struct {
	Level        int                 // Compression level (0-11, default: 6)
	MinLength    int                 // Minimum content length to compress (default: 1024)
	ContentTypes []string            // Content types to compress
	SkipFunc     func(*Context) bool // Skip compression for certain requests
}

// DefaultBrotliConfig returns the default Brotli config
func DefaultBrotliConfig() BrotliConfig {
	return BrotliConfig{
		Level:     brotli.DefaultCompression,
		MinLength: 1024,
		ContentTypes: []string{
			"text/html",
			"text/css",
			"text/plain",
			"text/javascript",
			"application/json",
			"application/xml",
			"application/javascript",
		},
	}
}

var brotliWriterPool = sync.Pool{
	New: func() any {
		return brotli.NewWriterLevel(io.Discard, brotli.DefaultCompression)
	},
}

// brotliResponseWriter wraps ResponseWriter with brotli compression
type brotliResponseWriter struct {
	ResponseWriter
	brotliWriter *brotli.Writer
	minLength    int
	written      bool
}

func (w *brotliResponseWriter) Write(data []byte) (int, error) {
	if !w.written {
		w.written = true
		w.Header().Set("Content-Encoding", "br")
		w.Header().Del("Content-Length")
	}
	return w.brotliWriter.Write(data)
}

func (w *brotliResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

func (w *brotliResponseWriter) Close() error {
	return w.brotliWriter.Close()
}

// Brotli returns a Brotli middleware with default config
func Brotli() HandlerFunc {
	return BrotliWithConfig(DefaultBrotliConfig())
}

// BrotliWithConfig returns a Brotli middleware with custom config
func BrotliWithConfig(config BrotliConfig) HandlerFunc {
	if config.Level == 0 {
		config.Level = brotli.DefaultCompression
	}
	if config.MinLength == 0 {
		config.MinLength = 1024
	}

	return func(c *Context) {
		// Skip if configured
		if config.SkipFunc != nil && config.SkipFunc(c) {
			c.Next()
			return
		}

		// Check Accept-Encoding
		if !strings.Contains(c.Header("Accept-Encoding"), "br") {
			c.Next()
			return
		}

		// Get brotli writer from pool
		brotliWriter := brotliWriterPool.Get().(*brotli.Writer)
		brotliWriter.Reset(c.Writer)

		brw := &brotliResponseWriter{
			ResponseWriter: c.Writer,
			brotliWriter:   brotliWriter,
			minLength:      config.MinLength,
		}

		c.Writer = brw
		defer func() {
			brw.Close()
			brotliWriterPool.Put(brotliWriter)
		}()

		c.SetHeader("Vary", "Accept-Encoding")
		c.Next()
	}
}

// ============== Auto Compress Middleware ==============

// CompressConfig defines the config for auto compression middleware
type CompressConfig struct {
	GzipLevel    int                 // Gzip compression level
	BrotliLevel  int                 // Brotli compression level
	MinLength    int                 // Minimum content length to compress
	ContentTypes []string            // Content types to compress
	SkipFunc     func(*Context) bool // Skip compression for certain requests
}

// DefaultCompressConfig returns the default compression config
func DefaultCompressConfig() CompressConfig {
	return CompressConfig{
		GzipLevel:   gzip.DefaultCompression,
		BrotliLevel: brotli.DefaultCompression,
		MinLength:   1024,
		ContentTypes: []string{
			"text/html",
			"text/css",
			"text/plain",
			"text/javascript",
			"application/json",
			"application/xml",
			"application/javascript",
		},
	}
}

// Compress returns an auto compression middleware that selects the best encoding
func Compress() HandlerFunc {
	return CompressWithConfig(DefaultCompressConfig())
}

// CompressWithConfig returns an auto compression middleware with custom config
func CompressWithConfig(config CompressConfig) HandlerFunc {
	if config.GzipLevel == 0 {
		config.GzipLevel = gzip.DefaultCompression
	}
	if config.BrotliLevel == 0 {
		config.BrotliLevel = brotli.DefaultCompression
	}
	if config.MinLength == 0 {
		config.MinLength = 1024
	}

	return func(c *Context) {
		// Skip if configured
		if config.SkipFunc != nil && config.SkipFunc(c) {
			c.Next()
			return
		}

		acceptEncoding := c.Header("Accept-Encoding")

		// Prefer brotli over gzip
		if strings.Contains(acceptEncoding, "br") {
			brotliWriter := brotliWriterPool.Get().(*brotli.Writer)
			brotliWriter.Reset(c.Writer)

			brw := &brotliResponseWriter{
				ResponseWriter: c.Writer,
				brotliWriter:   brotliWriter,
				minLength:      config.MinLength,
			}

			c.Writer = brw
			defer func() {
				brw.Close()
				brotliWriterPool.Put(brotliWriter)
			}()

			c.SetHeader("Vary", "Accept-Encoding")
			c.Next()
			return
		}

		if strings.Contains(acceptEncoding, "gzip") {
			gzipWriter := gzipWriterPool.Get().(*gzip.Writer)
			gzipWriter.Reset(c.Writer)

			gzw := &gzipResponseWriter{
				ResponseWriter: c.Writer,
				gzipWriter:     gzipWriter,
				minLength:      config.MinLength,
			}

			c.Writer = gzw
			defer func() {
				gzw.Close()
				gzipWriterPool.Put(gzipWriter)
			}()

			c.SetHeader("Vary", "Accept-Encoding")
			c.Next()
			return
		}

		// No compression
		c.Next()
	}
}
