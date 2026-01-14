package rue

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// QUICConfig defines QUIC/HTTP3 configuration
type QUICConfig struct {
	// TLS configuration (required for QUIC)
	TLSConfig *tls.Config

	// QUIC specific settings
	MaxIncomingStreams    int64
	MaxIncomingUniStreams int64
	IdleTimeout           time.Duration

	// 0-RTT settings
	Enable0RTT bool

	// Alt-Svc header configuration
	AltSvcPort int // Port to advertise in Alt-Svc header
}

// DefaultQUICConfig returns default QUIC configuration
func DefaultQUICConfig() QUICConfig {
	return QUICConfig{
		MaxIncomingStreams:    100,
		MaxIncomingUniStreams: 100,
		IdleTimeout:           30 * time.Second,
		Enable0RTT:            false,
	}
}

// HTTP3Server wraps the HTTP/3 server
type HTTP3Server struct {
	engine *Engine
	server *http3.Server
	config QUICConfig
}

// NewHTTP3Server creates a new HTTP/3 server
func NewHTTP3Server(engine *Engine, config QUICConfig) *HTTP3Server {
	return &HTTP3Server{
		engine: engine,
		config: config,
	}
}

// ListenAndServe starts the HTTP/3 server
func (s *HTTP3Server) ListenAndServe(addr, certFile, keyFile string) error {
	// Load TLS certificate
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return err
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"h3"},
	}

	// Create QUIC config
	quicConfig := &quic.Config{
		MaxIncomingStreams:    s.config.MaxIncomingStreams,
		MaxIncomingUniStreams: s.config.MaxIncomingUniStreams,
		Allow0RTT:             s.config.Enable0RTT,
	}

	if s.config.IdleTimeout > 0 {
		quicConfig.MaxIdleTimeout = s.config.IdleTimeout
	}

	// Create HTTP/3 server
	s.server = &http3.Server{
		Addr:       addr,
		Handler:    s.engine,
		TLSConfig:  tlsConfig,
		QUICConfig: quicConfig,
	}

	return s.server.ListenAndServe()
}

// Close closes the HTTP/3 server
func (s *HTTP3Server) Close() error {
	if s.server != nil {
		return s.server.Close()
	}
	return nil
}

// CloseGracefully closes the server gracefully
func (s *HTTP3Server) CloseGracefully(timeout time.Duration) error {
	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		return s.server.Shutdown(ctx)
	}
	return nil
}

// RunQUIC starts the HTTP/3 server on the engine
func (e *Engine) RunQUIC(addr, certFile, keyFile string) error {
	return e.RunQUICWithConfig(addr, certFile, keyFile, DefaultQUICConfig())
}

// RunQUICWithConfig starts the HTTP/3 server with custom config
func (e *Engine) RunQUICWithConfig(addr, certFile, keyFile string, config QUICConfig) error {
	server := NewHTTP3Server(e, config)
	return server.ListenAndServe(addr, certFile, keyFile)
}

// AltSvc returns a middleware that adds Alt-Svc header for HTTP/3
func AltSvc(port int) HandlerFunc {
	altSvcValue := "h3=\":" + itoa(port) + "\""

	return func(c *Context) {
		c.SetHeader("Alt-Svc", altSvcValue)
		c.Next()
	}
}

// AltSvcWithMaxAge returns a middleware that adds Alt-Svc header with max-age
func AltSvcWithMaxAge(port int, maxAge int) HandlerFunc {
	altSvcValue := "h3=\":" + itoa(port) + "\"; ma=" + itoa(maxAge)

	return func(c *Context) {
		c.SetHeader("Alt-Svc", altSvcValue)
		c.Next()
	}
}

// itoa converts int to string (simple implementation)
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var digits []byte
	negative := n < 0
	if negative {
		n = -n
	}

	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if negative {
		digits = append([]byte{'-'}, digits...)
	}

	return string(digits)
}

// RunDualStack starts both HTTP/1.1+2 and HTTP/3 servers
func (e *Engine) RunDualStack(httpAddr, http3Addr, certFile, keyFile string) error {
	// Add Alt-Svc middleware to advertise HTTP/3
	// Extract port from http3Addr
	port := 443 // default
	for i := len(http3Addr) - 1; i >= 0; i-- {
		if http3Addr[i] == ':' {
			p := 0
			for j := i + 1; j < len(http3Addr); j++ {
				p = p*10 + int(http3Addr[j]-'0')
			}
			if p > 0 {
				port = p
			}
			break
		}
	}

	e.Use(AltSvcWithMaxAge(port, 86400))

	// Start HTTP/3 server in goroutine
	errChan := make(chan error, 2)

	go func() {
		errChan <- e.RunQUIC(http3Addr, certFile, keyFile)
	}()

	// Start HTTP/1.1+2 server
	go func() {
		errChan <- e.RunTLS(httpAddr, certFile, keyFile)
	}()

	// Wait for first error
	return <-errChan
}

// HTTP3Handler wraps a handler to work with HTTP/3
type HTTP3Handler struct {
	handler http.Handler
}

// ServeHTTP implements http.Handler
func (h *HTTP3Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Add HTTP/3 specific headers if needed
	if r.ProtoMajor == 3 {
		w.Header().Set("X-Protocol", "HTTP/3")
	}
	h.handler.ServeHTTP(w, r)
}
