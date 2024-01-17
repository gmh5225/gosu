package task

import (
	"context"

	"github.com/can1357/gosu/pkg/automarshal"
)

type ITask interface {
	Launch(Controller) (exitReason <-chan error)
}
type TaskWithOpts interface {
	Configure(*Options)
}
type TaskEx interface {
	LaunchEx(ctx context.Context, options Options) Worker
}

type Task struct {
	ITask
	automarshal.ID
}

var Registry = automarshal.NewRegistry[Task, ITask]()

func (t *Task) UnmarshalJSON(data []byte) error { return Registry.Unmarshal(t, data) }
func (t Task) MarshalJSON() ([]byte, error)     { return Registry.Marshal(t) }
