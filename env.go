package rue

import (
	"os"
	"sync"
)

// Mode represents the running mode of the framework
type Mode string

const (
	// DevMode is development mode with verbose logging
	DevMode Mode = "dev"
	// TestMode is testing mode
	TestMode Mode = "test"
	// PrdMode is production mode with optimized settings
	PrdMode Mode = "prd"
)

const (
	// Environment variable name for mode
	EnvRueMode = "RUE_MODE"
)

var (
	rueMode     = DevMode
	rueModeLock sync.RWMutex
)

func init() {
	// Check environment variable
	if mode := os.Getenv(EnvRueMode); mode != "" {
		SetMode(Mode(mode))
	}
}

// SetMode sets the running mode of the framework
func SetMode(mode Mode) {
	rueModeLock.Lock()
	defer rueModeLock.Unlock()

	switch mode {
	case DevMode, TestMode, PrdMode:
		rueMode = mode
	default:
		rueMode = DevMode
	}
}

// GetMode returns the current running mode
func GetMode() Mode {
	rueModeLock.RLock()
	defer rueModeLock.RUnlock()
	return rueMode
}

// IsDevMode returns true if running in development mode
func IsDevMode() bool {
	return GetMode() == DevMode
}

// IsTestMode returns true if running in test mode
func IsTestMode() bool {
	return GetMode() == TestMode
}

// IsPrdMode returns true if running in production mode
func IsPrdMode() bool {
	return GetMode() == PrdMode
}

// ModeConfig returns default configuration based on current mode
type ModeConfig struct {
	LogLevel     LogLevel
	LogFormat    LogFormat
	EnableCaller bool
	EnableColor  bool
}

// GetModeConfig returns the default configuration for the current mode
func GetModeConfig() ModeConfig {
	switch GetMode() {
	case PrdMode:
		return ModeConfig{
			LogLevel:     InfoLevel,
			LogFormat:    JSONFormat,
			EnableCaller: true,
			EnableColor:  false,
		}
	case TestMode:
		return ModeConfig{
			LogLevel:     InfoLevel,
			LogFormat:    TextFormat,
			EnableCaller: true,
			EnableColor:  false,
		}
	default: // DevMode
		return ModeConfig{
			LogLevel:     DebugLevel,
			LogFormat:    TextFormat,
			EnableCaller: true,
			EnableColor:  true,
		}
	}
}
