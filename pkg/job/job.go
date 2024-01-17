package job

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/can1357/gosu/pkg/clog"
	"github.com/can1357/gosu/pkg/settings"
	"github.com/can1357/gosu/pkg/task"
	"github.com/can1357/gosu/pkg/util"
)

type LoggerOptions = clog.Options
type Manifest struct {
	ID            string    `json:"id,omitempty"` // The task's ID.
	Main          task.Task `json:"main"`         // The main task to run.
	Launch        *Trigger  `json:"launch,omitempty"`
	Drop          *Trigger  `json:"drop,omitempty"`
	task.Options            // The task options.
	LoggerOptions           // The log configuration.
}

type Job struct {
	context.Context
	Manifest   *Manifest
	ID         string
	Main       task.Task
	Cancel     context.CancelFunc
	Options    task.Options
	Logger     *clog.Logger
	Launch     *Trigger
	Drop       *Trigger
	mu         sync.Mutex
	worker     task.Worker
	Whiteboard atomic.Pointer[task.Whiteboard]
}

func Parse(data string) (j *Job, err error) {
	j = &Job{}
	err = json.Unmarshal([]byte(data), j)
	return
}

func (s *Job) Worker() task.Worker {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.worker
}

func (s *Job) startLocked() (w task.Worker, started bool) {
	if s.worker != nil {
		return s.worker, false
	}
	worker := task.NewWorker(s.Context, s.Main, s.Options)
	s.worker = worker
	s.Whiteboard.Store(worker.Whiteboard())
	go worker.Run()
	return worker, true
}
func (s *Job) stopLocked() {
	w := s.worker
	if w == nil {
		return
	}
	s.worker = nil
	w.Stop()
	<-w.Done()
}
func (s *Job) Start() (w task.Worker, started bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("Starting job %s\n", s.ID)
	return s.startLocked()
}
func (s *Job) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("Stopping job %s\n", s.ID)
	s.stopLocked()
}
func (s *Job) Restart() {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("Restarting job %s\n", s.ID)
	s.stopLocked()
	s.startLocked()
}
func (s *Job) Join(c context.Context) error {
	work := s.Worker()
	if work == nil {
		return task.Idle
	}
	select {
	case <-c.Done():
		return util.Cause(c)
	case <-s.worker.Done():
		return util.Cause(s.worker)
	}
}
func (s *Job) Kill() {
	s.mu.Lock()
	defer s.mu.Unlock()
	fmt.Printf("Killing job %s\n", s.ID)
	if s.worker != nil {
		s.worker.Kill()
		s.worker = nil
	}
}
func (s *Job) Status() task.StatusOrError {
	work := s.Worker()
	if work != nil {
		return work.Status()
	}
	return task.Idle
}
func (s *Job) Traverse(fn func(task.Worker) bool) {
	work := s.Worker()
	if work != nil {
		fn(work)
	}
}
func (j *Job) Ready(ctx context.Context) {
	ctx, j.Cancel = context.WithCancel(ctx)
	j.Context = j.Logger.WithContext(ctx)

	startOrJoin := func() {
		w, _ := j.Start()
		select {
		case <-w.Done():
		case <-j.Context.Done():
		}
	}
	if trigger := j.Launch; trigger != nil {
		cancel := trigger.Listen(startOrJoin)
		context.AfterFunc(j.Context, cancel)
	}
	if trigger := j.Drop; trigger != nil {
		cancel := trigger.Listen(j.Stop)
		context.AfterFunc(j.Context, cancel)
	}
}
func (recipe *Manifest) Spawn() (j *Job, err error) {
	j = &Job{}
	err = recipe.SpawnAt(j)
	return
}
func (recipe *Manifest) SpawnAt(j *Job) (err error) {
	j.Options = recipe.Options
	j.ID = recipe.ID
	logOpts := recipe.LoggerOptions
	if logOpts.LogName == "" {
		logOpts.LogName = j.ID
	}
	if logOpts.Output == "" {
		logOpts.Output = filepath.Join(settings.LogDir.Path(), fmt.Sprintf("%s.%s", j.ID, "log"))
	}
	if logOpts.Error == "" {
		logOpts.Error = filepath.Join(settings.LogDir.Path(), fmt.Sprintf("%s.%s", j.ID, "err"))
	}
	if j.Logger, err = clog.New(nil, logOpts); err != nil {
		return
	}
	j.Main = recipe.Main
	if j.Main.ID.ID == "" {
		j.Main.ID.ID = j.ID
	}
	j.Drop = recipe.Drop
	j.Launch = recipe.Launch
	j.Manifest = recipe
	return nil
}

func (j *Job) UnmarshalJSON(data []byte) error {
	var recipe Manifest
	if err := json.Unmarshal(data, &recipe); err != nil {
		return err
	}
	return recipe.SpawnAt(j)
}
