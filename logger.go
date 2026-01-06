package rue

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bytedance/sonic"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	// DebugLevel is for debug messages
	DebugLevel LogLevel = iota
	// InfoLevel is for informational messages
	InfoLevel
	// WarnLevel is for warning messages
	WarnLevel
	// ErrorLevel is for error messages
	ErrorLevel
	// FatalLevel is for fatal messages
	FatalLevel
	// StatLevel is for statistics messages
	StatLevel
)

// String returns the string representation of the log level
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case FatalLevel:
		return "fatal"
	case StatLevel:
		return "stat"
	default:
		return "unknown"
	}
}

// Color returns the ANSI color code for the log level
func (l LogLevel) Color() string {
	switch l {
	case DebugLevel:
		return "\033[36m" // Cyan
	case InfoLevel:
		return "\033[32m" // Green
	case WarnLevel:
		return "\033[33m" // Yellow
	case ErrorLevel:
		return "\033[31m" // Red
	case FatalLevel:
		return "\033[35m" // Magenta
	case StatLevel:
		return "\033[34m" // Blue
	default:
		return "\033[0m"
	}
}

// LogFormat represents the output format of logs
type LogFormat int

const (
	// TextFormat outputs logs in human-readable text format
	TextFormat LogFormat = iota
	// JSONFormat outputs logs in JSON format
	JSONFormat
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp string `json:"@timestamp"`
	Level     string `json:"level"`
	Caller    string `json:"caller,omitempty"`
	Func      string `json:"func,omitempty"`
	Content   string `json:"content,omitempty"`
	// HTTP request fields
	Method   string `json:"method,omitempty"`
	Path     string `json:"path,omitempty"`
	Status   int    `json:"status,omitempty"`
	Latency  string `json:"latency,omitempty"`
	ClientIP string `json:"client_ip,omitempty"`
	// Error fields
	Error string `json:"error,omitempty"`
	Stack string `json:"stack,omitempty"`
	// Custom fields
	Fields map[string]any `json:"fields,omitempty"`
}

// Logger is a structured logger
type Logger struct {
	mu           sync.Mutex
	level        LogLevel
	format       LogFormat
	output       io.Writer
	enableCaller bool
	callerSkip   int
	enableColor  bool
	timeFormat   string
	fields       map[string]any
}

// LoggerOption is a function that configures a Logger
type LoggerOption func(*Logger)

