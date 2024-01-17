package task

import (
	"context"

	"github.com/can1357/gosu/pkg/clog"
)

type workerBase struct {
	context.Context              // The root context.
	task            Task         // The associated task.
	options         Options      // The options.
	logger          *clog.Logger // The logger.
	whiteboard      Whiteboard   // The whiteboard.
}

func newWorkerBase(ctx context.Context, task Task, options Options) (m *workerBase) {
	m = &workerBase{
		task:    task,
		options: options,
	}
	m.logger = clog.FromContext(ctx)
	m.whiteboard = WhiteboardFromContext(ctx)
	label := task.Label()
	if label == m.logger.Namespace {
		label = ""
	}
	m.logger = m.logger.Fork(label)
	if label != "" {
		m.whiteboard = m.whiteboard.Fork(label)
		ctx = m.logger.WithContext(ctx)
	}
	m.Context = ctx
	return
}
func (work *workerBase) Task() Task {
	return work.task
}
func (work *workerBase) Options() Options {
	return work.options
}
func (work *workerBase) Logger() *clog.Logger {
	return work.logger
}
func (work *workerBase) Namespace() string {
	return work.logger.Namespace // TODO: FIX THIS
}
func (work *workerBase) Whiteboard() *Whiteboard {
	return &work.whiteboard
}
