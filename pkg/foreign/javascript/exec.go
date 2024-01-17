package javascript

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"

	"github.com/can1357/gosu/pkg/settings"
)

type Executor = func(ctx context.Context, script string, args ...string) (*exec.Cmd, error)

func errorExecutor(e error) Executor {
	return func(ctx context.Context, script string, args ...string) (*exec.Cmd, error) {
		return nil, e
	}
}
func binaryExecutor(executable string) Executor {
	return func(ctx context.Context, script string, args ...string) (*exec.Cmd, error) {
		if script != "" {
			args = append([]string{script}, args...)
		}
		return exec.CommandContext(ctx, executable, args...), nil
	}
}

var ExecJS = sync.OnceValue(func() Executor {
	jsSettings := settings.Javascript.Get()
	pathToEngine, err := exec.LookPath(jsSettings.Engine)
	if err != nil {
		return errorExecutor(err)
	}
	return binaryExecutor(pathToEngine)
})
var ExecTS = sync.OnceValue(func() Executor {
	jsSettings := settings.Javascript.Get()
	if jsSettings.Engine == "node" {
		loaderModule, err := ResolveGlobalImport(jsSettings.Transpiler, true)
		if err != nil {
			// Try to use the binary-version.
			binaryName := strings.ReplaceAll(jsSettings.Transpiler, "/", "-")
			fullPath, err := exec.LookPath(binaryName)
			if err != nil {
				return errorExecutor(fmt.Errorf("transpiler %s not found: %w", jsSettings.Transpiler, err))
			}
			if binaryName == "ts-node" || binaryName == "ts-node-esm" || binaryName == "tsx" {
				log.Printf("[WARN] Failed to locate loader script, may cause sub-optimal process configuration: %v", err)
			}
			return binaryExecutor(fullPath)
		}

		return func(ctx context.Context, script string, args ...string) (*exec.Cmd, error) {
			args = append([]string{
				"--experimental-specifier-resolution=node",
				"--import",
				"file://" + loaderModule,
				script,
			}, args...)
			return ExecJS()(ctx, "", args...)
		}
	} else {
		return ExecJS()
	}
})
