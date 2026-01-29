package logger

import (
	"io"
	"log"
	"os"
	"strings"
)

// Level represents the logging level
type Level int

const (
	// DEBUG level for verbose logging (message content, transformations)
	DEBUG Level = iota
	// INFO level for general events
	INFO
	// ERROR level for errors only
	ERROR
)

// Logger wraps the standard log.Logger with level support
type Logger struct {
	level       Level
	debugLogger *log.Logger
	infoLogger  *log.Logger
	errorLogger *log.Logger
}

// New creates a new Logger with the specified level
func New(level Level) *Logger {
	return &Logger{
		level:       level,
		debugLogger: log.New(os.Stdout, "DEBUG: ", log.LstdFlags),
		infoLogger:  log.New(os.Stdout, "INFO: ", log.LstdFlags),
		errorLogger: log.New(os.Stderr, "ERROR: ", log.LstdFlags),
	}
}

// SetOutput sets the output destination for all loggers
func (l *Logger) SetOutput(w io.Writer) {
	l.debugLogger.SetOutput(w)
	l.infoLogger.SetOutput(w)
	l.errorLogger.SetOutput(w)
}

// Debug logs a message at DEBUG level
func (l *Logger) Debug(v ...interface{}) {
	if l.level <= DEBUG {
		l.debugLogger.Println(v...)
	}
}

// Debugf logs a formatted message at DEBUG level
func (l *Logger) Debugf(format string, v ...interface{}) {
	if l.level <= DEBUG {
		l.debugLogger.Printf(format, v...)
	}
}

// Info logs a message at INFO level
func (l *Logger) Info(v ...interface{}) {
	if l.level <= INFO {
		l.infoLogger.Println(v...)
	}
}

// Infof logs a formatted message at INFO level
func (l *Logger) Infof(format string, v ...interface{}) {
	if l.level <= INFO {
		l.infoLogger.Printf(format, v...)
	}
}

// Error logs a message at ERROR level
func (l *Logger) Error(v ...interface{}) {
	if l.level <= ERROR {
		l.errorLogger.Println(v...)
	}
}

// Errorf logs a formatted message at ERROR level
func (l *Logger) Errorf(format string, v ...interface{}) {
	if l.level <= ERROR {
		l.errorLogger.Printf(format, v...)
	}
}

// ParseLevel converts a string to a Level
func ParseLevel(s string) Level {
	switch strings.ToUpper(s) {
	case "DEBUG":
		return DEBUG
	case "INFO":
		return INFO
	case "ERROR":
		return ERROR
	default:
		return INFO
	}
}
