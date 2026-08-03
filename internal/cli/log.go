package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Logger writes progress to a console writer (usually stderr) and,
// optionally, to a timestamped log file.
type Logger struct {
	console io.Writer
	file    *os.File
	quiet   bool
	eta     bool
	etaLen  int
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
		if l.eta {
			_, _ = fmt.Fprintln(l.console)
			l.eta = false
			l.etaLen = 0
		}
		_, _ = fmt.Fprintln(l.console, msg)
	}
	l.toFile(msg)
}

func (l *Logger) ETA(msg string, tty bool) {
	if !l.quiet {
		if tty {
			padding := ""
			if len(msg) < l.etaLen {
				padding = strings.Repeat(" ", l.etaLen-len(msg))
			}
			_, _ = fmt.Fprintf(l.console, "\r%s%s", msg, padding)
			l.eta = true
			l.etaLen = len(msg)
		} else {
			_, _ = fmt.Fprintln(l.console, msg)
		}
	}
	l.toFile(msg)
}

// Warn reports a non-fatal issue; never suppressed on the console.
func (l *Logger) Warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.clearETA()
	_, _ = fmt.Fprintln(l.console, msg)
	l.toFile("WARN: " + msg)
}

// Error reports a failure; never suppressed on the console.
func (l *Logger) Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	l.clearETA()
	_, _ = fmt.Fprintln(l.console, msg)
	l.toFile("ERROR: " + msg)
}

func (l *Logger) clearETA() {
	if l.eta {
		_, _ = fmt.Fprintln(l.console)
		l.eta = false
		l.etaLen = 0
	}
}

func (l *Logger) toFile(msg string) {
	if l.file != nil {
		_, _ = fmt.Fprintf(l.file, "%s %s\n", time.Now().Format(time.RFC3339), msg)
	}
}
