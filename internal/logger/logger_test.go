package logger

import (
	"bytes"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	logger := New(INFO)
	if logger == nil {
		t.Fatal("Expected non-nil logger")
	}
	if logger.level != INFO {
		t.Errorf("Expected level INFO, got %v", logger.level)
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected Level
	}{
		{"DEBUG", DEBUG},
		{"debug", DEBUG},
		{"INFO", INFO},
		{"info", INFO},
		{"ERROR", ERROR},
		{"error", ERROR},
		{"unknown", INFO}, // defaults to INFO
		{"", INFO},        // defaults to INFO
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := ParseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestLoggerLevels(t *testing.T) {
	tests := []struct {
		name          string
		level         Level
		logFunc       func(*Logger, *bytes.Buffer)
		shouldContain string
	}{
		{
			name:  "debug logger logs at DEBUG level",
			level: DEBUG,
			logFunc: func(l *Logger, buf *bytes.Buffer) {
				l.SetOutput(buf)
				l.Debug("debug message")
			},
			shouldContain: "debug message",
		},
		{
			name:  "debug logger does not log at INFO level",
			level: INFO,
			logFunc: func(l *Logger, buf *bytes.Buffer) {
				l.SetOutput(buf)
				l.Debug("debug message")
			},
			shouldContain: "",
		},
		{
			name:  "info logger logs at INFO level",
			level: INFO,
			logFunc: func(l *Logger, buf *bytes.Buffer) {
				l.SetOutput(buf)
				l.Info("info message")
			},
			shouldContain: "info message",
		},
		{
			name:  "info logger logs at DEBUG level",
			level: DEBUG,
			logFunc: func(l *Logger, buf *bytes.Buffer) {
				l.SetOutput(buf)
				l.Info("info message")
			},
			shouldContain: "info message",
		},
		{
			name:  "error logger logs at all levels",
			level: ERROR,
			logFunc: func(l *Logger, buf *bytes.Buffer) {
				l.SetOutput(buf)
				l.Error("error message")
			},
			shouldContain: "error message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := New(tt.level)
			buf := &bytes.Buffer{}
			tt.logFunc(logger, buf)

			output := buf.String()
			if tt.shouldContain != "" {
				if !strings.Contains(output, tt.shouldContain) {
					t.Errorf("Expected output to contain %q, got: %s", tt.shouldContain, output)
				}
			} else {
				if output != "" {
					t.Errorf("Expected no output, got: %s", output)
				}
			}
		})
	}
}

func TestLoggerFormatted(t *testing.T) {
	logger := New(DEBUG)
	buf := &bytes.Buffer{}
	logger.SetOutput(buf)

	logger.Debugf("formatted %s %d", "message", 42)
	output := buf.String()
	if !strings.Contains(output, "formatted message 42") {
		t.Errorf("Expected formatted output, got: %s", output)
	}
}
