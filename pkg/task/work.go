package task

import (
	"context"

	"github.com/can1357/gosu/pkg/clog"
)

type StatusOrError = error
type Worker interface {
	Task() Task
	Options() Options
	Namespace() string
	Logger() *clog.Logger
	Status() StatusOrError
	Whiteboard() *Whiteboard

	context.Context
	Inspect() Report
	Run() error
	Stop()
	Kill()
	Traverse(func(Worker) bool)
}
type Controller interface {
	Worker
	Report(Report)
	Stopping() <-chan struct{}
	Launch(ctx context.Context, subtask Task, modifiers ...func(*Options)) <-chan error
}

func NewWorker(ctx context.Context, task Task, options Options) (w Worker) {
	options.WithDefaults()
	if tcfg, ok := task.ITask.(TaskWithOpts); ok {
		tcfg.Configure(&options)
	}
	if tex, ok := task.ITask.(TaskEx); ok {
		return tex.LaunchEx(ctx, options)
	}
	base := newWorkerBase(ctx, task, options)
	if options.RetryDisabled {
		w = newMustWorker(base)
	} else {
		w = newRetryWorker(base)
	}
	return
}
