package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/can1357/gosu/pkg/util"
	"github.com/samber/lo"
)

// Defines a non-retriable work.
type mustWorker struct {
	*workerBase
	context.Context                         // The context of the runner.
	cancel          context.CancelCauseFunc //
	stopChannel     chan struct{}           // The channel to stop the runner.
	stopChannelUsed atomic.Bool             // Whether the stop channel has been used.
	report          atomic.Value            // The stats to be reported by the runner.
	status          Status                  // The current status (if alive).
	children        sync.Map                // The children workers.
}

func newMustWorker(m *workerBase) *mustWorker {
	w := &mustWorker{workerBase: m, stopChannel: make(chan struct{})}
	w.Context, w.cancel = util.WithCancelOrOk(m)
	context.AfterFunc(w.Context, func() { w.signalStop(false) })
	return w
}
func (work *mustWorker) Kill() {
	work.cancel(Cancelled)
}
func (work *mustWorker) Stop() {
	if work.Err() != nil {
		return
	}
	work.signalStop(true)
}
func (work *mustWorker) Inspect() Report {
	report, _ := work.report.Load().(Report)
	return report
}
func (work *mustWorker) Report(r Report) {
	work.report.Store(r)
}
func (work *mustWorker) Stopping() <-chan struct{} {
	return work.stopChannel
}
func (work *mustWorker) Launch(ctx context.Context, subtask Task, modifiers ...func(*Options)) <-chan error {
	opt := work.options
	for _, mod := range modifiers {
		mod(&opt)
	}
	w := NewWorker(ctx, subtask, opt)
	go func() {
		<-work.Stopping()
		w.Stop()
	}()
	return lo.Async(func() error {
		work.children.Store(w, struct{}{})
		defer work.children.Delete(w)
		return w.Run()
	})
}
func (work *mustWorker) Traverse(fn func(Worker) bool) {
	work.children.Range(func(key, value interface{}) bool {
		w := key.(Worker)
		return fn(w)
	})
}

func (work *mustWorker) signalStop(wait bool) {
	work.status = Stopping
	enforceOrCancel := func() {
		if work.options.StopTimeout.IsPositive() {
			select {
			case <-work.options.StopTimeout.After():
				work.cancel(TimeoutStop)
			case <-work.Done():
				break
			}
		} else {
			work.cancel(Cancelled)
		}
	}

	if !work.stopChannelUsed.Swap(true) {
		close(work.stopChannel)
		if !work.options.StopTimeout.IsPositive() {
			work.cancel(Cancelled)
		} else if wait {
			enforceOrCancel()
		} else {
			go enforceOrCancel()
		}
	}
}
func (work *mustWorker) Status() error {
	if work.Err() != nil {
		c := util.Cause(work)
		switch c {
		case nil:
			return Complete
		default:
			return c
		}
	} else if work.status.IsAlive() {
		return work.status
	} else {
		return Idle
	}
}
func (work *mustWorker) Run() error {
	// If start timeout is set, start a timer.
	var launchTimeout chan struct{}
	if work.options.StartTimeout.IsPositive() {
		launchTimeout = make(chan struct{})
		go func() {
			select {
			case <-work.options.StartTimeout.AfterIf():
				work.cancel(TimeoutStart)
			case <-work.Done():
				return
			case <-launchTimeout:
				return
			}
		}()
	}

	// Enforce max memory.
	if work.options.MaxMemory.IsPositive() {
		go func() {
			for {
				select {
				case <-work.Done():
					return
				case <-time.After(time.Second * 30):
					report := work.Inspect()
					if report.Mem > float64(work.options.MaxMemory.Value) {
						work.cancel(errors.New("memory limit exceeded"))
						return
					}
				}
			}
		}()
	}

	// Launch the task.
	work.status = Starting
	exitReason := work.task.Launch(work)
	if launchTimeout != nil {
		close(launchTimeout)
	}
	select {
	case <-work.Done():
		return util.Cause(work)

	case <-work.options.MinUptime.After():
		work.status = Running
		select {
		case <-work.Done():
			return util.Cause(work)
		case <-work.options.ExecTimeout.AfterIf():
			work.cancel(TimeoutExec)
			return TimeoutExec
		case err := <-exitReason:
			if err != nil && StatusFromErr(err) != Errored {
				err = fmt.Errorf("%w: %s", Errored, err.Error())
			}
			work.cancel(err)
			return err
		}

	case err := <-exitReason:
		if err == nil {
			err = fmt.Errorf("%w: quit too early", Errored)
		} else if StatusFromErr(err) != Errored {
			err = fmt.Errorf("%w: %s", Errored, err.Error())
		}
		work.cancel(err)
		return err
	}
}
