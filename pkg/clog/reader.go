package clog

import (
	"io"
	"os"

	"github.com/icza/backscanner"
)

func tailFileN(file *os.File, numLines int) (out []string, err error) {
	// Seek to the end.
	fileSize, err := file.Seek(0, 2)
	if err != nil {
		return nil, err
	}
	backscanner := backscanner.New(file, int(fileSize))
	out = make([]string, numLines)
	at := numLines - 1
	for at >= 0 {
		line, _, err := backscanner.Line()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		out[at] = line
		at--
	}
	return out[at+1:], nil
}
func tailPathN(path string, numLines int) (out []string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return tailFileN(file, numLines)
}

func (logger *Logger) Tail(numLines int, stream Stream) (out []string, err error) {
	logger.mu.RLock()
	path := logger.Paths[stream]
	logger.mu.RUnlock()
	if path == "" {
		return nil, nil
	}
	return tailPathN(path, numLines)
}
