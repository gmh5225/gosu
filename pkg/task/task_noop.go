package task

import (
	"time"

	"github.com/can1357/gosu/pkg/util"
	"github.com/samber/lo"
)

type TaskWait struct {
	Duration util.ParsableDuration `json:"duration"`
}
type TaskNoop struct{}

func (t TaskNoop) Launch(ctx Controller) <-chan error {
	ch := make(chan error, 1)
	ch <- nil
	return ch
}

func (h *TaskWait) UnmarshalInline(text string) (err error) {
	return h.Duration.UnmarshalText([]byte(text))
}

func (h *TaskWait) Launch(ctx Controller) <-chan error {
	return lo.Async(func() error {
		select {
		case <-ctx.Done():
			return util.Cause(ctx)
		case <-time.After(h.Duration.Duration):
			return nil
		}
	})
}

func init() {
	Registry.Define("wait", TaskWait{})
	Registry.Define("noop", TaskNoop{})
}
