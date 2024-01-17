package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/can1357/gosu/pkg/automarshal"
	"github.com/can1357/gosu/pkg/client/view"
	"github.com/can1357/gosu/pkg/job"
	"github.com/can1357/gosu/pkg/session"
	"github.com/can1357/gosu/pkg/settings"
	"github.com/can1357/gosu/pkg/surpc"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/samber/lo"
)

var commands = map[string]func(body, flags string) error{}

func addCommand[T any](name any, fn func(body string, arg T) error) {
	fnw := func(body, c string) error {
		var arg T
		err := automarshal.NewArgReader(c).Unmarshal(&arg)
		if err != nil {
			return err
		}
		return fn(body, arg)
	}
	switch name := name.(type) {
	case string:
		commands[name] = fnw
	case []string:
		for _, n := range name {
			commands[n] = fnw
		}
	default:
		panic(fmt.Errorf("invalid command name: %v", name))
	}
}

var client *surpc.Client

func getClient() *surpc.Client {
	if client == nil {
		client = surpc.NewClient(settings.Rpc.Get().LocalAddress)
		if !session.Running() {
			self := lo.Must(os.Executable())
			daemon := exec.Command(self, "daemon")
			lo.Must0(daemon.Start())
			for i := 0; i < 50; i++ {
				x := 0
				if err := client.Call("daemon.Ping", &x, x); err == nil {
					break
				}
				time.Sleep(100 * time.Millisecond)
			}
			fmt.Printf("Started daemon %d\n", daemon.Process.Pid)
			daemon.Process.Release()
		}
	}
	return client
}
func Call(method string, reply any, arg any) error {
	return getClient().Call(method, reply, arg)
}

func Run() {
	display := func(jobs []session.RpcJobInfo, e error) {
		list := view.NewTasklist(func() ([]session.RpcJobInfo, error) { return jobs, e })
		fmt.Print(list.View())
	}

	addCommand([]string{"view", "v"}, func(p string, _ struct{}) error {
		list := view.NewTasklist(func() (info []session.RpcJobInfo, err error) {
			var res session.RpcSessionJobs
			err = Call("job.List", &res, p)
			if err == nil {
				info = res.Jobs
			}
			return
		})
		tea.NewProgram(list).Run()
		return nil
	})
	addCommand([]string{"list", "ls"}, func(p string, _ struct{}) error {
		var res session.RpcSessionJobs
		err := Call("job.List", &res, p)
		display(res.Jobs, err)
		return nil
	})
	addCommand("put", func(jobAndKey string, value any) error {
		var key session.RpcWhiteboardKey
		if before, after, found := strings.Cut(jobAndKey, ":"); found {
			key.Job = before
			key.Key = after
		} else {
			return fmt.Errorf("invalid key: '%s', expected job:key", jobAndKey)
		}

		var count int
		err := Call("whiteboard.Put", &count, key)
		if err != nil {
			display(nil, err)
		}
		fmt.Printf("Put %d values\n", count)
		return nil
	})
	addCommand("get", func(jobAndKey string, _ struct{}) error {
		var key session.RpcWhiteboardKey
		if before, after, found := strings.Cut(jobAndKey, ":"); found {
			key.Job = before
			key.Key = after
		} else {
			return fmt.Errorf("invalid key: '%s', expected job:key", jobAndKey)
		}
		var out []session.RpcWhiteboardKv
		err := Call("whiteboard.Get", &out, key)
		if err != nil {
			display(nil, err)
		} else {
			for _, v := range out {
				fmt.Printf("%s:%s: %s\n", v.Job, v.Key, string(v.Value))
			}
		}
		return nil
	})

	addCommand("signal", func(p string, _ struct{}) error {
		err := Call("event.Signal", nil, p)
		if err != nil {
			display(nil, err)
		}
		return nil
	})
	addCommand("shutdown", func(_ string, _ struct{}) error {
		var res any
		err := Call("daemon.Shutdown", &res, nil)
		if err != nil && !errors.Is(err, io.EOF) {
			display(nil, err)
		}
		return nil
	})

	addCreate := func(cmd string, name []string, human string) {
		addCommand(name, func(body string, mf job.Manifest) error {
			err := json.Unmarshal(lo.Must(json.Marshal(body)), &mf.Main)
			if err != nil {
				display(nil, err)
				return nil
			}
			js, _ := json.MarshalIndent(mf, "", "  ")
			fmt.Printf(human, string(js))
			var res session.RpcJobInfo
			err = Call(cmd, &res, mf)
			display([]session.RpcJobInfo{res}, err)
			return nil
		})
	}
	addCtl := func(cmd string, name []string, human string) {
		addCommand(name, func(p string, _ struct{}) error {
			var res []string
			err := Call(cmd, &res, p)
			if err != nil {
				display(nil, err)
			} else {
				fmt.Println(human, strings.Join(res, ","))
			}
			return nil
		})
	}
	addCreate("job.Update", []string{"update", "u"}, "Updating job %v\n")
	addCreate("job.Launch", []string{"launch", "a"}, "Starting job %v\n")
	addCtl("job.Start", []string{"start", "s"}, "Started job(s):")
	addCtl("job.Stop", []string{"stop", "x"}, "Stopped job(s):")
	addCtl("job.Restart", []string{"restart", "r"}, "Restarted job(s):")
	addCtl("job.Kill", []string{"kill", "k"}, "Killed job(s):")
	addCtl("job.Delete", []string{"delete", "d"}, "Deleted job(s):")

	cmd := ""
	args := ""
	body := ""
	if len(os.Args) > 1 {
		cmd = os.Args[1]
		cmd = strings.ToLower(cmd)
		argString := strings.Builder{}
		for _, arg := range os.Args[2:] {
			if !strings.HasPrefix(arg, "-") {
				if body == "" {
					body = arg
				}
				continue
			}
			if sp := strings.Index(arg, " "); sp >= 0 {
				if i := strings.IndexByte(arg, '='); i >= 0 && i < sp {
					argString.WriteString(arg[:i+1])
					argString.WriteString(`"`)
					argString.WriteString(strings.ReplaceAll(arg[i+1:], `"`, `\"`))
					argString.WriteString(`"`)
				} else {
					argString.WriteString(`"`)
					argString.WriteString(strings.ReplaceAll(arg, `"`, `\"`))
					argString.WriteString(`"`)
				}
			} else {
				argString.WriteString(arg)
			}
			argString.WriteString(" ")
		}
		args = argString.String()
	}

	if fn, ok := commands[cmd]; ok {
		err := fn(body, args)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Printf("Unknown command: %s\n", cmd)
	}
}
