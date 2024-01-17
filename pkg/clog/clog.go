package clog

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

type loggerKey struct{}

type Options struct {
	Output   string `json:"stdout,omitempty"`    // The file to which to redirect stdout, "null" to discard.
	Error    string `json:"stderr,omitempty"`    // The file to which to redirect stderr, "null" to discard, "merge" to redirect to stdout.
	LogTime  string `json:"log_time,omitempty"`  // The format to use for timestamps.
	LogName  string `json:"log_name,omitempty"`  // The name to prepend to log lines.
	PfxWidth int    `json:"pfx_width,omitempty"` // The width of the prefix.
}

type Stream uint8

const (
	StreamStdout Stream = iota
	StreamStderr
	StreamMax
)

func (str Stream) String() string {
	switch str {
	case StreamStdout:
		return "stdout"
	case StreamStderr:
		return "stderr"
	default:
		return fmt.Sprintf("stream(%d)", str)
	}
}

type Logger struct {
	Handles        [StreamMax]*os.File
	Paths          [StreamMax]string
	Opened         Stream
	Timestamp      string
	Namespace      string
	PfxWidth       int
	prefixComputed string

	// Cached formatters.
	mu  sync.RWMutex
	fmt [StreamMax]*Writer
}

func (logger *Logger) Fork(ns string) (r *Logger) {
	r = &Logger{
		Handles:   logger.Handles,
		Paths:     logger.Paths,
		Timestamp: logger.Timestamp,
		Namespace: logger.Namespace,
		PfxWidth:  logger.PfxWidth,
	}

	if ns != "" {
		if r.Namespace != "" {
			r.Namespace += "/" + ns
		} else {
			r.Namespace = ns
		}
	} else {
		r.prefixComputed = logger.prefixComputed
	}

	if r.Namespace != "" {
		r.prefixComputed = r.Namespace
		if max := r.PfxWidth; max != 0 {
			limit := max + 3
			if len(r.prefixComputed) > limit {
				r.prefixComputed = "..." + r.prefixComputed[len(r.prefixComputed)-max:]
			}
			if len(r.prefixComputed) < limit {
				r.prefixComputed += strings.Repeat(" ", limit-len(r.prefixComputed))
			}
		}
		r.prefixComputed += " | "
	}
	return
}

func (logger *Logger) getFormatter(str Stream) io.Writer {
	if logger.fmt[str] != nil {
		return logger.fmt[str]
	}
	logger.mu.Lock()
	defer logger.mu.Unlock()
	if logger.fmt[str] == nil {
		logger.fmt[str] = createWriter(logger, logger.Handles[str], str)
	}
	return logger.fmt[str]
}

func (logger *Logger) Stdout() io.Writer { return logger.getFormatter(StreamStdout) }
func (logger *Logger) Stderr() io.Writer { return logger.getFormatter(StreamStderr) }

func (logger *Logger) Printf(format string, args ...any) (n int, err error) {
	return fmt.Fprintf(logger.getFormatter(StreamStdout), format+"\n", args...)
}

func (logger *Logger) Close() {
	logger.mu.Lock()
	defer logger.mu.Unlock()

	mask := logger.Opened
	for i, handle := range logger.Handles {
		if fmt := logger.fmt[i]; fmt != nil {
			fmt.Flush()
		}
		if handle != nil && (mask&1) != 0 {
			handle.Close()
			logger.Handles[i] = nil
		}
		mask >>= 1
	}
	logger.Opened = 0
}

func (logger *Logger) Flush() {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	for _, fmt := range logger.fmt {
		if fmt != nil {
			fmt.Flush()
		}
	}
}
func (logger *Logger) Sync() {
	logger.mu.Lock()
	defer logger.mu.Unlock()
	for _, handle := range logger.Handles {
		if handle != nil {
			handle.Sync()
		}
	}
}

var defaultLogger = Logger{
	Handles: [StreamMax]*os.File{
		StreamStdout: os.Stdout,
		StreamStderr: os.Stderr,
	},
}

func FromContext(ctx context.Context) *Logger {
	if logger, ok := ctx.Value(loggerKey{}).(*Logger); ok {
		return logger
	}
	return &defaultLogger
}
func New(prev *Logger, opts Options) (r *Logger, err error) {
	if prev == nil {
		prev = &defaultLogger
	}
	if opts.PfxWidth == 0 {
		opts.PfxWidth = prev.PfxWidth
		if opts.PfxWidth == 0 {
			opts.PfxWidth = max(10, len(prev.Namespace))
		}
	}
	r = prev.Fork(opts.LogName)

	// Apply options.
	if opts.Output != "" {
		if opts.Output == "null" {
			r.Handles[StreamStdout] = nil
		} else {
			r.Paths[StreamStdout] = opts.Output
			r.Handles[StreamStdout], err = os.OpenFile(opts.Output, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				r.Close()
				return
			}
			r.Opened |= 1 << StreamStdout
		}
	}
	if opts.Error != "" {
		if opts.Error == "null" {
			r.Handles[StreamStderr] = nil
		} else if opts.Error == "merge" {
			r.Handles[StreamStderr] = r.Handles[StreamStdout]
		} else {
			r.Paths[StreamStderr] = opts.Error
			r.Handles[StreamStderr], err = os.OpenFile(opts.Error, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				r.Close()
				return
			}
			r.Opened |= 1 << StreamStderr
		}
	}
	if opts.LogTime != "" {
		r.Timestamp = opts.LogTime
	}
	if opts.PfxWidth != 0 {
		r.PfxWidth = opts.PfxWidth
	}
	return
}
func (logger *Logger) WithContext(ctx context.Context) context.Context {
	if FromContext(ctx) == logger {
		return ctx
	}
	ctx = context.WithValue(ctx, loggerKey{}, logger)
	context.AfterFunc(ctx, logger.Close)
	return ctx
}
