package foreign

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/can1357/gosu/pkg/clog"
)

type Bridge interface {
	// Run the script at the given path with the given arguments.
	Run(ctx context.Context, script string, args ...string) (cmd *exec.Cmd, err error)

	// Run the script at the given path with the given parameters, unmarshal the result into out.
	// Can also be a data format, such as JSON where the parameters are ignored.
	Unmarshal(ctx context.Context, path string, params any, out any) (err error)
}

var Languages = map[string]Bridge{}
var Extensions = map[string][]Bridge{}

// Utility functions.
func RunContextWith(ctx context.Context, bridge Bridge, path string, args ...string) (cmd *exec.Cmd, err error) {
	logger := clog.FromContext(ctx)
	cmd, err = bridge.Run(ctx, path, args...)
	if err == nil {
		cmd.Stdout = logger.Stdout()
		cmd.Stderr = logger.Stderr()
	}
	return
}

// External facing convenience functions.
func UnmarshalContext(ctx context.Context, path string, data any, out any) (err error) {
	if fi, err := os.Stat(path); err != nil {
		return err
	} else if fi.IsDir() {
		return errors.New("cannot unmarshal a directory")
	}

	ext := strings.ToLower(filepath.Ext(path))
	for _, rt := range Extensions[ext] {
		err = rt.Unmarshal(ctx, path, data, out)
		if err == nil {
			return
		}
	}
	if err == nil {
		err = errors.New("no unmarshaler found for extension: " + ext)
	}
	return
}
func RunContext(ctx context.Context, path string, args ...string) (cmd *exec.Cmd, err error) {
	ext := strings.ToLower(filepath.Ext(path))
	for _, rt := range Extensions[ext] {
		cmd, err = RunContextWith(ctx, rt, path, args...)
		if err == nil {
			return
		}
	}
	if err == nil {
		err = errors.New("no runtime found for extension: " + ext)
	}
	return
}
func Unmarshal(path string, data any, out any) (err error) {
	return UnmarshalContext(context.Background(), path, data, out)
}
func Run(path string, args ...string) (cmd *exec.Cmd, err error) {
	return RunContext(context.Background(), path, args...)
}

// Registrar.
func Register[T Bridge](i T, lang []string, ext []string) {
	for _, lang := range lang {
		Languages[lang] = i
	}
	for _, ext := range ext {
		Extensions[ext] = append(Extensions[ext], i)
	}
}
