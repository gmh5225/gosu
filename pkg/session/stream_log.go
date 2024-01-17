package session

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/can1357/gosu/pkg/clog"
	"github.com/can1357/gosu/pkg/job"
	"github.com/can1357/gosu/pkg/surpc"
)

type RpcLogMessage struct {
	Kind string `json:"kind"`
	Line string `json:"line"`
}
type logStreamHook struct {
	pattern *regexp.Regexp
	channel chan RpcLogMessage
}

func (hk *logStreamHook) Write(ns string, line string, kind clog.Stream) {
	if !hk.pattern.MatchString(ns) {
		return
	}
	select {
	case hk.channel <- RpcLogMessage{Line: line, Kind: kind.String()}:
	default:
	}
}

func (s *Session) LogsHandler(w http.ResponseWriter, r *http.Request) {
	secure, ok := s.RpcServer.Authorize(w, r)
	if !ok {
		return
	}

	if r.Method != "GET" {
		http.Error(w, "", http.StatusMethodNotAllowed)
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.NotFound(w, r)
		return
	}

	// Get the pattern.
	pattern := r.URL.Query().Get("q")
	if pattern == "" {
		pattern = ".*"
	}
	pattern = "(?i)" + pattern
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid pattern: %s", err), 400)
		return
	}

	// Get the tail count
	tail := 50
	if t := r.URL.Query().Get("t"); t != "" {
		n, e := strconv.Atoi(t)
		if e == nil {
			tail = n
		}
	}

	// Send the headers
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Create the encoder.
	var writer io.Writer = w
	if secure != nil {
		writer = surpc.NewCipherWriter(writer, secure)
	}
	enc := json.NewEncoder(writer)
	write := func(line RpcLogMessage) { enc.Encode(line); flusher.Flush() }

	// Find all jobs that match the pattern.
	s.ForEachJobRgx(compiled, func(j *job.Job) error {
		if logger := j.Logger; logger != nil {
			for i := clog.StreamStdout; i <= clog.StreamStderr; i++ {
				lines, err := logger.Tail(tail, i)
				if err == nil {
					for _, line := range lines {
						write(RpcLogMessage{Kind: i.String(), Line: line})
					}
				}
			}
		}
		return nil
	})

	// Insert the hook.
	ch := make(chan RpcLogMessage, 128)
	hk := &logStreamHook{
		pattern: compiled,
		channel: ch,
	}
	clog.RegisterHook(hk)
	defer clog.RemoveHook(hk)
	for {
		select {
		case line := <-ch:
			write(line)
		case <-r.Context().Done():
			return
		}
	}
}
