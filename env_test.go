package rue

import (
	"testing"
)

func TestSetMode(t *testing.T) {
	tests := []struct {
		name     string
		mode     Mode
		expected Mode
	}{
		{"DevMode", DevMode, DevMode},
		{"PrdMode", PrdMode, PrdMode},
		{"Invalid mode defaults to DevMode", "invalid", DevMode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetMode(tt.mode)
			if got := GetMode(); got != tt.expected {
				t.Errorf("GetMode() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestIsDevMode(t *testing.T) {
	SetMode(DevMode)
	if !IsDevMode() {
		t.Error("IsDevMode() should return true when mode is DevMode")
	}

	SetMode(PrdMode)
	if IsDevMode() {
		t.Error("IsDevMode() should return false when mode is PrdMode")
	}
}

func TestIsPrdMode(t *testing.T) {
	SetMode(PrdMode)
	if !IsPrdMode() {
		t.Error("IsPrdMode() should return true when mode is PrdMode")
	}

	SetMode(DevMode)
	if IsPrdMode() {
		t.Error("IsPrdMode() should return false when mode is DevMode")
	}
}

func TestGetModeConfig(t *testing.T) {
	tests := []struct {
		name           string
		mode           Mode
		expectedLevel  LogLevel
		expectedFormat LogFormat
		expectedColor  bool
	}{
		{"DevMode config", DevMode, DebugLevel, TextFormat, true},
		{"PrdMode config", PrdMode, InfoLevel, JSONFormat, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetMode(tt.mode)
			config := GetModeConfig()

			if config.LogLevel != tt.expectedLevel {
				t.Errorf("LogLevel = %v, want %v", config.LogLevel, tt.expectedLevel)
			}
			if config.LogFormat != tt.expectedFormat {
				t.Errorf("LogFormat = %v, want %v", config.LogFormat, tt.expectedFormat)
			}
			if config.EnableColor != tt.expectedColor {
				t.Errorf("EnableColor = %v, want %v", config.EnableColor, tt.expectedColor)
			}
		})
	}

	// Reset to default
	SetMode(DevMode)
}

func TestModeBuilderChaining(t *testing.T) {
	// Test chaining configuration
	SetMode(PrdMode).
		LogLevel(DebugLevel).
		Format(TextFormat).
		EnableColor(false).
		EnableCaller(false)

	config := GetModeConfig()

	if config.LogLevel != DebugLevel {
		t.Errorf("LogLevel = %v, want %v", config.LogLevel, DebugLevel)
	}
	if config.LogFormat != TextFormat {
		t.Errorf("LogFormat = %v, want %v", config.LogFormat, TextFormat)
	}
	if config.EnableColor != false {
		t.Errorf("EnableColor = %v, want %v", config.EnableColor, false)
	}
	if config.EnableCaller != false {
		t.Errorf("EnableCaller = %v, want %v", config.EnableCaller, false)
	}

	// Reset to default
	SetMode(DevMode)
}

func TestModeOverrideReset(t *testing.T) {
	// Set overrides
	SetMode(DevMode).LogLevel(ErrorLevel)
	config := GetModeConfig()
	if config.LogLevel != ErrorLevel {
		t.Errorf("LogLevel = %v, want %v", config.LogLevel, ErrorLevel)
	}

	// Change mode should reset overrides
	SetMode(PrdMode)
	config = GetModeConfig()
	if config.LogLevel != InfoLevel {
		t.Errorf("After mode change, LogLevel = %v, want %v (PrdMode default)", config.LogLevel, InfoLevel)
	}

	// Reset to default
	SetMode(DevMode)
}