// NewLogger creates a new Logger with the given options
func NewLogger(opts ...LoggerOption) *Logger {
	config := GetModeConfig()
	l := &Logger{
		level:        config.LogLevel,
		format:       config.LogFormat,
		output:       os.Stdout,
		enableCaller: config.EnableCaller,
		callerSkip:   2,
		enableColor:  config.EnableColor,
		timeFormat:   time.RFC3339Nano,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// WithLevel sets the log level
func WithLevel(level LogLevel) LoggerOption {
	return func(l *Logger) {
		l.level = level
	}
}

// WithFormat sets the log format
func WithFormat(format LogFormat) LoggerOption {
	return func(l *Logger) {
		l.format = format
	}
}

// WithOutput sets the output writer
func WithOutput(w io.Writer) LoggerOption {
	return func(l *Logger) {
		l.output = w
	}
}

// WithCaller enables or disables caller information
func WithCaller(enable bool) LoggerOption {
	return func(l *Logger) {
		l.enableCaller = enable
	}
}

// WithCallerSkip sets the number of stack frames to skip
func WithCallerSkip(skip int) LoggerOption {
	return func(l *Logger) {
		l.callerSkip = skip
	}
}

// WithColor enables or disables colored output
func WithColor(enable bool) LoggerOption {
	return func(l *Logger) {
		l.enableColor = enable
	}
}

// WithTimeFormat sets the time format
func WithTimeFormat(format string) LoggerOption {
	return func(l *Logger) {
		l.timeFormat = format
	}
}

// SetLevel sets the log level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetFormat sets the log format
func (l *Logger) SetFormat(format LogFormat) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.format = format
}

// WithFields returns a new logger with the given fields
func (l *Logger) WithFields(fields map[string]any) *Logger {
	newLogger := &Logger{
		level:        l.level,
		format:       l.format,
		output:       l.output,
		enableCaller: l.enableCaller,
		callerSkip:   l.callerSkip,
		enableColor:  l.enableColor,
		timeFormat:   l.timeFormat,
		fields:       make(map[string]any),
	}
	// Copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	// Add new fields
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

// log writes a log entry
func (l *Logger) log(level LogLevel, msg string, err error) {
	if level < l.level && level != StatLevel {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now().UTC().Format(l.timeFormat),
		Level:     level.String(),
		Content:   msg,
	}

	// Add caller info for error level and above, or if explicitly enabled
	if l.enableCaller && (level >= ErrorLevel || level == StatLevel) {
		if caller, fn := l.getCaller(); caller != "" {
			entry.Caller = caller
			entry.Func = fn
		}
	}

	// Add error info
	if err != nil {
		entry.Error = err.Error()
		if level >= ErrorLevel {
			entry.Stack = l.getStack()
		}
	}

	// Add custom fields
	if len(l.fields) > 0 {
		entry.Fields = l.fields
	}

	l.writeEntry(level, &entry)
}

// getCaller returns the caller file:line and function name
func (l *Logger) getCaller() (string, string) {
	pc, file, line, ok := runtime.Caller(l.callerSkip + 2)
	if !ok {
		return "", ""
	}

	// Get short file path
	file = filepath.Base(file)

	// Get function name
	fn := runtime.FuncForPC(pc)
	funcName := ""
	if fn != nil {
		funcName = fn.Name()
		// Get short function name
		if idx := strings.LastIndex(funcName, "/"); idx >= 0 {
			funcName = funcName[idx+1:]
		}
	}

	return fmt.Sprintf("%s:%d", file, line), funcName
}

// getStack returns the stack trace
func (l *Logger) getStack() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	return string(buf[:n])
}

// writeEntry writes the log entry to output
func (l *Logger) writeEntry(level LogLevel, entry *LogEntry) {
	if l.format == JSONFormat {
		l.writeJSON(entry)
	} else {
		l.writeText(level, entry)
	}
}

// writeJSON writes the entry in JSON format
func (l *Logger) writeJSON(entry *LogEntry) {
	data, err := sonic.Marshal(entry)
	if err != nil {
		return
	}
	l.output.Write(data)
	l.output.Write([]byte("\n"))
}

// writeText writes the entry in text format
func (l *Logger) writeText(level LogLevel, entry *LogEntry) {
	var buf strings.Builder

	// Color prefix
	if l.enableColor {
		buf.WriteString(level.Color())
	}

	// Timestamp
	buf.WriteString("[RUE] ")
	t, _ := time.Parse(l.timeFormat, entry.Timestamp)
	buf.WriteString(t.Local().Format("2006/01/02 - 15:04:05"))

	// Level
	buf.WriteString(" | ")
	buf.WriteString(strings.ToUpper(entry.Level))

	// Caller
	if entry.Caller != "" {
		buf.WriteString(" | ")
		buf.WriteString(entry.Caller)
		if entry.Func != "" {
			buf.WriteString(" | ")
			buf.WriteString(entry.Func)
		}
	}

	// Content
	if entry.Content != "" {
		buf.WriteString(" | ")
		buf.WriteString(entry.Content)
	}

	// Error
	if entry.Error != "" {
		buf.WriteString(" | error=")
		buf.WriteString(entry.Error)
	}

	// Reset color
	if l.enableColor {
		buf.WriteString("\033[0m")
	}

	buf.WriteString("\n")

	// Stack trace (only for errors)
	if entry.Stack != "" && IsDevMode() {
		buf.WriteString(entry.Stack)
		buf.WriteString("\n")
	}

	l.output.Write([]byte(buf.String()))
}

// Debug logs a debug message
func (l *Logger) Debug(msg string) {
	l.log(DebugLevel, msg, nil)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...any) {
	l.log(DebugLevel, fmt.Sprintf(format, args...), nil)
}

// Info logs an info message
func (l *Logger) Info(msg string) {
	l.log(InfoLevel, msg, nil)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...any) {
	l.log(InfoLevel, fmt.Sprintf(format, args...), nil)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string) {
	l.log(WarnLevel, msg, nil)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...any) {
	l.log(WarnLevel, fmt.Sprintf(format, args...), nil)
}

// Error logs an error message
func (l *Logger) Error(msg string) {
	l.log(ErrorLevel, msg, nil)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...any) {
	l.log(ErrorLevel, fmt.Sprintf(format, args...), nil)
}

// ErrorWithErr logs an error message with an error
func (l *Logger) ErrorWithErr(msg string, err error) {
	l.log(ErrorLevel, msg, err)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string) {
	l.log(FatalLevel, msg, nil)
	os.Exit(1)
}

// Fatalf logs a formatted fatal message and exits
func (l *Logger) Fatalf(format string, args ...any) {
	l.log(FatalLevel, fmt.Sprintf(format, args...), nil)
	os.Exit(1)
}

// Stat logs a statistics message
func (l *Logger) Stat(msg string) {
	l.log(StatLevel, msg, nil)
}

// Statf logs a formatted statistics message
func (l *Logger) Statf(format string, args ...any) {
	l.log(StatLevel, fmt.Sprintf(format, args...), nil)
}

// Default logger instance
var defaultLogger = NewLogger()

// SetDefaultLogger sets the default logger
func SetDefaultLogger(l *Logger) {
	defaultLogger = l
}

// GetDefaultLogger returns the default logger
func GetDefaultLogger() *Logger {
	return defaultLogger
}

// Package-level logging functions
func LogDebug(msg string)                   { defaultLogger.Debug(msg) }
func LogDebugf(format string, args ...any)  { defaultLogger.Debugf(format, args...) }
func LogInfo(msg string)                    { defaultLogger.Info(msg) }
func LogInfof(format string, args ...any)   { defaultLogger.Infof(format, args...) }
func LogWarn(msg string)                    { defaultLogger.Warn(msg) }
func LogWarnf(format string, args ...any)   { defaultLogger.Warnf(format, args...) }
func LogError(msg string)                   { defaultLogger.Error(msg) }
func LogErrorf(format string, args ...any)  { defaultLogger.Errorf(format, args...) }
func LogErrorWithErr(msg string, err error) { defaultLogger.ErrorWithErr(msg, err) }
func LogFatal(msg string)                   { defaultLogger.Fatal(msg) }
func LogFatalf(format string, args ...any)  { defaultLogger.Fatalf(format, args...) }
func LogStat(msg string)                    { defaultLogger.Stat(msg) }
func LogStatf(format string, args ...any)   { defaultLogger.Statf(format, args...) }
