package task

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"

	"github.com/can1357/gosu/pkg/automarshal"
	"github.com/can1357/gosu/pkg/util"
	"github.com/samber/lo"
)

// Pipeline controller.
type pipeController struct {
	context.Context
	cancel context.CancelCauseFunc
	ctrl   Controller
	wg     sync.WaitGroup
	left   atomic.Int32
}

func newPipeController(ctrl Controller) (s *pipeController) {
	s = &pipeController{ctrl: ctrl}
	s.Context, s.cancel = util.WithCancelOrOk(ctrl)
	return
}
func (p *pipeController) launch(task Task) <-chan error {
	return p.ctrl.Launch(p.Context, task)
}

type Pipe struct {
	Mode     PipeMode `json:"-"`
	Subtasks []Task   `json:"sub"`
}

func (p *Pipe) Launch(controller Controller) (exitReason <-chan error) {
	n := len(p.Subtasks)
	if n == 0 {
		return nil
	}
	return lo.Async(func() error {
		pipe := newPipeController(controller)
		pipe.left.Store(int32(n))
		for i, task := range p.Subtasks {
			if pipe.Err() != nil {
				break
			}
			if p.Mode.serialize(i, n) {
				pipe.wg.Wait()
			}
			if pipe.Err() == nil {
				pipe.wg.Add(1)
				go func(i int, task Task) {
					defer pipe.wg.Done()
					select {
					case <-pipe.Done():
						return
					case err := <-pipe.launch(task):
						p.Mode.end(err, pipe.cancel)
						if pipe.left.Add(-1) == 0 {
							pipe.cancel(nil)
						}
					}
				}(i, task)
			}
		}
		<-pipe.Done()
		e := util.Cause(pipe)
		if e != nil {
			if _, ok := p.Mode.(PipeParallel); ok {
				return NonRetriable(e)
			}
		}
		return e
	})
}

func init() {
	Registry.Define("pipe", Pipe{})

	for _, mode := range PipeModes {
		Registry.Define(mode.String(), Pipe{Mode: mode})
	}
	Registry.RegisterNonObject('[', func(t *Task, data []byte) error {
		var tmp []Task
		if err := json.Unmarshal(data, &tmp); err != nil {
			return err
		}
		if len(tmp) == 0 {
			t.ID = automarshal.ID{Kind: "noop"}
			t.ITask = TaskNoop{}
			return nil
		}
		var mode PipeMode = PipeParallel{}
		if pipe, ok := tmp[0].ITask.(*Pipe); ok && len(pipe.Subtasks) == 0 {
			mode = pipe.Mode
			tmp = tmp[1:]
		}
		t.ID = automarshal.ID{Kind: mode.String()}
		t.ITask = &Pipe{Mode: mode, Subtasks: tmp}
		return nil
	})
}
