package rue

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DebugLevel, "debug"},
		{InfoLevel, "info"},
		{WarnLevel, "warn"},
		{ErrorLevel, "error"},
		{FatalLevel, "fatal"},
		{StatLevel, "stat"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.level.String(); got != tt.expected {
				t.Errorf("LogLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewLogger(t *testing.T) {
	logger := NewLogger()
	if logger == nil {
		t.Fatal("NewLogger() returned nil")
	}
}

func TestLogger_WithOptions(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithLevel(WarnLevel),
		WithFormat(JSONFormat),
		WithOutput(&buf),
		WithCaller(true),
		WithColor(false),
	)

	if logger.level != WarnLevel {
		t.Errorf("level = %v, want %v", logger.level, WarnLevel)
	}
	if logger.format != JSONFormat {
		t.Errorf("format = %v, want %v", logger.format, JSONFormat)
	}
	if logger.enableCaller != true {
		t.Error("enableCaller should be true")
	}
	if logger.enableColor != false {
		t.Error("enableColor should be false")
	}
}

func TestLogger_SetLevel(t *testing.T) {
	logger := NewLogger()
	logger.SetLevel(ErrorLevel)

	if logger.level != ErrorLevel {
		t.Errorf("level = %v, want %v", logger.level, ErrorLevel)
	}
}

func TestLogger_SetFormat(t *testing.T) {
	logger := NewLogger()
	logger.SetFormat(JSONFormat)

	if logger.format != JSONFormat {
		t.Errorf("format = %v, want %v", logger.format, JSONFormat)
	}
}

func TestLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithLevel(DebugLevel),
		WithFormat(TextFormat),
		WithOutput(&buf),
		WithColor(false),
	)

	logger.Debug("test debug message")

	output := buf.String()
	if !strings.Contains(output, "DEBUG") {
		t.Errorf("output should contain DEBUG, got: %s", output)
	}
	if !strings.Contains(output, "test debug message") {
		t.Errorf("output should contain message, got: %s", output)
	}
}

func TestLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithLevel(InfoLevel),
		WithFormat(TextFormat),
		WithOutput(&buf),
		WithColor(false),
	)

	logger.Info("test info message")

	output := buf.String()
	if !strings.Contains(output, "INFO") {
		t.Errorf("output should contain INFO, got: %s", output)
	}
}

func TestLogger_Warn(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithLevel(WarnLevel),
		WithFormat(TextFormat),
		WithOutput(&buf),
		WithColor(false),
	)

	logger.Warn("test warn message")

	output := buf.String()
	if !strings.Contains(output, "WARN") {
		t.Errorf("output should contain WARN, got: %s", output)
	}
}

func TestLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithLevel(ErrorLevel),
		WithFormat(TextFormat),
		WithOutput(&buf),
		WithColor(false),
		WithCaller(true),
	)

	logger.Error("test error message")

	output := buf.String()
	if !strings.Contains(output, "ERROR") {
		t.Errorf("output should contain ERROR, got: %s", output)
	}
	// Should contain caller info for error level
	if !strings.Contains(output, ".go:") {
		t.Errorf("output should contain caller info, got: %s", output)
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithLevel(WarnLevel),
		WithFormat(TextFormat),
		WithOutput(&buf),
		WithColor(false),
	)

	// Debug and Info should be filtered out
	logger.Debug("debug message")
	logger.Info("info message")

	if buf.Len() > 0 {
		t.Errorf("debug and info should be filtered, got: %s", buf.String())
	}

	// Warn and above should be logged
	logger.Warn("warn message")
	if !strings.Contains(buf.String(), "warn message") {
		t.Error("warn message should be logged")
	}
}

func TestLogger_JSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithLevel(InfoLevel),
		WithFormat(JSONFormat),
		WithOutput(&buf),
	)

	logger.Info("test json message")

	output := buf.String()
	if !strings.Contains(output, `"level":"info"`) {
		t.Errorf("JSON output should contain level field, got: %s", output)
	}
	if !strings.Contains(output, `"content":"test json message"`) {
		t.Errorf("JSON output should contain content field, got: %s", output)
	}
	if !strings.Contains(output, `"@timestamp"`) {
		t.Errorf("JSON output should contain @timestamp field, got: %s", output)
	}
}

func TestLogger_WithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithLevel(InfoLevel),
		WithFormat(JSONFormat),
		WithOutput(&buf),
	)

	logger.WithFields(H{
		"user_id": 123,
		"action":  "login",
	}).Info("user logged in")

	output := buf.String()
	if !strings.Contains(output, `"user_id"`) {
		t.Errorf("JSON output should contain user_id field, got: %s", output)
	}
	if !strings.Contains(output, `"action"`) {
		t.Errorf("JSON output should contain action field, got: %s", output)
	}
}

func TestLogger_Stat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithLevel(InfoLevel),
		WithFormat(JSONFormat),
		WithOutput(&buf),
		WithCaller(true),
	)

	logger.Stat("CPU: 17m, MEMORY: Alloc=2.5Mi")

	output := buf.String()
	if !strings.Contains(output, `"level":"stat"`) {
		t.Errorf("output should contain stat level, got: %s", output)
	}
	if !strings.Contains(output, "CPU: 17m") {
		t.Errorf("output should contain stat message, got: %s", output)
	}
	// Stat should include caller info
	if !strings.Contains(output, `"caller"`) {
		t.Errorf("stat output should contain caller info, got: %s", output)
	}
}

func TestLogger_Debugf(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithLevel(DebugLevel),
		WithFormat(TextFormat),
		WithOutput(&buf),
		WithColor(false),
	)

	logger.Debugf("value: %d", 42)

	output := buf.String()
	if !strings.Contains(output, "value: 42") {
		t.Errorf("output should contain formatted message, got: %s", output)
	}
}

func TestLogger_Infof(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(
		WithLevel(InfoLevel),
		WithFormat(TextFormat),
		WithOutput(&buf),
		WithColor(false),
	)

	logger.Infof("user %s logged in", "john")

	output := buf.String()
	if !strings.Contains(output, "user john logged in") {
		t.Errorf("output should contain formatted message, got: %s", output)
	}
}

func TestDefaultLogger(t *testing.T) {
	logger := GetDefaultLogger()
	if logger == nil {
		t.Fatal("GetDefaultLogger() returned nil")
	}

	newLogger := NewLogger()
	SetDefaultLogger(newLogger)

	if GetDefaultLogger() != newLogger {
		t.Error("SetDefaultLogger did not update default logger")
	}
}

func TestPackageLevelLogging(t *testing.T) {
	var buf bytes.Buffer
	SetDefaultLogger(NewLogger(
		WithLevel(DebugLevel),
		WithFormat(TextFormat),
		WithOutput(&buf),
		WithColor(false),
	))

	LogDebug("debug")
	if !strings.Contains(buf.String(), "debug") {
		t.Error("LogDebug should log message")
	}

	buf.Reset()
	LogInfo("info")
	if !strings.Contains(buf.String(), "info") {
		t.Error("LogInfo should log message")
	}

	buf.Reset()
	LogWarn("warn")
	if !strings.Contains(buf.String(), "warn") {
		t.Error("LogWarn should log message")
	}

	buf.Reset()
	LogError("error")
	if !strings.Contains(buf.String(), "error") {
		t.Error("LogError should log message")
	}

	buf.Reset()
	LogStat("stat")
	if !strings.Contains(buf.String(), "stat") {
		t.Error("LogStat should log message")
	}
}
