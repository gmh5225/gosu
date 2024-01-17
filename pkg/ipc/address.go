package ipc

import (
	"encoding/base32"
	"runtime"
	"strconv"
	"strings"

	"github.com/can1357/gosu/pkg/util"
)

var prefix, suffix string

func init() {
	if runtime.GOOS == "windows" {
		prefix = `\\.\pipe\gosu-`
		suffix = ""
	} else {
		prefix = "/tmp/gosu-"
		suffix = ".sock"
	}
}

// Given an address, return the name of the pipe, or false if it is not a valid address.
func FromAddress(adr string) (name string, ok bool) {
	if !strings.HasPrefix(adr, prefix) || !strings.HasSuffix(adr, suffix) {
		return
	}
	return adr[len(prefix) : len(adr)-len(suffix)], true
}

// Given a name, return the address of the pipe, or generate a random name if none is provided.
func NewAddress(name string) string {
	if len(name) == 0 {
		bytes := util.RandomBytes(12)
		name = strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes))
	}
	return prefix + name + suffix
}

// Given a name, return the address of the pipe with the name escaped for use in a pipe path.
func NewAddressEscape(name string) string {
	builder := strings.Builder{}
	for _, c := range name {
		if !util.RunesAlphanum.TestRune(c) {
			builder.WriteString("xU" + strconv.FormatInt(int64(c), 16))
		} else {
			builder.WriteRune(c)
		}
	}
	return NewAddress(builder.String())
}
