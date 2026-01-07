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
	// PrdMode is production mode with optimized settings
	PrdMode Mode = "prd"
)

const (
	// Environment variable name for mode
	EnvRueMode = "RUE_MODE"
)

// ModeConfig holds the configuration for a mode
type ModeConfig struct {
	LogLevel     LogLevel
	LogFormat    LogFormat
	EnableCaller bool
	EnableColor  bool
}

// Global configuration state
var (
	globalConfig = &struct {
		sync.RWMutex
		mode         Mode
		logLevel     *LogLevel
		logFormat    *LogFormat
		enableCaller *bool
		enableColor  *bool
	}{
		mode: DevMode,
	}
)

func init() {
	// Check environment variable
	if mode := os.Getenv(EnvRueMode); mode != "" {
		SetMode(Mode(mode))
	}
}

// ModeBuilder provides fluent API for configuring mode settings
type ModeBuilder struct{}

// SetMode sets the running mode and returns a builder for chaining
func SetMode(mode Mode) *ModeBuilder {
	globalConfig.Lock()
	defer globalConfig.Unlock()

	switch mode {
	case DevMode, PrdMode:
		globalConfig.mode = mode
	default:
		globalConfig.mode = DevMode
	}

	// Reset custom overrides when mode changes
	globalConfig.logLevel = nil
	globalConfig.logFormat = nil
	globalConfig.enableCaller = nil
	globalConfig.enableColor = nil

	return &ModeBuilder{}
}

// LogLevel sets a custom log level (overrides mode default)
func (b *ModeBuilder) LogLevel(level LogLevel) *ModeBuilder {
	globalConfig.Lock()
	defer globalConfig.Unlock()
	globalConfig.logLevel = &level
	return b
}

// Format sets a custom log format (overrides mode default)
func (b *ModeBuilder) Format(format LogFormat) *ModeBuilder {
	globalConfig.Lock()
	defer globalConfig.Unlock()
	globalConfig.logFormat = &format
	return b
}

// EnableColor sets whether to enable colored output (overrides mode default)
func (b *ModeBuilder) EnableColor(enable bool) *ModeBuilder {
	globalConfig.Lock()
	defer globalConfig.Unlock()
	globalConfig.enableColor = &enable
	return b
}

// EnableCaller sets whether to enable caller info (overrides mode default)
func (b *ModeBuilder) EnableCaller(enable bool) *ModeBuilder {
	globalConfig.Lock()
	defer globalConfig.Unlock()
	globalConfig.enableCaller = &enable
	return b
}

// GetMode returns the current running mode
func GetMode() Mode {
	globalConfig.RLock()
	defer globalConfig.RUnlock()
	return globalConfig.mode
}

// IsDevMode returns true if running in development mode
func IsDevMode() bool {
	return GetMode() == DevMode
}

// IsPrdMode returns true if running in production mode
func IsPrdMode() bool {
	return GetMode() == PrdMode
}

// getDefaultConfig returns the default configuration for a mode
func getDefaultConfig(mode Mode) ModeConfig {
	switch mode {
	case PrdMode:
		return ModeConfig{
			LogLevel:     InfoLevel,
			LogFormat:    JSONFormat,
			EnableCaller: true,
			EnableColor:  true,
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

// GetModeConfig returns the effective configuration (mode defaults + overrides)
func GetModeConfig() ModeConfig {
	globalConfig.RLock()
	defer globalConfig.RUnlock()

	// Start with mode defaults
	config := getDefaultConfig(globalConfig.mode)

	// Apply overrides
	if globalConfig.logLevel != nil {
		config.LogLevel = *globalConfig.logLevel
	}
	if globalConfig.logFormat != nil {
		config.LogFormat = *globalConfig.logFormat
	}
	if globalConfig.enableCaller != nil {
		config.EnableCaller = *globalConfig.enableCaller
	}
	if globalConfig.enableColor != nil {
		config.EnableColor = *globalConfig.enableColor
	}

	return config
}
