package cli

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Logger writes progress to a console writer (usually stderr) and,
// optionally, to a timestamped log file.
type Logger struct {
	console io.Writer
	file    *os.File
	quiet   bool
}

func NewLogger(console io.Writer, logPath string, quiet bool) (*Logger, error) {
	l := &Logger{console: console, quiet: quiet}
	if logPath != "" {
		f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err != nil {
			return nil, fmt.Errorf("cannot open log file: %w", err)
		}
		l.file = f
	}
	return l, nil
}

func (l *Logger) Close() {
	if l.file != nil {
		_ = l.file.Close()
	}
}

// Progress reports status; suppressed on the console by --quiet,
// always written to the log file.
func (l *Logger) Progress(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if !l.quiet {
		_, _ = fmt.Fprintln(l.console, msg)
	}
	l.toFile(msg)
}

// Error reports a failure; never suppressed on the console.
func (l *Logger) Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintln(l.console, msg)
	l.toFile("ERROR: " + msg)
}

func (l *Logger) toFile(msg string) {
	if l.file != nil {
		_, _ = fmt.Fprintf(l.file, "%s %s\n", time.Now().Format(time.RFC3339), msg)
	}
}
