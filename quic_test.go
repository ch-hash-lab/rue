package rue

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDefaultQUICConfig(t *testing.T) {
	config := DefaultQUICConfig()

	if config.MaxIncomingStreams != 100 {
		t.Errorf("MaxIncomingStreams = %d, want 100", config.MaxIncomingStreams)
	}
	if config.MaxIncomingUniStreams != 100 {
		t.Errorf("MaxIncomingUniStreams = %d, want 100", config.MaxIncomingUniStreams)
	}
	if config.IdleTimeout != 30*time.Second {
		t.Errorf("IdleTimeout = %v, want 30s", config.IdleTimeout)
	}
	if config.Enable0RTT {
		t.Error("Enable0RTT should be false by default")
	}
}

func TestNewHTTP3Server(t *testing.T) {
	engine := New()
	config := DefaultQUICConfig()

	server := NewHTTP3Server(engine, config)

	if server.engine != engine {
		t.Error("Engine not set correctly")
	}
	if server.config.MaxIncomingStreams != config.MaxIncomingStreams {
		t.Error("Config not set correctly")
	}
}

func TestAltSvc(t *testing.T) {
	engine := New()
	engine.Use(AltSvc(443))
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	altSvc := w.Header().Get("Alt-Svc")
	if altSvc == "" {
		t.Error("Alt-Svc header should be set")
	}
}

func TestAltSvcWithMaxAge(t *testing.T) {
	engine := New()
	engine.Use(AltSvcWithMaxAge(8443, 3600))
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	engine.ServeHTTP(w, req)

	altSvc := w.Header().Get("Alt-Svc")
	if altSvc == "" {
		t.Error("Alt-Svc header should be set")
	}
	if altSvc != "h3=\":8443\"; ma=3600" {
		t.Errorf("Alt-Svc = %s, want 'h3=\":8443\"; ma=3600'", altSvc)
	}
}

func TestItoa(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{10, "10"},
		{123, "123"},
		{8443, "8443"},
		{-1, "-1"},
		{-123, "-123"},
	}

	for _, tt := range tests {
		result := itoa(tt.input)
		if result != tt.expected {
			t.Errorf("itoa(%d) = %s, want %s", tt.input, result, tt.expected)
		}
	}
}

func TestHTTP3Handler(t *testing.T) {
	engine := New()
	engine.GET("/test", func(c *Context) {
		c.Text(http.StatusOK, "OK")
	})

	handler := &HTTP3Handler{handler: engine}

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHTTP3Server_Close(t *testing.T) {
	engine := New()
	config := DefaultQUICConfig()
	server := NewHTTP3Server(engine, config)

	// Close without starting should not error
	err := server.Close()
	if err != nil {
		t.Errorf("Close error: %v", err)
	}
}

func TestHTTP3Server_CloseGracefully(t *testing.T) {
	engine := New()
	config := DefaultQUICConfig()
	server := NewHTTP3Server(engine, config)

	// CloseGracefully without starting should not error
	err := server.CloseGracefully(5 * time.Second)
	if err != nil {
		t.Errorf("CloseGracefully error: %v", err)
	}
}

// Note: Full HTTP/3 integration tests require TLS certificates
// and are typically run in integration test environments

func TestQUICConfig_CustomValues(t *testing.T) {
	config := QUICConfig{
		MaxIncomingStreams:    200,
		MaxIncomingUniStreams: 50,
		IdleTimeout:           60 * time.Second,
		Enable0RTT:            true,
		AltSvcPort:            8443,
	}

	if config.MaxIncomingStreams != 200 {
		t.Errorf("MaxIncomingStreams = %d, want 200", config.MaxIncomingStreams)
	}
	if config.MaxIncomingUniStreams != 50 {
		t.Errorf("MaxIncomingUniStreams = %d, want 50", config.MaxIncomingUniStreams)
	}
	if config.IdleTimeout != 60*time.Second {
		t.Errorf("IdleTimeout = %v, want 60s", config.IdleTimeout)
	}
	if !config.Enable0RTT {
		t.Error("Enable0RTT should be true")
	}
	if config.AltSvcPort != 8443 {
		t.Errorf("AltSvcPort = %d, want 8443", config.AltSvcPort)
	}
}
