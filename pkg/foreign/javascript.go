package foreign

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/can1357/gosu/pkg/foreign/javascript"
)

type lazyExecutor func() javascript.Executor

func (l lazyExecutor) Run(ctx context.Context, script string, args ...string) (cmd *exec.Cmd, err error) {
	return l()(ctx, script, args...)
}
func (l lazyExecutor) Unmarshal(ctx context.Context, path string, params any, out any) (err error) {
	path, err = filepath.Abs(path)
	if err != nil {
		return
	}
	if buf, err := os.ReadFile(path); err == nil {
		if strings.Contains(string(buf), "module.exports =") {
			return javascript.UnmarshalDynamicWith(l(), ctx, path, javascript.UnmarshalWrapperCJS, params, out)
		} else if strings.Contains(string(buf), "export default ") {
			return javascript.UnmarshalDynamicWith(l(), ctx, path, javascript.UnmarshalWrapperESM, params, out)
		}
	}

	err = javascript.UnmarshalDynamicWith(l(), ctx, path, javascript.UnmarshalWrapperESM, params, out)
	if err != nil {
		err = javascript.UnmarshalDynamicWith(l(), ctx, path, javascript.UnmarshalWrapperCJS, params, out)
	}
	return
}

func init() {
	Register(lazyExecutor(javascript.ExecJS), []string{"js", "javascript"}, []string{".js", ".cjs", ".mjs"})
	Register(lazyExecutor(javascript.ExecTS), []string{"ts", "typescript"}, []string{".ts", ".cts", ".mts"})
}
