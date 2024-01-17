package clog

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"time"
)

// Hooking system to inject listeners into all loggers.
type Hook interface {
	// Called when a logger is writing a line.
	// The line is already formatted.
	Write(ns string, line string, kind Stream)
}

var hooks sync.Map

func RegisterHook(hook Hook) {
	hooks.Store(hook, struct{}{})
}
func RemoveHook(hook Hook) {
	hooks.Delete(hook)
}

// Writer type to wrap an io.Writer with the processing.
type Writer struct {
	owner            *Logger
	mu               sync.Mutex
	buffer           strings.Builder
	underlyingStream io.Writer
	kind             Stream
}

func createWriter(owner *Logger, underlyingStream io.Writer, kind Stream) *Writer {
	return &Writer{
		owner:            owner,
		underlyingStream: underlyingStream,
		kind:             kind,
	}
}

func forEachLine(p []byte, out func(data []byte, ended bool)) (n int) {
	n = 0
	for len(p) > 0 {
		nl := bytes.IndexByte(p, '\n')
		if nl == -1 {
			out(p, false)
			break
		}
		nl++
		out(p[:nl], true)
		n += nl
		p = p[nl:]
	}
	return n
}

func (f *Writer) WritePrefix() {
	if f.owner.Timestamp != "" {
		f.buffer.WriteString(time.Now().Format(f.owner.Timestamp))
	}
	ns := f.owner.prefixComputed
	if ns != "" {
		f.buffer.WriteString(ns)
	}
}

func (f *Writer) Flush() (e error) {
	result := f.buffer.String()
	f.buffer.Reset()

	hooks.Range(func(key any, value any) bool {
		key.(Hook).Write(f.owner.Namespace, result, f.kind)
		return true
	})

	if f.underlyingStream != nil {
		_, e = f.underlyingStream.Write([]byte(result))
	}
	return
}

func (f *Writer) Write(input []byte) (n int, err error) {
	n = forEachLine(input, func(data []byte, isEnd bool) {
		f.mu.Lock()
		defer f.mu.Unlock()

		if f.buffer.Len() == 0 {
			f.WritePrefix()
		}
		f.buffer.Write(data)
		if isEnd {
			err = f.Flush()
		}
	})
	return
}
