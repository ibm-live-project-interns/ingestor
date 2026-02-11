package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ibm-live-project-interns/ingestor/shared/config"
)

// Level represents log level
type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
	FATAL
)

func (l Level) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a string to Level
func ParseLevel(s string) Level {
	switch s {
	case "DEBUG", "debug":
		return DEBUG
	case "INFO", "info":
		return INFO
	case "WARN", "warn", "WARNING", "warning":
		return WARN
	case "ERROR", "error":
		return ERROR
	case "FATAL", "fatal":
		return FATAL
	default:
		return INFO
	}
}

// LoggerConfig holds logger configuration
type LoggerConfig struct {
	// Minimum level to log
	Level Level
	// Service name for log prefix
	ServiceName string
	// Output writer (default: os.Stdout)
	Output io.Writer
	// Also write to file
	FileOutput string
	// Include caller information
	IncludeCaller bool
}

// Logger is a structured logger
type Logger struct {
	config     LoggerConfig
	stdLogger  *log.Logger
	fileLogger *log.Logger
	mu         sync.Mutex
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// DefaultLoggerConfig returns default logger settings
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		Level:         ParseLevel(config.GetEnv("LOG_LEVEL", "INFO")),
		ServiceName:   config.GetEnv("SERVICE_NAME", "service"),
		Output:        os.Stdout,
		FileOutput:    config.GetEnv("LOG_FILE", ""),
		IncludeCaller: config.GetEnvBool("LOG_INCLUDE_CALLER", false),
	}
}

// Init initializes the default logger
func Init(cfg LoggerConfig) *Logger {
	once.Do(func() {
		defaultLogger = New(cfg)
	})
	return defaultLogger
}

// Default returns the default logger (initializes with defaults if not already initialized)
func Default() *Logger {
	if defaultLogger == nil {
		Init(DefaultLoggerConfig())
	}
	return defaultLogger
}

// New creates a new logger
func New(cfg LoggerConfig) *Logger {
	l := &Logger{
		config:    cfg,
		stdLogger: log.New(cfg.Output, "", 0),
	}

	// Setup file logging if configured
	if cfg.FileOutput != "" {
		// Ensure directory exists
		dir := filepath.Dir(cfg.FileOutput)
		if err := os.MkdirAll(dir, 0755); err == nil {
			file, err := os.OpenFile(cfg.FileOutput, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if err == nil {
				l.fileLogger = log.New(file, "", 0)
			}
		}
	}

	return l
}

// log outputs a log message
func (l *Logger) log(level Level, format string, args ...interface{}) {
	if level < l.config.Level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	message := fmt.Sprintf(format, args...)
	prefix := fmt.Sprintf("[%s] [%s] [%s] ", timestamp, l.config.ServiceName, level.String())
	logLine := prefix + message

	l.stdLogger.Println(logLine)

	if l.fileLogger != nil {
		l.fileLogger.Println(logLine)
	}
}

// Debug logs a debug message
func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

// Info logs an info message
func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

// Warn logs a warning message
func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

// Error logs an error message
func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
	os.Exit(1)
}

// WithField returns a logger with a field context (for future structured logging)
func (l *Logger) WithField(key string, value interface{}) *Logger {
	// For now, just return the same logger
	// This can be expanded for structured logging
	return l
}

// ===== Package-level functions using default logger =====

// Debug logs a debug message using the default logger
func Debug(format string, args ...interface{}) {
	Default().Debug(format, args...)
}

// Info logs an info message using the default logger
func Info(format string, args ...interface{}) {
	Default().Info(format, args...)
}

// Warn logs a warning message using the default logger
func Warn(format string, args ...interface{}) {
	Default().Warn(format, args...)
}

// Error logs an error message using the default logger
func Error(format string, args ...interface{}) {
	Default().Error(format, args...)
}

// Fatal logs a fatal message and exits using the default logger
func Fatal(format string, args ...interface{}) {
	Default().Fatal(format, args...)
}
