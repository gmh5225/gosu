package task

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/can1357/gosu/pkg/automarshal"
	"github.com/can1357/gosu/pkg/foreign"
	"github.com/can1357/gosu/pkg/ipc"
	"github.com/can1357/gosu/pkg/revproxy"
	"github.com/can1357/gosu/pkg/settings"
	"github.com/can1357/gosu/pkg/util"
	"github.com/samber/lo"
	"github.com/shirou/gopsutil/v3/process"
)

const inspectRate = 1 * time.Second

type TaskRun struct {
	Foreign string            `json:"-"`               // The foreign language to run.
	Exec    string            `json:"exec,omitempty"`  // The executable used to run the script.
	Args    []string          `json:"args,omitempty"`  // Arguments passed.
	Cwd     string            `json:"cwd"`             // Working directory.
	Env     map[string]string `json:"env,omitempty"`   // The environment variables to set.
	N       int               `json:"n,omitempty"`     // The number of instances to launch, >1 will run as cluster with special env.
	Proxy   *revproxy.Options `json:"proxy,omitempty"` // The proxy options.

}

type processRunner struct {
	*TaskRun
	lb  *revproxy.LoadBalancer
	n   int
	ipc string
}

func lifecheck(adr string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	con, err := ipc.DialContext(ctx, adr)
	if err != nil {
		return false
	}
	defer con.Close()
	con.Write([]byte("GET / HTTP/1.1\r\n\r\n"))
	buf := make([]byte, 1024)
	n, _ := con.Read(buf)
	return strings.Contains(string(buf[:n]), "HTTP/1.1")
}

func (h *processRunner) Launch(ctx Controller) <-chan error {
	var err error
	var cmd *exec.Cmd
	if flavor := h.Foreign; flavor == "" || flavor == "run" {
		cmd = exec.CommandContext(ctx, h.Exec, h.Args...)
	} else {
		cmd, err = foreign.Languages[flavor].Run(ctx, h.Exec, h.Args...)
		if err != nil {
			return lo.Async(func() error { return err })
		}
	}
	cmd.Dir = h.Cwd
	cmd.Env = os.Environ()
	for k, v := range h.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = append(cmd.Env, "GOSU_NS="+ctx.Namespace())
	cmd.Env = append(cmd.Env, "GOSU_CID="+fmt.Sprintf("%d", h.n))
	cmd.Env = append(cmd.Env, "GOSU_LOCAL="+settings.Rpc.Get().LocalAddress)
	if h.lb != nil {
		h.ipc = ipc.NewAddress("")
		cmd.Env = append(cmd.Env, "GOSU_SERVE="+h.ipc)
	}

	cmd.Stdout = ctx.Logger().Stdout()
	cmd.Stderr = ctx.Logger().Stderr()

	err = cmd.Start()
	if err != nil {
		return lo.Async(func() error { return err })
	}

	// Start the inspector.
	//
	done := false
	proc, _ := process.NewProcess(int32(cmd.Process.Pid))
	go func() {
		for !done {
			ctx.Report(InspectProcess(proc))
			time.Sleep(inspectRate)
		}
		ctx.Report(Report{})
	}()
	go func() {
		<-ctx.Stopping()
		if runtime.GOOS == "windows" {
			cmd.Process.Signal(os.Kill)
		} else {
			cmd.Process.Signal(os.Interrupt)
		}
	}()
	resultChanel := lo.Async(func() error {
		err := cmd.Wait()
		done = true
		return err
	})

	// Wait for the server to start.
	//
	if h.lb != nil {
		ctx.Logger().Printf("Waiting for server to start...")
		var upstream *revproxy.Upstream
		if true {
		loop:
			for !done {
				select {
				case <-ctx.Done():
					return lo.Async(func() error { return ctx.Err() })
				case ok := <-lo.Async(func() bool { return lifecheck(h.ipc) }):
					if ok {
						ctx.Logger().Printf("Server started.")
						upstream = revproxy.NewIpcUpstream(ctx.Namespace(), h.ipc)
						break loop
					}
				}
				select {
				case <-ctx.Done():
					return lo.Async(func() error { return ctx.Err() })
				case <-time.After(100 * time.Millisecond):
				}
			}
		} else {
			ctx.Logger().Printf("Server started.")
			upstream = revproxy.NewIpcUpstream(ctx.Namespace(), h.ipc)
		}
		if upstream != nil {
			ctx.Logger().Printf("Adding upstream %v", upstream)
			h.lb.AddUpstream(upstream)
			go func() {
				<-ctx.Stopping()
				ctx.Logger().Printf("Removing upstream %v", upstream)
				h.lb.RemoveUpstream(upstream)
			}()
		}
	}
	return resultChanel
}

func (h *TaskRun) Launch(ctx Controller) <-chan error {
	var lb *revproxy.LoadBalancer
	if h.Proxy != nil {
		ctx.Logger().Printf("Starting proxy.")
		lb = revproxy.NewLoadBalancer(*h.Proxy)
		go func() {
			err := lb.Listen()
			if err != nil && ctx.Err() == nil {
				ctx.Logger().Printf("Proxy error: %v", err)
				ctx.Kill()
			}
		}()
	}

	newRunner := func(n int) *processRunner {
		r := &processRunner{TaskRun: h, lb: lb, n: n}
		return r
	}

	if h.N <= 1 {
		pr := newRunner(0)
		return lo.Async(func() error {
			if lb != nil {
				defer func() {
					ctx.Logger().Printf("Stopping proxy.")
					lb.Close()
				}()
			}
			return <-pr.Launch(ctx)
		})
	} else {
		pipe := newPipeController(ctx)
		return lo.Async(func() error {
			if lb != nil {
				defer func() {
					ctx.Logger().Printf("Stopping proxy.")
					lb.Close()
				}()
			}
			pipe.left.Store(int32(h.N))
			for i := 0; i < h.N && pipe.Err() == nil; i++ {
				i := i
				go func() {
					task := Task{
						ID: automarshal.ID{
							Kind: "",
							ID:   fmt.Sprintf("%d", i),
						},
						ITask: newRunner(i),
					}
					select {
					case <-pipe.Done():
						return
					case err := <-pipe.launch(task):
						if err != nil || pipe.left.Add(-1) == 0 {
							pipe.cancel(err)
						}
					}
				}()
			}
			<-pipe.Done()
			if e := util.Cause(pipe); e != nil {
				return NonRetriable(e)
			}
			return nil
		})
	}
}

func (t *TaskRun) WithDefaults() {
	if t.Cwd == "" {
		t.Cwd, _ = os.Getwd()
	}
}

func (h *TaskRun) UnmarshalInline(text string) (err error) {
	before, after, _ := strings.Cut(text, " ")
	h.Exec = before
	h.Args = strings.Fields(after)
	return
}

func init() {
	Registry.Define("run", TaskRun{})
	for lang := range foreign.Languages {
		Registry.Define(lang, TaskRun{Foreign: lang})
	}
}
