package javascript

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

var UnmarshalWrapperESM = minify(`
	import * as M from 'file://@';
	let R = M['default'] ?? M;
	R = typeof R == 'function' ? R(JSON.parse('$')) : R;
	console.error('\x0d\x01\x02'+JSON.stringify(R)+'\x03\x03\x0a')
`)
var UnmarshalWrapperCJS = minify(`
	const M = require('@');
	let R = M['default'] ?? M;
	R = typeof R == 'function' ? R(JSON.parse('$')) : R;
	console.error('\x0d\x01\x02'+JSON.stringify(R)+'\x03\x03\x0a')
`)

func minify(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}

func escapeString(bytes []byte) string {
	out := ""
	for _, b := range bytes {
		out += fmt.Sprintf("\\x%02x", b)
	}
	return out
}

func UnmarshalDynamicWith(execute Executor, ctx context.Context, path string, code string, params any, out any) (err error) {
	paramsEncoded := []byte("null")
	if params != nil {
		paramsEncoded, err = json.Marshal(params)
		if err != nil {
			return
		}
	}

	code = strings.ReplaceAll(code, "@", strings.ReplaceAll(path, "\\", "\\\\"))
	code = strings.ReplaceAll(code, "$", escapeString(paramsEncoded))

	var ty string = "--experimental-default-type=module"
	if code == UnmarshalWrapperCJS {
		ty = "--experimental-default-type=commonjs"
	}

	var cmd *exec.Cmd
	cmd, err = execute(ctx, ty, "-e", code)
	if err != nil {
		return
	}

	buf := bytes.NewBuffer(nil)
	cmd.Stdout = buf
	cmd.Stderr = buf
	err = cmd.Run()
	if err != nil {
		if buf.Len() > 0 {
			return errors.New(buf.String())
		}
		return
	}

	errbytes := buf.Bytes()
	start := bytes.LastIndex(errbytes, []byte("\x0d\x01\x02"))
	if start == -1 {
		return errors.New("invalid output")
	}
	end := bytes.LastIndex(errbytes, []byte("\x03\x03\x0a"))
	if end == -1 {
		return errors.New("invalid output")
	}
	if out != nil {
		err = json.Unmarshal(errbytes[start+3:end], out)
	}
	return
}
