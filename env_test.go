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
		{"TestMode", TestMode, TestMode},
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

func TestIsTestMode(t *testing.T) {
	SetMode(TestMode)
	if !IsTestMode() {
		t.Error("IsTestMode() should return true when mode is TestMode")
	}

	SetMode(DevMode)
	if IsTestMode() {
		t.Error("IsTestMode() should return false when mode is DevMode")
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
	}{
		{"DevMode config", DevMode, DebugLevel, TextFormat},
		{"TestMode config", TestMode, InfoLevel, TextFormat},
		{"PrdMode config", PrdMode, InfoLevel, JSONFormat},
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
		})
	}

	// Reset to default
	SetMode(DevMode)
}
