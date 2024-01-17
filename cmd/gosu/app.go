package main

import (
	"fmt"
	"os"

	"github.com/can1357/gosu/pkg/client"
	"github.com/can1357/gosu/pkg/clog"
	"github.com/can1357/gosu/pkg/session"
	"github.com/samber/lo"
)

type myhook struct{}

func (myhook) Write(ns string, line string, kind clog.Stream) {
	fmt.Print(line)
}

func server() {
	clog.RegisterHook(myhook{})
	lo.Must0(session.Open().Wait())
}

func main() {
	if len(os.Args) == 1 || os.Args[1] == "daemon" {
		if session.TryAcquire() {
			server()
			return
		}
	}
	client.Run()
}
